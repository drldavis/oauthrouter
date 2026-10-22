package oauthrouter

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// Register atomically replaces one approved destination, without a shared-file lock.
func Register(directory, name, origin string) error {
	if !ValidName(name) {
		return errors.New("invalid worktree name")
	}
	if err := ValidateOrigin(origin); err != nil {
		return err
	}
	if err := os.MkdirAll(directory, 0700); err != nil {
		return err
	}
	data, err := json.Marshal(origin)
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(directory, ".route-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err = file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), filepath.Join(directory, name+".json"))
}

func Unregister(directory, name string) error {
	if !ValidName(name) {
		return errors.New("invalid worktree name")
	}
	err := os.Remove(filepath.Join(directory, name+".json"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// DirectoryHandler reloads local registrations for each callback. It exposes no
// HTTP endpoint for registering destinations and never stores provider credentials.
func DirectoryHandler(directory string, callbackPaths ...string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origins := map[string]string{}
		entries, err := os.ReadDir(directory)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			http.Error(w, "cannot read worktree registrations", 503)
			return
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(directory, entry.Name()))
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			var origin string
			if err != nil || json.Unmarshal(data, &origin) != nil {
				http.Error(w, "invalid worktree registration", 503)
				return
			}
			origins[strings.TrimSuffix(entry.Name(), ".json")] = origin
		}
		handler, err := New(origins, callbackPaths...)
		if err != nil {
			http.Error(w, "invalid worktree registration", 503)
			return
		}
		handler.ServeHTTP(w, r)
	})
}
