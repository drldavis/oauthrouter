package oauthrouter

import (
	"fmt"
	"strings"
)

// Wrapper remembers the Portless worktree prefix discovered at startup.
// It is immutable and safe to share across concurrent authorization requests.
type Wrapper struct{ prefix string }

// NewWrapper runs "portless get <appName>" once in the application's working
// directory. Portless must be installed on PATH and use its .localhost naming.
func NewWrapper(appName string) (*Wrapper, error) {
	origin, err := discoverOrigin(appName)
	if err != nil {
		return nil, err
	}
	baseHost := appName + ".localhost"
	host := origin.Hostname()
	if host == baseHost {
		return &Wrapper{}, nil
	}
	prefix, found := strings.CutSuffix(host, "."+baseHost)
	if !found || !labelPattern.MatchString(prefix) {
		return nil, fmt.Errorf("portless URL does not match app %q or one of its worktrees", appName)
	}
	return &Wrapper{prefix: prefix}, nil
}

// Wrap preserves the application's original state inside a routing envelope.
// Call this only when sending state to the provider; keep the original for validation.
func (w *Wrapper) Wrap(state string) string { return w.prefix + "." + state }
