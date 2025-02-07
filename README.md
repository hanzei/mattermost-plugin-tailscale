# Tailscale Plugin for Mattermost

This plugin integrates Tailscale with Mattermost, allowing users to manage their Tailscale network directly from Mattermost.

## Features

- Expose your Mattermost instance securely via Tailscale
- List all devices in your Tailnet
- View ACL configurations
- Check your current Tailnet name

## Installation

1. Download the latest release from the [releases page](https://github.com/hanzei/mattermost-plugin-tailscale/releases)
2. Upload the plugin to your Mattermost instance through **System Console > Plugins > Plugin Management**
3. Enable the plugin

## Usage

The plugin adds a `/tailscale` slash command with the following subcommands:

- `/tailscale connect <tailnet> <api-key>` - Connect to your Tailscale network
- `/tailscale disconnect` - Disconnect from your Tailscale network
- `/tailscale list` - List all devices in your Tailnet
- `/tailscale acl` - Show the ACL configuration for your Tailnet
- `/tailscale tailnet` - Show your current Tailnet name
- `/tailscale serve setup <auth-key>` - Configure Tailscale serve with an auth key (System Admins only)
- `/tailscale serve status` - Check if Tailscale serve is running (System Admins only)
- `/tailscale serve start` - Start the Tailscale reverse proxy (System Admins only)
- `/tailscale serve stop` - Stop the Tailscale reverse proxy (System Admins only)

### Tailscale Serve

The Tailscale serve feature allows System Administrators to expose their Mattermost instance securely over Tailscale. This provides:

- Automatic HTTPS with valid certificates
- Access control via Tailscale ACLs
- No need for public IP addresses or port forwarding
- Simple setup with just an auth key

To use this feature:

1. Generate an auth key in your Tailscale admin console
2. Run `/tailscale serve setup <auth-key>` to configure the plugin
3. Run `/tailscale serve start` to start the reverse proxy
4. Update your Mattermost Site URL to match the Tailscale DNS name shown in the status message

## Development

Build your plugin:
```
make dist
```

This will produce a single plugin file (with support for multiple architectures) for upload to your Mattermost server:

```
dist/com.example.my-plugin.tar.gz
```

## Development

To avoid having to manually install your plugin, build and deploy your plugin using one of the following options. In order for the below options to work, you must first enable plugin uploads via your config.json or API and restart Mattermost.

```json
    "PluginSettings" : {
        ...
        "EnableUploads" : true
    }
```

### Deploying with Local Mode

If your Mattermost server is running locally, you can enable [local mode](https://docs.mattermost.com/administration/mmctl-cli-tool.html#local-mode) to streamline deploying your plugin. Edit your server configuration as follows:

```json
{
    "ServiceSettings": {
        ...
        "EnableLocalMode": true,
        "LocalModeSocketLocation": "/var/tmp/mattermost_local.socket"
    },
}
```

and then deploy your plugin:
```
make deploy
```

You may also customize the Unix socket path:
```bash
export MM_LOCALSOCKETPATH=/var/tmp/alternate_local.socket
make deploy
```

If developing a plugin with a webapp, watch for changes and deploy those automatically:
```bash
export MM_SERVICESETTINGS_SITEURL=http://localhost:8065
export MM_ADMIN_TOKEN=j44acwd8obn78cdcx7koid4jkr
make watch
```

### Deploying with credentials

Alternatively, you can authenticate with the server's API with credentials:
```bash
export MM_SERVICESETTINGS_SITEURL=http://localhost:8065
export MM_ADMIN_USERNAME=admin
export MM_ADMIN_PASSWORD=password
make deploy
```

or with a [personal access token](https://docs.mattermost.com/developer/personal-access-tokens.html):
```bash
export MM_SERVICESETTINGS_SITEURL=http://localhost:8065
export MM_ADMIN_TOKEN=j44acwd8obn78cdcx7koid4jkr
make deploy
```

### Releasing new versions

The version of a plugin is determined at compile time, automatically populating a `version` field in the [plugin manifest](plugin.json):
* If the current commit matches a tag, the version will match after stripping any leading `v`, e.g. `1.3.1`.
* Otherwise, the version will combine the nearest tag with `git rev-parse --short HEAD`, e.g. `1.3.1+d06e53e1`.
* If there is no version tag, an empty version will be combined with the short hash, e.g. `0.0.0+76081421`.

To disable this behaviour, manually populate and maintain the `version` field.

## How to Release

To trigger a release, follow these steps:

1. **For Patch Release:** Run the following command:
    ```
    make patch
    ```
   This will release a patch change.

2. **For Minor Release:** Run the following command:
    ```
    make minor
    ```
   This will release a minor change.

3. **For Major Release:** Run the following command:
    ```
    make major
    ```
   This will release a major change.

4. **For Patch Release Candidate (RC):** Run the following command:
    ```
    make patch-rc
    ```
   This will release a patch release candidate.

5. **For Minor Release Candidate (RC):** Run the following command:
    ```
    make minor-rc
    ```
   This will release a minor release candidate.

6. **For Major Release Candidate (RC):** Run the following command:
    ```
    make major-rc
    ```
   This will release a major release candidate.

## Q&A

### How do I make a server-only or web app-only plugin?

Simply delete the `server` or `webapp` folders and remove the corresponding sections from `plugin.json`. The build scripts will skip the missing portions automatically.


### How do I build the plugin with unminified JavaScript?
Setting the `MM_DEBUG` environment variable will invoke the debug builds. The simplist way to do this is to simply include this variable in your calls to `make` (e.g. `make dist MM_DEBUG=1`).
