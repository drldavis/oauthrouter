package main

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
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
	if len(args) == 0 || args[0] != "serve" {
		return errors.New("usage: oauthrouter serve -app-name myapp [-listen 127.0.0.1:9090]")
	}
	flags := flag.NewFlagSet("serve", flag.ContinueOnError)
	appName := flags.String("app-name", "", "Portless app name; run from its main checkout (required)")
	listen := flags.String("listen", "127.0.0.1:9090", "loopback listen address")
	var callbackPaths pathsFlag
	flags.Var(&callbackPaths, "callback-path", "accepted callback path (repeatable; default /oauth/callback)")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("unexpected arguments")
	}
	handler, err := oauthrouter.NewFromPortless(*appName, callbackPaths...)
	if err != nil {
		return err
	}
	host, _, err := net.SplitHostPort(*listen)
	ip := net.ParseIP(host)
	if err != nil || ip == nil || !ip.IsLoopback() {
		return errors.New("listen address must use a loopback IP")
	}
	server := &http.Server{Addr: *listen, Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 16384}
	fmt.Fprintf(os.Stderr, "OAuth callback router listening on %s; Portless app %s\n", *listen, *appName)
	return server.ListenAndServe()
}

type pathsFlag []string

func (p *pathsFlag) String() string         { return strings.Join(*p, ",") }
func (p *pathsFlag) Set(value string) error { *p = append(*p, value); return nil }
