package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	timeflip "github.com/mitchellrj/timeflip-go"
	"github.com/mitchellrj/timeflip-go/macos"
)

func main() {
	cfg, err := parseFlags(os.Args[1:])
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(0)
		}
		writeLine(os.Stderr, err)
		os.Exit(2)
	}
	transport := timeflip.Transport(macos.NewTransport())
	var traceFile *os.File
	if cfg.TraceBLEPath != "" {
		traceWriter := os.Stderr
		if cfg.TraceBLEPath != "-" {
			traceFile, err = os.Create(cfg.TraceBLEPath)
			if err != nil {
				writeLine(os.Stderr, err)
				os.Exit(1)
			}
			defer func() {
				_ = traceFile.Close()
			}()
			traceWriter = traceFile
		}
		transport = NewTracingTransport(transport, traceWriter)
	}
	client, err := timeflip.NewClient(transport, timeflip.Config{
		CommunicationTimeout: cfg.CommunicationTimeout,
	})
	if err != nil {
		writeLine(os.Stderr, err)
		os.Exit(1)
	}
	color := colorEnabled(os.Stdout, cfg.NoColor)
	formatter := NewTextFormatterWithColor(os.Stdout, os.Stderr, color)
	app := NewDemoApp(client, cfg, NewTerminalPrompter(os.Stdin, os.Stdout), formatter)
	formatter.PrintLine("TimeFlip2 demo CLI. Type help for commands, exit to quit.")
	if err := app.Run(context.Background()); err != nil {
		writeLine(os.Stderr, err)
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
	fs.BoolVar(&cfg.NoColor, "no-color", false, "disable ANSI color output")
	fs.StringVar(&cfg.TraceBLEPath, "trace-ble", "", "write raw BLE operation trace to PATH, or '-' for stderr; trace includes password bytes")
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

func colorEnabled(out *os.File, disabled bool) bool {
	if disabled || os.Getenv("NO_COLOR") != "" {
		return false
	}
	if strings.EqualFold(os.Getenv("TERM"), "dumb") {
		return false
	}
	return isTerminalFile(out)
}
