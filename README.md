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

Run your server from its checkout, using the same app name in each worktree.
For separate servers, see [Vite + separate backend](#vite--separate-backend).

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

- **Shared router callback URL:** `http://localhost:9090/oauth/callback`. Register
  it with the provider and supply it as `redirect_uri` during authorization and
  code exchange.
- **Worktree callback destination:** for example,
  `https://feature-a.myapp.localhost/oauth/callback`. oauthrouter redirects the
  browser here afterward, using the worktree discovered by `portless get`.
  You do not configure a callback URL for each branch.

Your app must handle `/oauth/callback`; see [Options](#options) for other paths.
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
before redirecting the browser to the worktree callback destination. Your callback
handler needs no wrapper.

Start a login from a worktree's Portless URL to check the full flow. Keep the
authorization start and final callback on the same hostname (see
[Cookies](#cookies)).

## Vite + separate backend

If Vite already proxies API requests to Go, keep that arrangement. Run **Vite**
through Portless, and use its app name and base URL for oauthrouter. Run these
in separate terminals:

```sh
portless run --name myapp vite
oauthrouter serve -base-url https://myapp.localhost
```

In Go, use `oauthrouter.NewWrapper("myapp")` in the startup example above.
Run Go from the same worktree so `portless get myapp` discovers that worktree's
Vite hostname. Match `-base-url` to Vite's Portless scheme and port, without the
worktree prefix. The shared router callback URL registered with the provider
remains `http://localhost:9090/oauth/callback`.

Add callback forwarding alongside your existing API routes in Vite's
[`server.proxy`](https://vite.dev/config/server-options#server-proxy) configuration:

```js
server: {
  proxy: {
    "/api": "http://localhost:8080",
    "/oauth": "http://localhost:8080",
  },
}
```

`8080` is illustrative: both targets must point to the Go server belonging to
**that worktree**. Start authorization through a proxied route, such as
`/api/login` or `/oauth/login`; if using the earlier `/login` example, proxy that
path too. Proxy your actual callback paths if they differ from `/oauth/callback`.

```text
OAuth provider
  → shared oauthrouter callback: http://localhost:9090/oauth/callback
  → worktree's Vite hostname: https://feature-a.myapp.localhost/oauth/callback
  → Vite forwards the callback request to that worktree's Go server
```

oauthrouter **redirects the browser** to the worktree callback destination.
Vite **forwards the HTTP request internally** to Go. Go validates the callback
and exchanges the code, using the shared router callback URL as `redirect_uri`.

### Cookies

For the common host-only cookie setup (no `Domain` attribute), the authorization-start
response sets a cookie for the hostname the browser requested. The browser must
send that cookie to the callback handler later. Keep authorization initiation
and the final callback on the same hostname.

Vite's proxy preserves this arrangement: Go generates and validates the cookie,
but the browser receives it from and sends it to the worktree's Vite hostname.
The proxy passes it to Go with the callback request. The cookie's `Path` must
cover the callback, and its `Secure` and `SameSite` settings must allow the flow.
oauthrouter itself does not need the application's cookie.

### Direct-backend alternative

If the browser already sends authorization-start requests directly to a separately
named backend, use **that backend's** Portless name in `NewWrapper` and its base
URL in `-base-url`. The worktree callback destination will then be the backend's
hostname, matching the authorization-start hostname.

Do not bypass an existing Vite API proxy just to adopt oauthrouter; that can add
credentialed cross-origin requests and unnecessary CORS configuration. Vite,
proxying, and environment variables are application choices, not oauthrouter
requirements.

## Options

Use `-callback-path` for a different path, or repeat it for multiple providers:

```sh
oauthrouter serve -base-url https://myapp.localhost \
  -callback-path /auth/provider-a/callback \
  -callback-path /auth/provider-b/callback
```

These replace the default `/oauth/callback`. Register the matching shared router
callback URLs with your providers; each path is preserved when redirecting to your app.

Use `-listen 127.0.0.1:9091` to change the port, and update your shared router
callback URLs to match. The listen address must be a loopback IP.

## How it works

The wrapper sends state as `<worktree-prefix>.<original-state>`:

| State sent to provider | Worktree callback destination hostname | State received by app |
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
