package main

import "testing"

func TestCommandRejectsInvalidConfigurationAndRemovedCommands(t *testing.T) {
	for _, args := range [][]string{
		{"serve"},
		{"serve", "-base-url", "https://myapp.localhost", "-listen", "0.0.0.0:9090"},
		{"serve", "-base-url", "https://example.com"},
		{"serve", "-base-url", "https://myapp.localhost", "-callback-path", "//external.example"},
		{"serve", "-base-url", "https://myapp.localhost", "unexpected"},
		{"register"}, {"unregister"}, {"unknown"},
	} {
		if err := run(args); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
}
