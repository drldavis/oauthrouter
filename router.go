// Package oauthrouter routes development OAuth callbacks to approved local worktrees.
package oauthrouter

import (
	"errors"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

var namePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,100}$`)

func ValidName(name string) bool { return namePattern.MatchString(name) }

// WrapState leaves normal OAuth unchanged when development routing is disabled.
// The original state remains the application's browser-bound, one-time secret.
func WrapState(worktree, state string) string {
	if worktree == "" {
		return state
	}
	return "v1." + worktree + "." + state
}

func ValidateOrigin(origin string) error {
	target, err := url.Parse(origin)
	if err != nil {
		return errors.New("invalid local origin")
	}
	host := target.Hostname()
	ip := net.ParseIP(host)
	local := host == "localhost" || strings.HasSuffix(host, ".localhost") || (ip != nil && ip.IsLoopback())
	if !local || (target.Scheme != "http" && target.Scheme != "https") || target.User != nil || target.Path != "" || target.RawQuery != "" || target.ForceQuery || strings.Contains(origin, "#") || target.Opaque != "" {
		return errors.New("destination must be a local HTTP(S) origin without a path, query or fragment")
	}
	if port := target.Port(); port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return errors.New("invalid origin port")
		}
	}
	return nil
}

// New copies the explicit allowlist; no destination is taken from callback URLs.
func New(origins map[string]string, callbackPaths ...string) (http.Handler, error) {
	destinations := make(map[string]string, len(origins))
	for name, origin := range origins {
		if !ValidName(name) {
			return nil, errors.New("invalid worktree name")
		}
		if err := ValidateOrigin(origin); err != nil {
			return nil, err
		}
		destinations[name] = origin
	}
	if len(callbackPaths) == 0 {
		callbackPaths = []string{"/oauth/callback"}
	}
	paths := make(map[string]bool, len(callbackPaths))
	for _, path := range callbackPaths {
		parsed, err := url.Parse(path)
		if err != nil || !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") || parsed.Path != path || strings.ContainsAny(path, "?#") {
			return nil, errors.New("callback paths must be absolute URL paths without query or fragment")
		}
		paths[path] = true
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Referrer-Policy", "no-referrer")
		if !paths[r.URL.Path] {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			http.Error(w, "method not allowed", 405)
			return
		}
		query, err := url.ParseQuery(r.URL.RawQuery)
		if err != nil || len(r.URL.RawQuery) > 8192 || len(query["state"]) != 1 || len(query["code"]) > 1 || len(query["error"]) > 1 || (query.Get("code") != "" && query.Get("error") != "") {
			http.Error(w, "invalid callback", 400)
			return
		}
		parts := strings.SplitN(query.Get("state"), ".", 3)
		if len(parts) != 3 || parts[0] != "v1" || parts[2] == "" {
			http.Error(w, "invalid routing state", 400)
			return
		}
		origin, ok := destinations[parts[1]]
		if !ok {
			http.Error(w, "unknown worktree", 400)
			return
		}
		query.Set("state", parts[2])
		w.Header().Set("Location", origin+r.URL.Path+"?"+query.Encode())
		w.WriteHeader(http.StatusFound)
	}), nil
}
