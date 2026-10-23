// Package oauthrouter routes local OAuth callbacks to Portless worktree URLs.
package oauthrouter

import (
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

var labelPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`)

func validHostname(host string) bool {
	if len(host) > 253 {
		return false
	}
	for _, label := range strings.Split(host, ".") {
		if !labelPattern.MatchString(label) {
			return false
		}
	}
	return true
}

func parseOrigin(origin string) (*url.URL, error) {
	target, err := url.Parse(origin)
	if err != nil {
		return nil, errors.New("invalid base URL")
	}
	if (target.Scheme != "http" && target.Scheme != "https") || target.User != nil || target.Path != "" || target.RawQuery != "" || target.ForceQuery || strings.Contains(origin, "#") || !strings.HasSuffix(target.Hostname(), ".localhost") || !validHostname(target.Hostname()) {
		return nil, errors.New("expected an HTTP(S) .localhost origin without a path, query or fragment")
	}
	if port := target.Port(); port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return nil, errors.New("invalid origin port")
		}
	}
	return target, nil
}

// New derives destinations under baseURL; it does not read Git or registration files.
// An empty worktree prefix routes to the base URL itself (the main checkout).
func New(baseURL string, callbackPaths ...string) (http.Handler, error) {
	base, err := parseOrigin(baseURL)
	if err != nil {
		return nil, err
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
		if len(parts) != 3 || parts[0] != "v2" || parts[2] == "" || (parts[1] != "" && !labelPattern.MatchString(parts[1])) {
			http.Error(w, "invalid routing state", 400)
			return
		}
		destination := *base
		if parts[1] != "" {
			destination.Host = parts[1] + "." + base.Host
		}
		if len(destination.Hostname()) > 253 {
			http.Error(w, "invalid routing state", 400)
			return
		}
		destination.Path = r.URL.Path
		query.Set("state", parts[2])
		destination.RawQuery = query.Encode()
		w.Header().Set("Location", destination.String())
		w.WriteHeader(http.StatusFound)
	}), nil
}
