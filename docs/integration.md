# Integration guide

Start with the [quickstart](../README.md#quickstart). This guide covers handler
wiring, separate development servers, and routing details.

## Handler integration

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
mux.HandleFunc("/oauth/login", authHandler.Login)
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
through Portless, and use its app name for oauthrouter. Run these in separate
terminals:

```sh
# In each worktree:
portless run --name myapp vite

# Once, from the main or master branch:
oauthrouter serve -app-name myapp
```

In Go, use `oauthrouter.NewWrapper("myapp")` in the [startup example above](#handler-integration).
Run Go from the same worktree so `portless get myapp` discovers that worktree's
Vite hostname. Run the shared router from `main` or `master` to discover the
Vite URL without a worktree prefix, including scheme and port. The shared router callback URL registered
with the provider remains `http://localhost:9090/oauth/callback`.

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
`/api/login` or `/oauth/login` (used in the handler example). Proxy your actual callback paths if they differ from `/oauth/callback`.

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
named backend, use **that backend's** Portless name in both `NewWrapper` and
`-app-name`. Run the shared router from `main` or `master`. The worktree callback
destination will then be the backend's
hostname, matching the authorization-start hostname.

Do not bypass an existing Vite API proxy just to adopt oauthrouter; that can add
credentialed cross-origin requests and unnecessary CORS configuration. Vite,
proxying, and environment variables are application choices, not oauthrouter
requirements.

## Options

The provider must accept a local HTTP shared router callback URL. If it requires
HTTPS, put a local HTTPS proxy in front of oauthrouter and use that HTTPS URL as
the shared router callback URL.

Use `-callback-path` for a different path, or repeat it for multiple providers:

```sh
oauthrouter serve -app-name myapp \
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
credentials. Each redirect logs its destination hostname and path. Query parameters, including
authorization codes and state, are not logged. Responses disable caching and referrers.

