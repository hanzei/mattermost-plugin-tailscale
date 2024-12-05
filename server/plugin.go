package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	"tailscale.com/client/tailscale"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/mattermost/mattermost/server/public/pluginapi"
	"github.com/mattermost/mattermost/server/public/pluginapi/experimental/command"
)

// Plugin implements the interface expected by the Mattermost server to communicate between the server and plugin processes.
type Plugin struct {
	plugin.MattermostPlugin

	// configurationLock synchronizes access to the configuration.
	configurationLock sync.RWMutex

	// configuration is the active plugin configuration. Consult getConfiguration and
	// setConfiguration for usage.
	configuration *configuration

	// botID is the ID of the bot user created by the plugin
	botID string

	// client is the pluginapi client
	client *pluginapi.Client
}

func (p *Plugin) OnActivate() error {
	p.client = pluginapi.NewClient(p.API, p.Driver)
	bot := &model.Bot{
		Username:    "tailscale",
		DisplayName: "Tailscale",
		Description: "A bot account for the Tailscale plugin",
	}

	// Use pluginapi to create/ensure bot
	botID, err := p.client.Bot.EnsureBot(bot, pluginapi.ProfileImagePath("assets/tailscale-icon.png"))
	if err != nil {
		return errors.Wrap(err, "failed to ensure bot account")
	}
	p.botID = botID

	iconData, err := command.GetIconData(&p.client.System, "assets/tailscale-icon.svg")
	if err != nil {
		return errors.Wrap(err, "failed to get icon data")
	}

	if err := p.API.RegisterCommand(&model.Command{
		Trigger:              "tailscale",
		AutoComplete:         true,
		AutoCompleteDesc:     "Manage your Tailscale network",
		AutoCompleteHint:     "[command]",
		AutocompleteData:     getAutocompleteData(),
		AutocompleteIconData: iconData,
	}); err != nil {
		return errors.Wrap(err, "failed to register /tailscale command")
	}

	tailscale.I_Acknowledge_This_API_Is_Unstable = true

	return nil
}

func getAutocompleteData() *model.AutocompleteData {
	tailscale := model.NewAutocompleteData("tailscale", "[command]", "Available commands: connect, disconnect, list, acl, tauilnet, about")

	connect := model.NewAutocompleteData("connect", "<tailnet> <api-key>", "Connect to your Tailscale network")
	tailscale.AddCommand(connect)

	disconnect := model.NewAutocompleteData("disconnect", "", "Disconnect from your Tailscale network")
	tailscale.AddCommand(disconnect)

	list := model.NewAutocompleteData("list", "", "List all devices in your Tailnet")
	tailscale.AddCommand(list)

	acl := model.NewAutocompleteData("acl", "", "Show the ACL configuration for your Tailnet")
	tailscale.AddCommand(acl)

	tailnet := model.NewAutocompleteData("tailnet", "", "Show your current Tailnet name")
	tailscale.AddCommand(tailnet)

	about := command.BuildInfoAutocomplete("about")
	tailscale.AddCommand(about)

	return tailscale
}

func (p *Plugin) ExecuteCommand(c *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	p.executeCommand(c, args)
	return &model.CommandResponse{}, nil
}

func (p *Plugin) executeCommand(_ *plugin.Context, args *model.CommandArgs) {
	split := strings.Fields(args.Command)
	if len(split) < 2 {
		p.postEphemeral(args.UserId, args.ChannelId, "Usage: /tailscale connect <tailnet> <api-key>")
		return
	}
	cmd := split[1]

	var err error

	switch cmd {
	case "connect":
		if len(split) != 4 {
			p.postEphemeral(args.UserId, args.ChannelId, "Usage: /tailscale connect <tailnet> <api-key>")
			return
		}
		err = p.handleConnect(args, split[2], split[3])
	case "list":
		err = p.handleList(args)
	case "acl":
		err = p.handleACL(args)
	case "tailnet":
		err = p.handleTailnet(args)
	case "disconnect":
		err = p.handleDisconnect(args)
	case "about":
		err = p.handleAbout(args)
	default:
		p.postEphemeral(args.UserId, args.ChannelId, "Available commands: connect <tailnet> <api-key>, disconnect, list, acl, tailnet")
		return
	}

	if err != nil {
		p.postEphemeral(args.UserId, args.ChannelId, fmt.Sprintf("An error occurred: %s", err.Error()))
	}
}

func (p *Plugin) handleConnect(args *model.CommandArgs, tailnet, apiKey string) error {
	// Validate credentials by creating a client and making a test API call
	client := tailscale.NewClient(tailnet, tailscale.APIKey(apiKey))

	// Try to list devices as a basic API test
	_, err := client.Devices(context.Background(), nil)
	if err != nil {
		return fmt.Errorf("failed to authenticate with Tailscale: %w", err)
	}

	// Credentials are valid, store them in KV store
	config := UserTailscaleConfig{
		APIKey:  apiKey,
		Tailnet: tailnet,
	}

	data, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal configuration: %w", err)
	}

	if err := p.API.KVSet("tailscale_"+args.UserId, data); err != nil {
		return fmt.Errorf("failed to store configuration: %w", err)
	}

	p.postEphemeral(args.UserId, args.ChannelId, "Successfully authenticated with Tailscale for tailnet: "+tailnet)
	return nil
}

func (p *Plugin) handleList(args *model.CommandArgs) error {
	config, err := p.getUserTailscaleConfig(args.UserId)
	if err != nil {
		return fmt.Errorf("failed to retrieve Tailscale configuration: %w", err)
	}

	if config == nil {
		p.postEphemeral(args.UserId, args.ChannelId, "Please authenticate first using: `/tailscale connect <tailnet> <api-key>`")
		return nil
	}

	client := tailscale.NewClient(config.Tailnet, tailscale.APIKey(config.APIKey))
	devices, err := client.Devices(context.Background(), nil)
	if err != nil {
		return fmt.Errorf("failed to retrieve devices from Tailscale API: %w", err)
	}

	var taggedDevices, untaggedDevices []string

	for _, device := range devices {
		var deviceInfo strings.Builder
		deviceInfo.WriteString(fmt.Sprintf("- %s", device.Hostname))

		// Add tags if present
		if len(device.Tags) > 0 {
			deviceInfo.WriteString(fmt.Sprintf(" [%s]", strings.Join(device.Tags, ", ")))
		} else {
			// Add owner for untagged devices
			deviceInfo.WriteString(fmt.Sprintf(" (Owner: %s)", device.User))
		}

		// Add online/offline status
		if device.LastSeen != "" {
			lastSeen, err := time.Parse(time.RFC3339, device.LastSeen)
			if err == nil {
				if time.Since(lastSeen) < time.Minute {
					deviceInfo.WriteString(" (Online)")
				} else {
					deviceInfo.WriteString(" (**Offline**)")
				}
			}
		}
		deviceInfo.WriteString("\n")

		if len(device.Tags) > 0 {
			taggedDevices = append(taggedDevices, deviceInfo.String())
		} else {
			untaggedDevices = append(untaggedDevices, deviceInfo.String())
		}
	}

	var deviceList strings.Builder
	deviceList.WriteString("#### Devices in your Tailnet\n")

	// List tagged devices first
	if len(taggedDevices) > 0 {
		deviceList.WriteString("\n**Tagged Devices:**\n")
		for _, device := range taggedDevices {
			deviceList.WriteString(device)
		}
	}

	// Then list untagged devices
	if len(untaggedDevices) > 0 {
		deviceList.WriteString("\n**Untagged Devices:**\n")
		for _, device := range untaggedDevices {
			deviceList.WriteString(device)
		}
	}

	p.postEphemeral(args.UserId, args.ChannelId, deviceList.String())
	return nil
}

// getUserTailscaleConfig gets a user's Tailscale configuration from the KV store
func (p *Plugin) getUserTailscaleConfig(userID string) (*UserTailscaleConfig, error) {
	data, err := p.API.KVGet("tailscale_" + userID)
	if err != nil {
		return nil, err
	}

	if data == nil {
		return nil, nil
	}

	var config UserTailscaleConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

func (p *Plugin) handleACL(args *model.CommandArgs) error {
	config, err := p.getUserTailscaleConfig(args.UserId)
	if err != nil {
		return fmt.Errorf("failed to retrieve Tailscale configuration: %w", err)
	}

	if config == nil {
		p.postEphemeral(args.UserId, args.ChannelId, "Please authenticate first using: `/tailscale connect <tailnet> <api-key>`")
		return nil
	}

	client := tailscale.NewClient(config.Tailnet, tailscale.APIKey(config.APIKey))
	acl, err := client.ACL(context.Background())
	if err != nil {
		return fmt.Errorf("failed to retrieve ACL from Tailscale API: %w", err)
	}

	aclJSON, err := json.MarshalIndent(acl.ACL, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to format ACL: %w", err)
	}

	message := fmt.Sprintf("#### Tailscale ACL\n```json\n%s\n```", string(aclJSON))
	p.postEphemeral(args.UserId, args.ChannelId, message)
	return nil
}

func (p *Plugin) handleDisconnect(args *model.CommandArgs) error {
	config, err := p.getUserTailscaleConfig(args.UserId)
	if err != nil {
		return fmt.Errorf("failed to retrieve Tailscale configuration: %w", err)
	}

	if config == nil {
		p.postEphemeral(args.UserId, args.ChannelId, "Please authenticate first using: `/tailscale connect <tailnet> <api-key>`")
		return nil
	}

	if err := p.API.KVDelete("tailscale_" + args.UserId); err != nil {
		return fmt.Errorf("failed to remove Tailscale configuration: %w", err)
	}

	p.postEphemeral(args.UserId, args.ChannelId, fmt.Sprintf("Successfully disconnected from Tailnet: %s", config.Tailnet))
	return nil
}

func (p *Plugin) handleTailnet(args *model.CommandArgs) error {
	config, err := p.getUserTailscaleConfig(args.UserId)
	if err != nil {
		return fmt.Errorf("failed to retrieve Tailscale configuration: %w", err)
	}

	if config == nil {
		p.postEphemeral(args.UserId, args.ChannelId, "Please authenticate first using: `/tailscale connect <tailnet> <api-key>`")
		return nil
	}

	message := fmt.Sprintf("#### Your Tailnet\n%s", config.Tailnet)
	p.postEphemeral(args.UserId, args.ChannelId, message)
	return nil
}

func (p *Plugin) handleAbout(args *model.CommandArgs) error {
	text, err := command.BuildInfo(model.Manifest{
		Id:      manifest.Id,
		Version: manifest.Version,
		Name:    manifest.Name,
	})
	if err != nil {
		return fmt.Errorf("failed to get build info: %w", err)
	}

	p.postEphemeral(args.UserId, args.ChannelId, text)

	return nil
}

func (p *Plugin) postEphemeral(userID, channelID string, message string) {
	ephemeralPost := &model.Post{
		ChannelId: channelID,
		UserId:    p.botID,
		Message:   message,
	}
	p.client.Post.SendEphemeralPost(userID, ephemeralPost)
}
