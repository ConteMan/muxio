package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	"github.com/ConteMan/muxio/internal/config"
	"github.com/ConteMan/muxio/internal/paths"
)

const configUsage = `Usage:
  muxio config path              Print the config file path without creating it
  muxio config show              Print the effective config and where each value came from
  muxio config init              Write a commented default config, refusing to overwrite
  muxio config set <key> <value> Change one setting, for example server.addr
`

func runConfig(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		_, _ = io.WriteString(stderr, configUsage)
		return exitUsage
	}

	switch args[0] {
	case "path":
		return runConfigPath(args[1:], stdout, stderr)
	case "show":
		return runConfigShow(args[1:], stdout, stderr)
	case "init":
		return runConfigInit(args[1:], stdout, stderr)
	case "set":
		return runConfigSet(args[1:], stdout, stderr)
	default:
		_, _ = fmt.Fprintf(stderr, "unknown config subcommand %q\n\n%s", args[0], configUsage)
		return exitUsage
	}
}

func runConfigPath(args []string, stdout, stderr io.Writer) int {
	if len(args) != 0 {
		_, _ = fmt.Fprintln(stderr, "config path takes no arguments")
		return exitUsage
	}
	configPath, err := paths.ConfigFile()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "config path: %v\n", err)
		return exitError
	}
	_, _ = fmt.Fprintln(stdout, configPath)
	return exitOK
}

func runConfigShow(args []string, stdout, stderr io.Writer) int {
	if len(args) != 0 {
		_, _ = fmt.Fprintln(stderr, "config show takes no arguments")
		return exitUsage
	}

	loaded, err := loadConfig()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "config show: %v\n", err)
		return exitError
	}

	source := loaded.Path
	if !loaded.Exists {
		source += "  (not created, using defaults)"
	}
	_, _ = fmt.Fprintf(stdout, "file  %s\n\n", source)

	table := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(table, "SETTING\tVALUE\tFROM")
	for _, key := range config.Keys() {
		value, err := config.Get(loaded.Config, key)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "config show: %v\n", err)
			return exitError
		}
		_, _ = fmt.Fprintf(table, "%s\t%s\t%s\n", key, value, loaded.Origin(key))
	}
	if err := table.Flush(); err != nil {
		_, _ = fmt.Fprintf(stderr, "config show: %v\n", err)
		return exitError
	}
	return exitOK
}

func runConfigInit(args []string, stdout, stderr io.Writer) int {
	if len(args) != 0 {
		_, _ = fmt.Fprintln(stderr, "config init takes no arguments")
		return exitUsage
	}

	if _, err := paths.EnsureConfigDir(); err != nil {
		_, _ = fmt.Fprintf(stderr, "config init: %v\n", err)
		return exitError
	}
	configPath, err := paths.ConfigFile()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "config init: %v\n", err)
		return exitError
	}

	if err := config.Create(configPath, config.Default()); err != nil {
		_, _ = fmt.Fprintf(stderr, "config init: %v\n", err)
		if errors.Is(err, config.ErrExists) {
			return exitError
		}
		return exitError
	}
	_, _ = fmt.Fprintf(stdout, "wrote %s\n", configPath)
	return exitOK
}

func runConfigSet(args []string, stdout, stderr io.Writer) int {
	if len(args) != 2 {
		_, _ = io.WriteString(stderr, configUsage)
		return exitUsage
	}
	key, value := args[0], args[1]

	loaded, err := loadConfig()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "config set: %v\n", err)
		return exitError
	}

	// Only the file's own values are carried forward. Environment overrides are
	// not written back: doing so would silently bake a temporary override into
	// the durable configuration.
	onFile, err := fileConfig(loaded)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "config set: %v\n", err)
		return exitError
	}

	updated, err := config.Set(onFile, key, value)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "config set: %v\n", err)
		return exitUsage
	}

	if _, err := paths.EnsureConfigDir(); err != nil {
		_, _ = fmt.Fprintf(stderr, "config set: %v\n", err)
		return exitError
	}
	if err := config.Write(loaded.Path, updated, loaded.ModTime); err != nil {
		_, _ = fmt.Fprintf(stderr, "config set: %v\n", err)
		return exitError
	}

	_, _ = fmt.Fprintf(stdout, "%s = %s\n", key, value)
	if loaded.Origin(key) == config.FromEnv {
		_, _ = fmt.Fprintf(stderr,
			"note: %s is currently overridden by the environment, so this change will not take effect until that is unset\n",
			key)
	}
	return exitOK
}

// fileConfig rebuilds the configuration as it exists on disk, ignoring the
// environment, so that a write preserves only durable values.
func fileConfig(loaded config.Loaded) (config.Config, error) {
	result := config.Default()
	for _, key := range config.Keys() {
		if loaded.Origin(key) != config.FromFile {
			continue
		}
		value, err := config.Get(loaded.Config, key)
		if err != nil {
			return config.Config{}, err
		}
		result, err = config.Set(result, key, value)
		if err != nil {
			return config.Config{}, err
		}
	}
	return result, nil
}

// loadConfig resolves the effective configuration from file and environment.
func loadConfig() (config.Loaded, error) {
	configPath, err := paths.ConfigFile()
	if err != nil {
		return config.Loaded{}, err
	}
	return config.Load(configPath, os.LookupEnv)
}
