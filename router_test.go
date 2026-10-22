package oauthrouter_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/drldavis/oauthrouter"
)

func TestCallbacksReturnToTheirOwnWorktree(t *testing.T) {
	handler, err := oauthrouter.New(map[string]string{"one": "https://one.myapp.localhost", "two": "http://localhost:5174"}, "/oauth/alpha/callback", "/oauth/beta/callback")
	if err != nil {
		t.Fatal(err)
	}
	for _, provider := range []string{"alpha", "beta"} {
		for name, origin := range map[string]string{"one": "https://one.myapp.localhost", "two": "http://localhost:5174"} {
			for _, result := range []string{"code=secret%2Bcode", "error=access_denied&error_description=No+thanks"} {
				request := httptest.NewRequest("GET", "/oauth/"+provider+"/callback?state="+url.QueryEscape(oauthrouter.WrapState(name, "original-token"))+"&"+result, nil)
				response := httptest.NewRecorder()
				handler.ServeHTTP(response, request)
				if response.Code != http.StatusFound {
					t.Fatalf("status %d", response.Code)
				}
				target, err := url.Parse(response.Header().Get("Location"))
				if err != nil {
					t.Fatal(err)
				}
				if target.Scheme+"://"+target.Host != origin || target.Path != request.URL.Path || target.Query().Get("state") != "original-token" {
					t.Fatalf("wrong destination: %s", target)
				}
				expected, _ := url.ParseQuery(result)
				for key := range expected {
					if target.Query().Get(key) != expected.Get(key) {
						t.Fatalf("lost %s", key)
					}
				}
				if response.Header().Get("Cache-Control") != "no-store" || response.Header().Get("Referrer-Policy") != "no-referrer" {
					t.Fatal("callback may leak through cache/referrer")
				}
				if strings.Contains(response.Body.String(), "secret") {
					t.Fatal("callback leaked in body")
				}
			}
		}
	}
}

func TestRouterRejectsUnapprovedAndMalformedCallbacks(t *testing.T) {
	handler, _ := oauthrouter.New(map[string]string{"one": "http://localhost:5173"})
	for _, query := range []string{"", "state=plain", "state=v1.unknown.token", "state=v1.../token", "state=v1.one.", "state=v1.one.token&state=v1.one.other", "state=v1.one.token&code=a&code=b", "state=v1.one.token&code=a&error=denied", "state=%zz", "state=" + strings.Repeat("a", 9000)} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest("GET", "/oauth/callback?"+query, nil))
		if response.Code != http.StatusBadRequest || response.Header().Get("Location") != "" {
			t.Fatalf("accepted %q: %d", query, response.Code)
		}
	}
	for _, path := range []string{"/", "/oauth/other/callback", "/oauth/callback/extra"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest("GET", path, nil))
		if response.Code != 404 {
			t.Fatalf("path %s: %d", path, response.Code)
		}
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest("POST", "/oauth/callback", nil))
	if response.Code != 405 {
		t.Fatal(response.Code)
	}
}

func TestDestinationsMustBeLocalOrigins(t *testing.T) {
	for _, origin := range []string{"https://example.com", "https://localhost.evil.com", "http://0.0.0.0:80", "//localhost", "https://user@localhost", "https://localhost/path", "https://localhost?x=1", "https://localhost#fragment", "http://localhost#", "ftp://localhost", "http://localhost:bad"} {
		if _, err := oauthrouter.New(map[string]string{"one": origin}); err == nil {
			t.Fatalf("accepted %s", origin)
		}
	}
	for _, name := range []string{"../one", "one.two", "", "one/two"} {
		if _, err := oauthrouter.New(map[string]string{name: "http://localhost"}); err == nil {
			t.Fatalf("accepted name %q", name)
		}
	}
	if oauthrouter.WrapState("", "original") != "original" {
		t.Fatal("disabled routing changed state")
	}
}

func TestCallbackPathsAreExplicitlyConfigured(t *testing.T) {
	for _, path := range []string{"relative", "//external.example/callback", "/callback?query", "/callback#", "https://external.example/callback"} {
		if _, err := oauthrouter.New(nil, path); err == nil {
			t.Fatalf("accepted path %q", path)
		}
	}
	handler, err := oauthrouter.New(map[string]string{"one": "http://localhost:5173"}, "/auth/custom/callback")
	if err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]int{"/auth/custom/callback": 302, "/oauth/callback": 404} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest("GET", path+"?state=v1.one.original&code=code", nil))
		if response.Code != want {
			t.Fatalf("%s: %d", path, response.Code)
		}
	}
}
