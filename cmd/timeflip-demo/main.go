package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	timeflip "timeflip-go"
	"timeflip-go/macos"
)

func main() {
	cfg, err := parseFlags(os.Args[1:])
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(0)
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	client, err := timeflip.NewClient(macos.NewTransport(), timeflip.Config{
		CommunicationTimeout: cfg.CommunicationTimeout,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	formatter := NewTextFormatter(os.Stdout, os.Stderr)
	app := NewDemoApp(client, cfg, NewTerminalPrompter(os.Stdin, os.Stdout), formatter)
	formatter.PrintLine("TimeFlip2 demo CLI. Type help for commands, exit to quit.")
	if err := app.Run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func parseFlags(args []string) (DemoConfig, error) {
	cfg := DemoConfig{EventBuffer: 16}
	fs := flag.NewFlagSet("timeflip-demo", flag.ContinueOnError)
	fs.DurationVar(&cfg.CommunicationTimeout, "timeout", 0, "global communication timeout, for example 10s")
	fs.DurationVar(&cfg.CommandTimeout, "command-timeout", 0, "per-command timeout override, for example 5s")
	fs.IntVar(&cfg.EventBuffer, "event-buffer", cfg.EventBuffer, "event channel buffer size")
	fs.BoolVar(&cfg.IncludeRawEvents, "include-raw", false, "include raw event bytes when streaming")
	fs.BoolVar(&cfg.IncludeUnsupportedDevices, "include-unsupported", false, "include unsupported BLE devices in list output")
	if err := fs.Parse(args); err != nil {
		return DemoConfig{}, err
	}
	if cfg.CommunicationTimeout < 0 || cfg.CommandTimeout < 0 {
		return DemoConfig{}, fmt.Errorf("timeouts must be non-negative")
	}
	if cfg.EventBuffer < 0 {
		return DemoConfig{}, fmt.Errorf("event-buffer must be non-negative")
	}
	if cfg.CommunicationTimeout == 0 {
		cfg.CommunicationTimeout = 10 * time.Second
	}
	return cfg, nil
}
