package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/ConteMan/muxio/internal/api"
	"github.com/ConteMan/muxio/internal/app"
	"github.com/ConteMan/muxio/internal/config"
	"github.com/ConteMan/muxio/internal/logging"
	"github.com/ConteMan/muxio/internal/paths"
	"github.com/ConteMan/muxio/internal/store/sqlite"
	"github.com/ConteMan/muxio/internal/version"
)

const usage = `Muxio is a local-first personal information capture core.

Usage:
  muxio <command>

Commands:
  serve      Start the local HTTP service
  import     Read JSONL capture records from stdin
  runs       Show run history and what happened during a run
  config     Inspect and change settings
  db         Inspect the local database
  version    Print build version
  help       Show this help
`

const dbUsage = `Usage:
  muxio db path    Print the database path without creating it
`

// Exit codes follow sysexits: 64 is a usage error, 1 is a runtime failure.
const (
	exitOK    = 0
	exitError = 1
	exitUsage = 64
)

// errLoopbackOnly reports a listen address outside the loopback interface.
// ADR-002 forbids remote listening until authentication and a threat model exist.
var errLoopbackOnly = errors.New("only loopback addresses are allowed")

// Run executes the CLI and returns a process exit code.
func Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		_, _ = io.WriteString(stdout, usage)
		return exitOK
	}

	switch args[0] {
	case "help", "-h", "--help":
		_, _ = io.WriteString(stdout, usage)
		return exitOK
	case "version":
		_, _ = fmt.Fprintln(stdout, version.String())
		return exitOK
	case "serve":
		return runServe(ctx, args[1:], stdout, stderr)
	case "import":
		return runImport(ctx, args[1:], stdin, stdout, stderr)
	case "runs":
		return runRuns(ctx, args[1:], stdout, stderr)
	case "config":
		return runConfig(args[1:], stdout, stderr)
	case "db":
		return runDB(ctx, args[1:], stdout, stderr)
	default:
		_, _ = fmt.Fprintf(stderr, "unknown command %q\n\n%s", args[0], usage)
		return exitUsage
	}
}

func runImport(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("import", flag.ContinueOnError)
	flags.SetOutput(stderr)
	sourceName := flags.String("source", "", "name of the source to import into")
	logLevel := flags.String("log-level", "", "debug, info, warn or error")
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if flags.NArg() != 0 {
		_, _ = fmt.Fprintln(stderr, "import reads records from stdin and takes no positional arguments")
		return exitUsage
	}
	if strings.TrimSpace(*sourceName) == "" {
		_, _ = fmt.Fprintln(stderr, "import requires --source")
		return exitUsage
	}

	loaded, err := loadConfig()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "import: %v\n", err)
		return exitError
	}
	if isFlagSet(flags, "log-level") {
		if err := loaded.Override("log.level", *logLevel); err != nil {
			_, _ = fmt.Fprintf(stderr, "import: %v\n", err)
			return exitUsage
		}
	}

	logger, err := newLogger(loaded.Config.Log.Level, stderr)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "import: %v\n", err)
		return exitUsage
	}

	store, cleanup, err := openStore(ctx)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "import: %v\n", err)
		return exitError
	}
	defer cleanup()

	result, err := app.ImportJSONL(ctx, store, stdin, logger,
		strings.TrimSpace(*sourceName), importOptions(loaded.Config))
	// Counts are reported even on failure: knowing what landed matters most
	// exactly when something went wrong. The run id points at the full story.
	_, _ = fmt.Fprintf(stdout, "run=%d imported=%d duplicate=%d failed=%d\n",
		result.RunID, result.Imported, result.Duplicate, result.Failed)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "import: %v\n", err)
		return exitError
	}
	if result.Failed > 0 {
		_, _ = fmt.Fprintf(stderr, "see `muxio runs show %d` for the rejected lines\n", result.RunID)
		return exitError
	}
	return exitOK
}

// newLogger builds the logger from the already-resolved configuration.
func newLogger(level string, stderr io.Writer) (*slog.Logger, error) {
	parsed, err := logging.ParseLevel(level)
	if err != nil {
		return nil, err
	}
	return logging.New(stderr, parsed), nil
}

// importOptions translates configuration into the settings the use case needs.
func importOptions(c config.Config) app.Options {
	return app.Options{
		MaxBodyBytes:   c.Capture.MaxBodyBytes,
		EventRetention: c.RunEventRetention(),
	}
}

// isFlagSet reports whether the user actually passed a flag, as opposed to it
// holding its zero value. Only flags that were set may override the file.
func isFlagSet(flags *flag.FlagSet, name string) bool {
	found := false
	flags.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
}

func runDB(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		_, _ = io.WriteString(stderr, dbUsage)
		return exitUsage
	}

	switch args[0] {
	case "path":
		if len(args) != 1 {
			_, _ = fmt.Fprintln(stderr, "db path takes no arguments")
			return exitUsage
		}
		databasePath, err := paths.Database()
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "db path: %v\n", err)
			return exitError
		}
		_, _ = fmt.Fprintln(stdout, databasePath)
		return exitOK
	default:
		_, _ = fmt.Fprintf(stderr, "unknown db subcommand %q\n\n%s", args[0], dbUsage)
		return exitUsage
	}
}

// openStore resolves the data directory, opens the database, and migrates it.
func openStore(ctx context.Context) (*sqlite.Store, func(), error) {
	if _, err := paths.EnsureHome(); err != nil {
		return nil, nil, err
	}
	databasePath, err := paths.Database()
	if err != nil {
		return nil, nil, err
	}

	store, err := sqlite.Open(ctx, databasePath)
	if err != nil {
		return nil, nil, err
	}
	if err := store.Migrate(ctx); err != nil {
		_ = store.Close()
		return nil, nil, err
	}
	return store, func() { _ = store.Close() }, nil
}

func runServe(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("serve", flag.ContinueOnError)
	flags.SetOutput(stderr)
	addr := flags.String("addr", "", "HTTP listen address (overrides config)")
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if flags.NArg() != 0 {
		_, _ = fmt.Fprintln(stderr, "serve does not accept positional arguments")
		return exitUsage
	}

	loaded, err := loadConfig()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "serve: %v\n", err)
		return exitError
	}
	if isFlagSet(flags, "addr") {
		if err := loaded.Override("server.addr", *addr); err != nil {
			_, _ = fmt.Fprintf(stderr, "invalid --addr: %v\n", err)
			return exitUsage
		}
	}
	listenAddr := loaded.Config.Server.Addr

	if err := validateLoopbackAddress(listenAddr); err != nil {
		_, _ = fmt.Fprintf(stderr, "invalid listen address: %v\n", err)
		return exitUsage
	}

	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "serve: %v\n", err)
		return exitError
	}
	// The pre-flight check only inspects the requested string. Verify the address
	// we actually bound so a hostname resolving off loopback cannot slip through.
	if err := requireLoopbackAddr(listener.Addr()); err != nil {
		_ = listener.Close()
		_, _ = fmt.Fprintf(stderr, "invalid --addr: %v\n", err)
		return exitUsage
	}

	server := &http.Server{
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

	_, _ = fmt.Fprintf(stdout, "muxio listening on http://%s\n", listener.Addr())
	err = server.Serve(listener)
	close(stopped)
	if err == nil || errors.Is(err, http.ErrServerClosed) {
		return exitOK
	}
	_, _ = fmt.Fprintf(stderr, "serve: %v\n", err)
	return exitError
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
		return errLoopbackOnly
	}
	return nil
}

func requireLoopbackAddr(addr net.Addr) error {
	tcpAddr, ok := addr.(*net.TCPAddr)
	if !ok || !tcpAddr.IP.IsLoopback() {
		return errLoopbackOnly
	}
	return nil
}
