package main

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func commandPortless(t *testing.T, output string) {
	t.Helper()
	directory := t.TempDir()
	script := "#!/bin/sh\nprintf '%s\\n' \"$PORTLESS_TEST_URL\"\n"
	if err := os.WriteFile(filepath.Join(directory, "portless"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", directory)
	t.Setenv("PORTLESS_TEST_URL", output)
}

func TestCommandRejectsInvalidConfigurationAndRemovedCommands(t *testing.T) {
	commandPortless(t, "https://myapp.localhost")
	for _, test := range []struct {
		args    []string
		message string
	}{
		{[]string{"serve"}, "app name"},
		{[]string{"serve", "-app-name", "myapp", "-listen", "0.0.0.0:9090"}, "loopback"},
		{[]string{"serve", "-app-name", "https://example.com"}, "app name"},
		{[]string{"serve", "-app-name", "myapp", "-callback-path", "//external.example"}, "callback paths"},
		{[]string{"serve", "-app-name", "myapp", "unexpected"}, "unexpected arguments"},
		{[]string{"serve", "-base-url", "https://myapp.localhost"}, "flag provided but not defined"},
		{[]string{"register"}, "usage:"}, {[]string{"unregister"}, "usage:"}, {[]string{"unknown"}, "usage:"},
	} {
		if err := run(test.args); err == nil || !strings.Contains(err.Error(), test.message) {
			t.Fatalf("run(%v) = %v; want %q", test.args, err, test.message)
		}
	}
}

func TestCommandDiscoversAppBeforeStartingListener(t *testing.T) {
	commandPortless(t, "http://myapp.localhost:1355")
	// Occupy the requested port so the command returns after successful setup.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	err = run([]string{"serve", "-app-name", "myapp", "-listen", listener.Addr().String()})
	var networkError *net.OpError
	if !errors.As(err, &networkError) || networkError.Op != "listen" {
		t.Fatalf("expected listener error after discovery, got %v", err)
	}
}

func TestCommandRejectsLinkedWorktree(t *testing.T) {
	commandPortless(t, "https://feature.myapp.localhost")
	err := run([]string{"serve", "-app-name", "myapp"})
	if err == nil || !strings.Contains(err.Error(), "main checkout") {
		t.Fatalf("expected main checkout guidance, got %v", err)
	}
}
