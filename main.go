package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/BurntSushi/toml"
	"github.com/charmbracelet/log"
	"github.com/joho/godotenv"
	gap "github.com/muesli/go-app-paths"
	"github.com/rlcurrall/muxi/pkg/multiplexer"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

var (
	// Version as provided by goreleaser.
	Version = ""
	// CommitSHA as provided by goreleaser.
	CommitSHA = ""

	rootCmd = &cobra.Command{
		Use:           "muxi",
		Short:         "A simple multiplexer",
		SilenceErrors: false,
		RunE:          execute,
	}
)

func main() {
	closer, err := setupLog()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if err := rootCmd.Execute(); err != nil {
		_ = closer()
		os.Exit(1)
	}

	_ = closer()
}

func getLogFilePath() (string, error) {
	dir, err := gap.NewScope(gap.User, "muxi").CacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "muxi.log"), nil
}

func setupLog() (func() error, error) {
	log.SetOutput(io.Discard)
	// Log to file, if set
	logFile, err := getLogFilePath()
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(filepath.Dir(logFile), 0o755); err != nil {
		// log disabled
		return func() error { return nil }, nil
	}

	f, err := os.OpenFile(logFile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		// log disabled
		return func() error { return nil }, nil
	}

	log.SetOutput(f)
	log.SetLevel(log.DebugLevel)
	return f.Close, nil
}

func execute(cmd *cobra.Command, args []string) error {
	env := os.Environ()
	godotenv.Load()

	var conf MuxiConfig
	if _, err := os.Stat("muxi.toml"); !os.IsNotExist(err) {
		if _, err := toml.DecodeFile("muxi.toml", &conf); err != nil {
			log.Errorf("Error loading conif file: %s", err)
			return err
		}
	}

	if len(conf.Commands) == 0 {
		log.Warn("No commands configured")
		return errors.New("No commands configured")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	interruptChannel := make(chan os.Signal, 1)
	signal.Notify(interruptChannel, syscall.SIGINT)
	go func() {
		<-interruptChannel
		cancel()
	}()

	mux := multiplexer.New(ctx)
	for _, c := range conf.Commands {
		mux.AddProcess(c.Name, c.Args, c.Icon, c.Name, c.Cwd, true, c.Autostart, env...)
	}

	var wg errgroup.Group
	wg.Go(func() error {
		defer cancel()
		mux.Start()
		return nil
	})

	if err := wg.Wait(); err != nil {
		log.Errorf("An error occurred in wait group: %s", err)
		return err
	}

	return nil
}

type PackageJSON struct {
	Scripts map[string]string `json:"scripts"`
}

type MuxiConfig struct {
	AddNpmScripts bool `toml:"add_npm_scripts"`
	Commands      []Command
}

type Command struct {
	Name      string
	Args      []string
	Autostart bool
	Icon      string
	Cwd       string
}
