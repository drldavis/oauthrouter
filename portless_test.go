package oauthrouter_test

import (
	"net/http/httptest"
	"net/url"
	"os"
	"testing"

	"github.com/drldavis/oauthrouter"
)

func TestRouterDiscoversMainCheckoutOnce(t *testing.T) {
	for _, base := range []string{"https://myapp.localhost", "http://myapp.localhost:1355"} {
		t.Run(base, func(t *testing.T) {
			script, calls := fakePortless(t, base+"\n")
			t.Chdir(t.TempDir())
			handler, err := oauthrouter.NewFromPortless("myapp", "/auth/callback")
			if err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(calls)
			if err != nil {
				t.Fatal(err)
			}
			wd, _ := os.Getwd()
			if string(data) != wd+"|get myapp\n" {
				t.Fatalf("wrong discovery: %q", data)
			}
			if err := os.Remove(script); err != nil {
				t.Fatal(err)
			}
			for _, prefix := range []string{"", "feature-a"} {
				response := httptest.NewRecorder()
				handler.ServeHTTP(response, httptest.NewRequest("GET", "/auth/callback?state="+prefix+".original.token&code=code", nil))
				target, err := url.Parse(response.Header().Get("Location"))
				if err != nil {
					t.Fatal(err)
				}
				expected, _ := url.Parse(base)
				if prefix != "" {
					expected.Host = prefix + "." + expected.Host
				}
				if response.Code != 302 || target.Scheme != expected.Scheme || target.Host != expected.Host || target.Path != "/auth/callback" || target.Query().Get("state") != "original.token" || target.Query().Get("code") != "code" {
					t.Fatalf("wrong destination: %d %s", response.Code, target)
				}
			}
		})
	}
}

func TestRouterRejectsUnavailableOrInvalidPortlessDiscovery(t *testing.T) {
	t.Run("missing executable", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())
		if _, err := oauthrouter.NewFromPortless("myapp"); err == nil {
			t.Fatal("missing Portless accepted")
		}
	})
	for _, output := range []string{"", "not a URL", "https://evil.example", "https://other.localhost", "https://feature.myapp.localhost", "https://myapp.localhost/path"} {
		t.Run(output, func(t *testing.T) {
			fakePortless(t, output)
			if _, err := oauthrouter.NewFromPortless("myapp"); err == nil {
				t.Fatalf("accepted %q", output)
			}
		})
	}
	for _, name := range []string{"", "--help", "../myapp", "myapp.localhost"} {
		t.Run(name, func(t *testing.T) {
			_, calls := fakePortless(t, "https://myapp.localhost")
			if _, err := oauthrouter.NewFromPortless(name); err == nil {
				t.Fatalf("accepted app name %q", name)
			}
			if _, err := os.Stat(calls); !os.IsNotExist(err) {
				t.Fatal("executed invalid app name")
			}
		})
	}
}
