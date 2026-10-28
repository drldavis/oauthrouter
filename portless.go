package oauthrouter

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os/exec"
	"strings"
	"time"
)

// NewFromPortless discovers the base URL with "portless get <appName>".
// Run it from the main or master branch so Portless returns a URL without a worktree prefix.
func NewFromPortless(appName string, callbackPaths ...string) (http.Handler, error) {
	origin, err := discoverOrigin(appName)
	if err != nil {
		return nil, err
	}
	if origin.Hostname() != appName+".localhost" {
		return nil, fmt.Errorf("expected a URL without a worktree prefix for %q; run oauthrouter from the main or master branch (Portless returned %s)", appName, origin)
	}
	return New(origin.String(), callbackPaths...)
}

func discoverOrigin(appName string) (*url.URL, error) {
	if !validHostname(appName) || strings.HasSuffix(appName, ".localhost") {
		return nil, fmt.Errorf("app name must be a DNS name without the .localhost suffix")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "portless", "get", appName)
	command.WaitDelay = time.Second
	output, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("discover app URL with portless get %q: %w", appName, err)
	}
	origin, err := parseOrigin(strings.TrimSpace(string(output)))
	if err != nil {
		return nil, fmt.Errorf("portless returned an invalid app URL: %w", err)
	}
	return origin, nil
}
