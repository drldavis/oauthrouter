# oauthrouter

Use one OAuth callback URL for all your local Git worktrees. After sign-in,
oauthrouter redirects your browser to the worktree that started the login.

## Prerequisites

- Go 1.25.11+ and npm.
- A Go app with an existing OAuth login flow.

## Quickstart

### 1. Install Portless

```sh
npm install -g portless
```

Run your app through [Portless](https://github.com/vercel-labs/portless), using the
same app name in every worktree. For Vite + Go, [run Vite through Portless](docs/integration.md#vite--separate-backend).

```sh
portless run --name myapp <your-app-command>
```

### 2. Install the router globally

```sh
go install github.com/drldavis/oauthrouter/cmd/oauthrouter@latest
```

Ensure Go's binary install directory is on your `PATH`.

### 3. Wrap the outgoing state

In your Go app:

```sh
go get github.com/drldavis/oauthrouter@latest
```

Import `github.com/drldavis/oauthrouter` and create the wrapper at startup:

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

Enable your app's `config.LocalOAuthRouting` setting locally; leave it false in
production. Pass the wrapper to your login handler ([full example](docs/integration.md#handler-integration)).

Keep storing the original state. Wrap only the state you send to the provider:

```go
providerState := originalState
if wrapper != nil {
    providerState = wrapper.Wrap(originalState)
}
```

### 4. Set up provider callback URLs

Register this **shared router callback URL** with each provider and use it as
`redirect_uri` in your app's local authorization and code-exchange requests:

```text
http://localhost:9090/oauth/callback
```

Your app must handle `/oauth/callback`. Keep your production callback URL unchanged.
For other paths or HTTPS, see [configuration options](docs/integration.md#options).

### 5. Run the router from main

From your app's `main` or `master` branch, run one shared router:

```sh
oauthrouter serve -app-name myapp
```

Keep it running, open your app at a worktree's Portless URL, and sign in.

## More details

- [Full Go handler example](docs/integration.md#handler-integration)
- [Vite + separate backend](docs/integration.md#vite--separate-backend)
- [Cookies and callback routing](docs/integration.md#cookies)
- [How it works](docs/integration.md#how-it-works)

## Development

Run `make test`, `make lint`, and `make build`. The binary is written to `bin/oauthrouter`.
