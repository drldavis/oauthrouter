package oauthrouter_test

import (
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/drldavis/oauthrouter"
)

func fakePortless(t *testing.T, output string) (string, string) {
	t.Helper()
	directory := t.TempDir()
	calls := filepath.Join(directory, "calls")
	script := filepath.Join(directory, "portless")
	body := "#!/bin/sh\nprintf '%s\\n' \"$PWD|$*\" >> \"$CALLS_FILE\"\nprintf '%s\\n' \"$PORTLESS_TEST_URL\"\n"
	if err := os.WriteFile(script, []byte(body), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", directory)
	t.Setenv("CALLS_FILE", calls)
	t.Setenv("PORTLESS_TEST_URL", output)
	return script, calls
}

func TestWrapperDiscoversPortlessOnceAndRoutesMainAndLinkedWorktrees(t *testing.T) {
	for _, test := range []struct{ output, destination, prefix string }{
		{"https://myapp.localhost", "https://myapp.localhost", ""},
		{"https://feature-login.myapp.localhost\n", "https://feature-login.myapp.localhost", "feature-login"},
		{"http://feature-login.myapp.localhost:1355", "http://feature-login.myapp.localhost:1355", "feature-login"},
	} {
		t.Run(test.destination, func(t *testing.T) {
			script, calls := fakePortless(t, test.output)
			dir := t.TempDir()
			t.Chdir(dir)
			wrapper, err := oauthrouter.NewWrapper("myapp")
			if err != nil {
				t.Fatal(err)
			}
			data, _ := os.ReadFile(calls)
			wd, _ := os.Getwd()
			if string(data) != wd+"|get myapp\n" {
				t.Fatalf("wrong discovery: %q", data)
			}
			// Wrapping must continue to work without starting Portless on every request.
			if err := os.Remove(script); err != nil {
				t.Fatal(err)
			}
			base := "https://myapp.localhost"
			if strings.HasPrefix(test.output, "http:") {
				base = "http://myapp.localhost:1355"
			}
			router, _ := oauthrouter.New(base)
			for _, original := range []string{"random-one", "random.two+three"} {
				if got := wrapper.Wrap(original); got != test.prefix+"."+original {
					t.Fatalf("wrong wrapped state: %q", got)
				}
				response := httptest.NewRecorder()
				router.ServeHTTP(response, httptest.NewRequest("GET", "/oauth/callback?state="+url.QueryEscape(wrapper.Wrap(original))+"&code=code", nil))
				target, _ := url.Parse(response.Header().Get("Location"))
				if response.Code != 302 || target.Scheme+"://"+target.Host != test.destination || target.Query().Get("state") != original {
					t.Fatalf("wrong roundtrip: %d %s", response.Code, target)
				}
			}
		})
	}
}

func TestWrapperFailsClearlyForMissingPortlessAndUnexpectedOutput(t *testing.T) {
	t.Run("missing executable", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())
		if _, err := oauthrouter.NewWrapper("myapp"); err == nil {
			t.Fatal("missing executable accepted")
		}
	})
	for _, output := range []string{"", "not a URL", "https://evil.example", "https://other.localhost", "https://nested.feature.myapp.localhost", "https://bad_name.myapp.localhost", "https://myapp.localhost/path", "https://myapp.localhost?x=1", "https://myapp.localhost#"} {
		t.Run(output, func(t *testing.T) {
			fakePortless(t, output)
			if _, err := oauthrouter.NewWrapper("myapp"); err == nil {
				t.Fatalf("accepted %q", output)
			}
		})
	}
	for _, app := range []string{"", "--help", "myapp; echo bad", "../myapp"} {
		t.Run(app, func(t *testing.T) {
			_, calls := fakePortless(t, "https://myapp.localhost")
			if _, err := oauthrouter.NewWrapper(app); err == nil {
				t.Fatal("invalid app accepted")
			}
			if _, err := os.Stat(calls); !os.IsNotExist(err) {
				t.Fatal("executed invalid app")
			}
		})
	}
}
