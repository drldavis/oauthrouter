# oauthrouter

Use one OAuth callback URL for all your local Git worktrees. After you sign in
with a provider, oauthrouter sends your browser back to the copy of your app
that started the login. You don't register a new URL for every branch.

It works with [Portless](https://portless.sh/), which gives each copy its own
hostname, such as `feature-a.myapp.localhost`. Your app still handles login;
oauthrouter only gets the browser to the right place.

## Quickstart

You need Go 1.25.11+, Portless on your app's `PATH`, and an app with working OAuth
login. This guide adds local routing to that flow.

### 1. Start your app and the router

From your app's checkout, use the same Portless app name in every worktree.
**Separate frontend and backend?** See [Vite + Go](#vite--go) for which server to run.

```sh
portless run --name myapp <your-app-command>
```

In another terminal, from the **main checkout**:

```sh
go install github.com/drldavis/oauthrouter/cmd/oauthrouter@latest
oauthrouter serve -app-name myapp
```

Run one shared router for all worktrees. It calls `portless get myapp` at startup
to discover the main app's URL, including HTTP/HTTPS and port. Startup fails if
Portless is unavailable or returns a worktree URL instead of the main URL.

### 2. Set one callback address

A callback is where the provider sends your browser after sign-in. Set this
**shared router callback URL** in your provider's settings and your app's local
OAuth configuration:

```text
http://localhost:9090/oauth/callback
```

Use it as `redirect_uri` both when starting login and when exchanging the returned
code for tokens. Your app must handle `/oauth/callback`; see [other paths and
HTTPS requirements](docs/integration.md#options) if your setup differs.

The router automatically chooses the **worktree callback destination**, such as
`https://feature-a.myapp.localhost/oauth/callback`. Don't register that address
for each branch.

### 3. Wrap the outgoing state

Install the helper and import `github.com/drldavis/oauthrouter`:

```sh
go get github.com/drldavis/oauthrouter@latest
```

At startup, from your app's checkout:

```go
var wrapper *oauthrouter.Wrapper
if config.LocalOAuthRouting {
    var err error
    wrapper, err = oauthrouter.NewWrapper("myapp")
    if err != nil {
        return err
    }
}
```

`config.LocalOAuthRouting` is your own app setting. Enable it for local development.
Leave it false in production and keep your production callback URL; Portless is
never called when it is false.

OAuth uses `state` to verify that a callback belongs to the login your app started.
In the handler that starts login, keep generating and storing your original
state, then wrap only the value you send to the provider:

```go
providerState := originalState
if wrapper != nil {
    providerState = wrapper.Wrap(originalState)
}
// Pass providerState as the authorization request's state parameter.
```

Pass the wrapper from startup to that handler; see the [full handler
example](docs/integration.md#handler-integration). The helper discovers the
worktree with `portless get myapp`. The router restores the original state before
your callback handler receives it, so keep its existing validation.

### 4. Try a login

Open a worktree's Portless URL and sign in. You should return to that same
hostname. Keep login initiation and the final callback on the same browser
hostname so your app receives its verification cookie.

## Vite + Go

If Vite already proxies API requests to Go, keep that arrangement. Run **Vite**
through Portless, and pass Vite's app name to both `NewWrapper` and the router
(`-app-name`). Run the shared router from the main checkout. Proxy both login
and callback paths to the Go server for that worktree. The shared router callback URL stays the same.

See the [Vite proxy example and cookie explanation](docs/integration.md#vite--separate-backend).
Apps that already send login requests directly to a named backend can use the
[direct-backend setup](docs/integration.md#direct-backend-alternative).

## More details

- [Callback paths, ports, and HTTPS](docs/integration.md#options)
- [How state routing works](docs/integration.md#how-it-works)
- [Supported flows and responsibilities](docs/integration.md#scope)

## Development

Run `make test`, `make lint`, and `make build`. The binary is written to `bin/oauthrouter`.
