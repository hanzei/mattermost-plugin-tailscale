package main

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path/filepath"
	"strings"

	"tailscale.com/tsnet"
)

func (p *Plugin) startTSSever() error {
	config := p.getConfiguration()

	fileDir := *p.client.Configuration.GetConfig().FileSettings.Directory
	stateDir := filepath.Join(fileDir, "plugin-data", manifest.Id)

	// Start serve
	tsServer := &tsnet.Server{
		Dir:      stateDir,
		Hostname: "mattermost",
		AuthKey:  config.AuthKey,
	}
	ln, err := tsServer.ListenTLS("tcp", ":443")
	if err != nil {
		return fmt.Errorf("Failed to listen: %w", err)
	}
	p.tsServer = tsServer

	// Create a reverse proxy
	la := *p.client.Configuration.GetConfig().ServiceSettings.ListenAddress
	if strings.HasPrefix(la, ":") {
		la = "localhost" + la
	}
	target, err := url.Parse("http://" + la)
	if err != nil {
		return fmt.Errorf("Failed to parse ListenAddress: %w", err)
	}

	proxy := httputil.NewSingleHostReverseProxy(target)
	h := reverseHandler{proxy: proxy}

	// Serve HTTP traffic over Tailscale
	go func() {
		// TODO: Add stop signal
		if err := http.Serve(ln, h); err != nil {
			log.Fatalf("Failed to start HTTP server: %v", err)
		}
	}()

	return nil
}

type reverseHandler struct {
	proxy *httputil.ReverseProxy
}

func (h reverseHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("Forwarding request: %s %s\n", r.Method, r.URL.Path)
	h.proxy.ServeHTTP(w, r)
}
