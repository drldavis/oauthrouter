package main

import "testing"

func TestCommandRejectsPublicListenerAndInvalidDestinations(t *testing.T) {
	directory := t.TempDir()
	for _, args := range [][]string{
		{"serve", "-listen", "0.0.0.0:9090"},
		{"register", "-name", "feature", "-origin", "https://example.com"},
		{"register", "-name", "../escape", "-origin", "http://localhost"},
		{"unknown"},
	} {
		args = append(args, "-routes", directory)
		if err := run(args); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
}
