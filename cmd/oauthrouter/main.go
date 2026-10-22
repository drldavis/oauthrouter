package main

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/drldavis/oauthrouter"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: oauthrouter serve|register|unregister [flags]")
	}
	flags := flag.NewFlagSet(args[0], flag.ContinueOnError)
	directory := flags.String("routes", "", "shared registrations directory (default: Git common directory/oauth-router)")
	name := flags.String("name", "", "worktree name")
	origin := flags.String("origin", "", "worktree browser origin, e.g. https://feature.myapp.localhost")
	var callbackPaths pathsFlag
	flags.Var(&callbackPaths, "callback-path", "accepted callback path (repeatable; default /oauth/callback)")
	listen := flags.String("listen", "127.0.0.1:9090", "loopback listen address")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("unexpected arguments")
	}
	if *directory == "" {
		output, err := exec.Command("git", "rev-parse", "--path-format=absolute", "--git-common-dir").Output()
		if err != nil {
			return errors.New("run inside the repository or specify -routes")
		}
		*directory = filepath.Join(strings.TrimSpace(string(output)), "oauth-router")
	}
	switch args[0] {
	case "register":
		return oauthrouter.Register(*directory, *name, *origin)
	case "unregister":
		return oauthrouter.Unregister(*directory, *name)
	case "serve":
		if _, err := oauthrouter.New(nil, callbackPaths...); err != nil {
			return err
		}
		host, _, err := net.SplitHostPort(*listen)
		ip := net.ParseIP(host)
		if err != nil || ip == nil || !ip.IsLoopback() {
			return errors.New("listen address must use a loopback IP")
		}
		server := &http.Server{Addr: *listen, Handler: oauthrouter.DirectoryHandler(*directory, callbackPaths...), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 16384}
		fmt.Fprintf(os.Stderr, "OAuth callback router listening on %s\n", *listen)
		return server.ListenAndServe()
	default:
		return errors.New("unknown command")
	}
}

type pathsFlag []string

func (p *pathsFlag) String() string         { return strings.Join(*p, ",") }
func (p *pathsFlag) Set(value string) error { *p = append(*p, value); return nil }
