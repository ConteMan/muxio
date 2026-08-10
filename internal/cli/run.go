package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/ConteMan/muxio/internal/api"
	"github.com/ConteMan/muxio/internal/version"
)

const usage = `Muxio is a local-first personal information capture core.

Usage:
  muxio <command>

Commands:
  serve      Start the local HTTP service
  version    Print build version
  help       Show this help
`

// Run executes the CLI and returns a process exit code.
func Run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		_, _ = io.WriteString(stdout, usage)
		return 0
	}

	switch args[0] {
	case "help", "-h", "--help":
		_, _ = io.WriteString(stdout, usage)
		return 0
	case "version":
		_, _ = fmt.Fprintln(stdout, version.String())
		return 0
	case "serve":
		return runServe(ctx, args[1:], stdout, stderr)
	default:
		_, _ = fmt.Fprintf(stderr, "unknown command %q\n\n%s", args[0], usage)
		return 64
	}
}

func runServe(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("serve", flag.ContinueOnError)
	flags.SetOutput(stderr)
	addr := flags.String("addr", "127.0.0.1:8080", "HTTP listen address")
	if err := flags.Parse(args); err != nil {
		return 64
	}
	if flags.NArg() != 0 {
		_, _ = fmt.Fprintln(stderr, "serve does not accept positional arguments")
		return 64
	}
	if err := validateLoopbackAddress(*addr); err != nil {
		_, _ = fmt.Fprintf(stderr, "invalid --addr: %v\n", err)
		return 64
	}

	server := &http.Server{
		Addr:              *addr,
		Handler:           api.NewHandler(version.Version),
		ReadHeaderTimeout: 5 * time.Second,
	}

	stopped := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_ = server.Shutdown(shutdownCtx)
		case <-stopped:
		}
	}()

	_, _ = fmt.Fprintf(stdout, "muxio listening on http://%s\n", *addr)
	err := server.ListenAndServe()
	close(stopped)
	if err == nil || errors.Is(err, http.ErrServerClosed) {
		return 0
	}
	_, _ = fmt.Fprintf(stderr, "serve: %v\n", err)
	return 1
}

func validateLoopbackAddress(addr string) error {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return err
	}
	if strings.EqualFold(host, "localhost") {
		return nil
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return errors.New("only loopback addresses are allowed")
	}
	return nil
}
