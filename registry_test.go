package oauthrouter_test

import (
	"net/http/httptest"
	"testing"

	"github.com/drldavis/oauthrouter"
)

func TestRegistrationChangesApplyWithoutRestart(t *testing.T) {
	directory := t.TempDir()
	handler := oauthrouter.DirectoryHandler(directory)
	check := func(want int, location string) {
		t.Helper()
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest("GET", "/oauth/callback?state=v1.branch.original&code=code", nil))
		if response.Code != want || response.Header().Get("Location") != location {
			t.Fatalf("got %d %s", response.Code, response.Header().Get("Location"))
		}
	}
	check(400, "")
	if err := oauthrouter.Register(directory, "branch", "https://branch.myapp.localhost"); err != nil {
		t.Fatal(err)
	}
	check(302, "https://branch.myapp.localhost/oauth/callback?code=code&state=original")
	if err := oauthrouter.Register(directory, "branch", "http://localhost:5174"); err != nil {
		t.Fatal(err)
	}
	check(302, "http://localhost:5174/oauth/callback?code=code&state=original")
	if err := oauthrouter.Unregister(directory, "branch"); err != nil {
		t.Fatal(err)
	}
	check(400, "")
	if err := oauthrouter.Register(directory, "../escape", "http://localhost"); err == nil {
		t.Fatal("accepted traversal")
	}
	if err := oauthrouter.Register(directory, "branch", "https://external.example"); err == nil {
		t.Fatal("accepted external destination")
	}
}
