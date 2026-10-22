# oauthrouter

A standalone Go server that redirects local OAuth callbacks to the worktree that
started the connection. It also exports a small Go library for wrapping state and
managing approved destinations. The implementation uses only the Go standard
library.

## Build and run

Requires Go 1.25.11 or newer.

```sh
go install github.com/drldavis/oauthrouter/cmd/oauthrouter@v0.1.0
```

Or clone the repository and run `make build`; the binary is written to `bin/oauthrouter`.

From your application's checkout, register a worktree and start the server:

```sh
oauthrouter register \
  -name feature -origin https://feature.myapp.localhost
oauthrouter serve
```

It listens on `127.0.0.1:9090`. By default, registrations live in the calling
repository's shared Git directory under `oauth-router/`, so its worktrees share
one registry. Outside Git, pass `-routes /path/to/registrations` to every command.
Use one server per registry. Registrations are reread on each callback.

The default callback path is `/oauth/callback`. Configure additional or different
paths explicitly (supplying flags replaces the default):

```sh
oauthrouter serve -callback-path /auth/provider-a/callback -callback-path /auth/provider-b/callback
```

Register the server's stable local callback URL with each provider, using a scheme
and hostname that provider accepts. The server itself speaks HTTP; an existing
local HTTPS proxy can sit in front of it when HTTPS is required. No public tunnel
is needed for browser redirects to an accepted local URL.

## Application integration

The application preserves its existing random OAuth state and browser binding,
then wraps only the state sent to the provider:

```go
import "github.com/drldavis/oauthrouter"

providerState := oauthrouter.WrapState(worktreeName, originalState)
```

This produces `v1.<worktree-name>.<original-state>`. An empty worktree name leaves
state unchanged. The router resolves the name against its local allowlist and
redirects the browser to the approved origin and original callback path, restoring
the original state and preserving success or denial parameters.

The application still validates the original state, browser cookie, expiry and
single-use attempt, then exchanges the code using the registered router callback
URI. Each worktree retains its own database and credentials. The router does not
exchange codes, store credentials, or authenticate OAuth attempts.

Add the library to a consuming Go project with:

```sh
go get github.com/drldavis/oauthrouter@v0.1.0
```

## Commands

```sh
oauthrouter serve -listen 127.0.0.1:9090 -routes /path/to/registrations
oauthrouter register -name feature -origin https://feature.myapp.localhost -routes /path/to/registrations
oauthrouter unregister -name feature -routes /path/to/registrations
```

Worktree names accept letters, digits, underscores and hyphens. Origins must be
HTTP(S) on loopback addresses or `localhost`/`.localhost`, with no path, query or
fragment. Listeners must use a loopback IP. There is no HTTP registration endpoint;
registrations are local files and updates are atomic.

Use distinct hostnames for simultaneous connection flows: localhost ports share
cookies. Start and finish the flow on the same worktree hostname. Portless or
another local proxy can supply those hostnames; installing that proxy is separate.

All registered worktrees and local configuration are trusted. This tool is for
local development, not public or production authentication. Responses suppress
caching and referrers; callback parameters and authorization codes are not logged.

Provider-specific environment setup, lifecycle hooks, and provider integration
tests belong in the consuming project.

## Verify

```sh
make test
make lint
make build
```

Tests cover callbacks across multiple destinations, provider denials, invalid
requests/origins, live registration changes, and command validation. Live provider
login and callback URL acceptance need to be checked in the consuming application.
