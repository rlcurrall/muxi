package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/BurntSushi/toml"
	"github.com/joho/godotenv"
	"github.com/rlcurrall/muxi/pkg/multiplexer"
	"golang.org/x/sync/errgroup"
)

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

func main() {
	env := os.Environ()
	godotenv.Load()

	var conf MuxiConfig
	if _, err := os.Stat("muxi.toml"); !os.IsNotExist(err) {
		if _, err := toml.DecodeFile("muxi.toml", &conf); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}

	if len(conf.Commands) == 0 {
		fmt.Println("No commands configured")
		os.Exit(0)
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

	err := wg.Wait()
	if err != nil {
		fmt.Print("err", err)
	}
}
