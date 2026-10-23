# oauthrouter

Use one OAuth callback URL for all your local Git worktrees. oauthrouter sends
each login back to the copy of your app that started it, so you don't need to
register a callback URL for every branch.

It works with [Portless](https://portless.sh/), which gives each worktree its own
local hostname:

```text
feature-a.myapp.localhost → OAuth provider
                                  ↓
                   localhost:9090/oauth/callback
                                  ↓
                feature-a.myapp.localhost/oauth/callback
```

Your app still completes the login. The router just redirects the browser to
the right worktree.

## Setup

Requires Go 1.25.11+, an existing OAuth flow, and Portless on your app's `PATH`.

### 1. Start your app and the router

Run your app from its checkout, using the same app name in each worktree:

```sh
portless run --name myapp <your-app-command>
```

In another terminal, install and start the router:

```sh
go install github.com/drldavis/oauthrouter/cmd/oauthrouter@latest
oauthrouter serve -base-url https://myapp.localhost
```

Use your app's base URL **without a worktree prefix**, matching Portless's scheme
and port (for example, `http://myapp.localhost:1355`). The router listens on
`127.0.0.1:9090` by default.

### 2. Set the callback URL

In your provider settings and your app's local OAuth configuration, use:

```text
http://localhost:9090/oauth/callback
```

Use the same redirect URI when exchanging the authorization code. Your app must
handle callbacks at `/oauth/callback`; see [Options](#options) for other paths.
Providers requiring HTTPS need a local HTTPS proxy in front of the router.

### 3. Add the wrapper to your app

```sh
go get github.com/drldavis/oauthrouter@latest
```

Import `github.com/drldavis/oauthrouter`. Pass the wrapper into your auth handler,
and wrap the outgoing state after storing the original in the user's session:

```go
type AuthHandler struct {
    wrapper *oauthrouter.Wrapper
}

func NewAuthHandler(wrapper *oauthrouter.Wrapper) *AuthHandler {
    return &AuthHandler{wrapper: wrapper}
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
    // Generate a random state and store the original in the user's session.
    originalState, err := generateAndStoreOAuthState(w, r)
    if err != nil {
        http.Error(w, "could not start login", http.StatusInternalServerError)
        return
    }

    providerState := originalState
    if h.wrapper != nil {
        providerState = h.wrapper.Wrap(originalState)
    }

    authorizationURL := buildAuthorizationURL(providerState)
    http.Redirect(w, r, authorizationURL, http.StatusFound)
}
```

Then wire it up during application startup:

```go
var wrapper *oauthrouter.Wrapper
if config.LocalOAuthRouting {
    // Run from the app's checkout, using the same app name as Portless.
    var err error
    wrapper, err = oauthrouter.NewWrapper("myapp")
    if err != nil {
        return err
    }
}

authHandler := NewAuthHandler(wrapper)
mux.HandleFunc("/login", authHandler.Login)
```

`generateAndStoreOAuthState` and `buildAuthorizationURL` stand for your app's
existing OAuth code. Pass `providerState` as the authorization request's `state`
parameter, keeping your other provider parameters unchanged.

`config.LocalOAuthRouting` is your app's flag. Leave it false in production:
the wrapper stays nil, Portless is never called, and state is sent unchanged.
Use your normal production callback URL there.

When enabled, `NewWrapper` runs `portless get myapp` once from your app's checkout.
It returns an error if discovery fails. The router restores the original state
before forwarding the callback, so your callback handler needs no wrapper.

Start a login from a worktree's Portless URL to check the full flow. You should
return to that same hostname, where your app can read its session cookie and
validate the original state.

## Options

Use `-callback-path` for a different path, or repeat it for multiple providers:

```sh
oauthrouter serve -base-url https://myapp.localhost \
  -callback-path /auth/provider-a/callback \
  -callback-path /auth/provider-b/callback
```

These replace the default `/oauth/callback`. Register the matching router URLs
with your providers; each path is preserved when redirecting to your app.

Use `-listen 127.0.0.1:9091` to change the port, and update your redirect URIs to
match. The listen address must be a loopback IP.

## How it works

The wrapper sends state as `<worktree-prefix>.<original-state>`:

| State sent to provider | Destination hostname | State received by app |
| --- | --- | --- |
| `.nonce` | `myapp.localhost` | `nonce` |
| `feature-a.nonce` | `feature-a.myapp.localhost` | `nonce` |

The router splits at the first dot, preserving any dots in the original state.
Callback paths and other query parameters are preserved. No worktree registration
is needed.

## Scope

This is for local development with trusted worktrees and Portless `.localhost`
domains. It supports GET callbacks with OAuth `state`, not POST callbacks or
flows without state.

Your app still validates state, cookies, expiration, and single-use attempts,
and exchanges codes for tokens. The router does not authenticate users or store
credentials. Callback codes are not logged; responses disable caching and referrers.

## Development

```sh
make test
make lint
make build
```

The binary is written to `bin/oauthrouter`.
