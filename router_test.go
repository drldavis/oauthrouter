package oauthrouter_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/drldavis/oauthrouter"
)

func TestCallbacksDeriveDestinationsWithoutRegistrations(t *testing.T) {
	for _, base := range []string{"https://myapp.localhost", "http://myapp.localhost:1355"} {
		handler, err := oauthrouter.New(base, "/auth/a/callback", "/auth/b/callback")
		if err != nil {
			t.Fatal(err)
		}
		parsed, _ := url.Parse(base)
		for _, prefix := range []string{"", "feature-a", "fix-login"} {
			for _, path := range []string{"/auth/a/callback", "/auth/b/callback"} {
				for _, result := range []string{"code=secret%2Bcode", "error=access_denied&error_description=No+thanks"} {
					response := httptest.NewRecorder()
					handler.ServeHTTP(response, httptest.NewRequest("GET", path+"?state="+url.QueryEscape("v2."+prefix+".original.token+")+"&"+result, nil))
					if response.Code != 302 {
						t.Fatalf("status %d", response.Code)
					}
					target, _ := url.Parse(response.Header().Get("Location"))
					host := parsed.Host
					if prefix != "" {
						host = prefix + "." + host
					}
					if target.Scheme != parsed.Scheme || target.Host != host || target.Path != path || target.Query().Get("state") != "original.token+" {
						t.Fatalf("wrong destination: %s", target)
					}
					expected, _ := url.ParseQuery(result)
					for key := range expected {
						if target.Query().Get(key) != expected.Get(key) {
							t.Fatalf("lost %s", key)
						}
					}
					if response.Header().Get("Cache-Control") != "no-store" || response.Header().Get("Referrer-Policy") != "no-referrer" {
						t.Fatal("callback may leak")
					}
					if strings.Contains(response.Body.String(), "secret") {
						t.Fatal("callback leaked in body")
					}
				}
			}
		}
	}
}

func TestMalformedRoutingStateCannotEscapeBaseDomain(t *testing.T) {
	handler, _ := oauthrouter.New("https://myapp.localhost")
	for _, state := range []string{"", "plain", "v1.feature.token", "v2.feature.", "v2.-bad.token", "v2.bad-.token", "v2.bad_name.token", "v2.https://evil.example.token", "v2.user@evil.token", "v2.feature/path.token", "v2.%2e%2e.token", "v2." + strings.Repeat("a", 64) + ".token"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest("GET", "/oauth/callback?state="+url.QueryEscape(state)+"&code=code", nil))
		if response.Code != 400 || response.Header().Get("Location") != "" {
			t.Fatalf("accepted %q", state)
		}
	}
	for _, query := range []string{"state=v2.a.token&state=v2.b.token", "state=v2.a.token&code=a&code=b", "state=v2.a.token&error=a&error=b", "state=v2.a.token&code=a&error=denied", "state=%zz", "state=" + strings.Repeat("a", 9000)} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest("GET", "/oauth/callback?"+query, nil))
		if response.Code != 400 {
			t.Fatalf("accepted %q", query)
		}
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest("POST", "/oauth/callback", nil))
	if response.Code != 405 {
		t.Fatal(response.Code)
	}
}

func TestBaseURLAndCallbackPathsAreValidated(t *testing.T) {
	for _, base := range []string{"", "https://example.com", "https://localhost.evil.com", "http://localhost", "http://127.0.0.1:9090", "//myapp.localhost", "https://user@myapp.localhost", "https://myapp.localhost/path", "https://myapp.localhost?x=1", "https://myapp.localhost#", "ftp://myapp.localhost", "http://myapp.localhost:bad", "http://myapp.localhost:0", "https://bad_name.localhost"} {
		if _, err := oauthrouter.New(base); err == nil {
			t.Fatalf("accepted base %q", base)
		}
	}
	for _, path := range []string{"relative", "//external.example/callback", "/callback?query", "/callback#", "https://external.example/callback"} {
		if _, err := oauthrouter.New("https://myapp.localhost", path); err == nil {
			t.Fatalf("accepted path %q", path)
		}
	}
	handler, err := oauthrouter.New("https://myapp.localhost", "/custom/callback")
	if err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]int{"/custom/callback": 302, "/oauth/callback": 404} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path+"?state=v2..original&code=code", nil))
		if response.Code != want {
			t.Fatalf("%s: %d", path, response.Code)
		}
	}
}
