# oauthrouter

Route local OAuth callbacks to the Portless worktree that started the connection.
A standalone Go server and a small Go helper, using only the standard library.

## Setup

Requires Go 1.25.11 or newer and [Portless](https://portless.sh/) on your application's
PATH. Use Portless's default `.localhost` domains.

```sh
go install github.com/drldavis/oauthrouter/cmd/oauthrouter@v0.2.0
go get github.com/drldavis/oauthrouter@v0.2.0
```

Run your app through `portless run --name myapp <your-app-command>` from its checkout.
Start the callback server from any directory:

```sh
oauthrouter serve -base-url https://myapp.localhost
```

It listens on `127.0.0.1:9090`, with `/oauth/callback` as its default callback path.
Configure each provider and your application's OAuth redirect URI to use
`http://localhost:9090/oauth/callback` (if that provider accepts local HTTP).
Use that same URI when exchanging the authorization code.
For other callback paths, supply one flag per path:

```sh
oauthrouter serve -base-url https://myapp.localhost \
  -callback-path /auth/provider-a/callback \
  -callback-path /auth/provider-b/callback
```

Flags replace the default callback path. `-listen 127.0.0.1:9090` changes the
loopback listener. The server speaks HTTP; providers requiring HTTPS need a local
HTTPS proxy in front of it. The base URL must match Portless's scheme, app name,
and port; for example `http://myapp.localhost:1355` for that Portless configuration.

## Application integration

Create a wrapper once during application startup, from the app's checkout:

```go
wrapper, err := oauthrouter.NewWrapper("myapp")
if err != nil {
    return err
}
```

When building the provider's authorization request:

```go
providerState := wrapper.Wrap(originalState)
```

Keep storing and validating `originalState` as before. Send only `providerState`
to the provider. The helper runs `portless get myapp` once at startup to discover
the worktree prefix, so every worktree uses the same code. Startup fails if
Portless is missing or returns an incompatible URL. No subprocess runs per login.

The server restores the original state before redirecting:

| Checkout | Wrapped state | Destination |
| --- | --- | --- |
| Main | `v2..nonce` | `https://myapp.localhost/oauth/callback` |
| Worktree with prefix `feature-a` | `v2.feature-a.nonce` | `https://feature-a.myapp.localhost/oauth/callback` |

The callback path and other query parameters are preserved. The server needs no
Git checkout, registration directory, or per-worktree setup. It constructs the
destination under its configured base URL. Start and finish a flow on the same
worktree hostname so the application's browser cookie remains available.

## Boundaries

This is a local development tool for trusted worktrees. It handles GET query
callbacks using OAuth `state`; POST callbacks and flows without state are not
supported. Custom Portless TLDs are not supported.

The application still validates its random state, browser cookie, expiry, and
single-use attempt. The router does not authenticate attempts, exchange codes,
or store credentials. Destinations stay under the configured `.localhost` base;
responses disable caching and referrers. Callback codes are not logged.

Version 0.2 replaces the 0.1 registration API and `v1` state format. Remove
register/unregister hooks and `WrapState` calls, use `NewWrapper` instead, and
restart outstanding login flows. The server and consuming app must be upgraded
together.

## Development

```sh
make test
make lint
make build
```

The binary is written to `bin/oauthrouter`. Tests cover startup discovery,
main/worktree routing, original-state restoration, provider denials, malformed
callbacks, destination validation, and CLI validation. Live provider login and
callback URL acceptance must be checked in the consuming application.
