package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	timeflip "github.com/mitchellrj/timeflip-go"
)

type DemoConfig struct {
	CommunicationTimeout      time.Duration
	CommandTimeout            time.Duration
	EventBuffer               int
	IncludeRawEvents          bool
	IncludeUnsupportedDevices bool
	NoColor                   bool
}

type DemoState struct {
	SelectedDeviceID   timeflip.DeviceID
	ActiveSession      *timeflip.Session
	Authorized         bool
	ActiveStreamCancel context.CancelFunc
}

func (s *DemoState) SetSelectedDevice(id timeflip.DeviceID) {
	s.SelectedDeviceID = id
}

func (s *DemoState) SetSession(session *timeflip.Session) {
	s.ActiveSession = session
	s.Authorized = false
}

func (s *DemoState) ClearSession() {
	s.ActiveSession = nil
	s.Authorized = false
	s.ActiveStreamCancel = nil
}

type DemoApp struct {
	client   *timeflip.Client
	cfg      DemoConfig
	state    DemoState
	in       InputPrompter
	out      *TextFormatter
	commands map[string]demoCommand
	mu       sync.Mutex
}

func NewDemoApp(client *timeflip.Client, cfg DemoConfig, in InputPrompter, out *TextFormatter) *DemoApp {
	app := &DemoApp{client: client, cfg: cfg, in: in, out: out}
	app.commands = buildCommands()
	return app
}

func (a *DemoApp) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			a.cleanup(context.Background())
			return ctx.Err()
		default:
		}
		line, err := a.in.Prompt("timeflip> ")
		if errors.Is(err, io.EOF) {
			a.cleanup(context.Background())
			return nil
		}
		if err != nil {
			a.cleanup(context.Background())
			return err
		}
		if keepRunning := a.Execute(ctx, line); !keepRunning {
			a.cleanup(context.Background())
			return nil
		}
	}
}

func (a *DemoApp) Execute(ctx context.Context, line string) bool {
	args, err := splitArgs(line)
	if err != nil {
		a.out.PrintError(err)
		return true
	}
	if len(args) == 0 {
		return true
	}
	name := strings.ToLower(args[0])
	if name == "quit" {
		name = "exit"
	}
	cmd, ok := a.commands[name]
	if !ok {
		a.out.PrintLine("unknown command: " + args[0] + " (type help)")
		return true
	}
	if err := cmd.run(ctx, a, args[1:]); err != nil {
		a.out.PrintError(err)
	}
	return name != "exit"
}

func (a *DemoApp) commandOptions() timeflip.CommandOptions {
	return timeflip.CommandOptions{Timeout: a.cfg.CommandTimeout}
}

func (a *DemoApp) eventOptions() timeflip.EventOptions {
	return timeflip.EventOptions{Buffer: a.cfg.EventBuffer, IncludeRaw: a.cfg.IncludeRawEvents}
}

func (a *DemoApp) selectedDevice(args []string) (timeflip.DeviceID, error) {
	if len(args) > 0 && strings.TrimSpace(args[0]) != "" {
		return timeflip.DeviceID(args[0]), nil
	}
	if a.state.SelectedDeviceID != "" {
		return a.state.SelectedDeviceID, nil
	}
	id, err := a.in.Prompt("Device ID: ")
	if err != nil {
		return "", err
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return "", fmt.Errorf("device ID is required")
	}
	return timeflip.DeviceID(id), nil
}

func (a *DemoApp) requireSession() (*timeflip.Session, error) {
	if a.state.ActiveSession == nil {
		return nil, fmt.Errorf("no active session; run connect first")
	}
	return a.state.ActiveSession, nil
}

func (a *DemoApp) stopStream() {
	a.mu.Lock()
	cancel := a.state.ActiveStreamCancel
	a.state.ActiveStreamCancel = nil
	a.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (a *DemoApp) streamActive() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.state.ActiveStreamCancel != nil
}

func (a *DemoApp) setStreamCancel(cancel context.CancelFunc) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.state.ActiveStreamCancel = cancel
}

func (a *DemoApp) clearStream() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.state.ActiveStreamCancel = nil
}

func (a *DemoApp) cleanup(ctx context.Context) {
	a.stopStream()
	if a.state.ActiveSession != nil {
		_ = a.state.ActiveSession.Close(ctx)
		a.state.ClearSession()
	}
}

func splitArgs(line string) ([]string, error) {
	var args []string
	var b strings.Builder
	var quote rune
	escaped := false
	for _, r := range line {
		if escaped {
			b.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
				continue
			}
			b.WriteRune(r)
			continue
		}
		if r == '\'' || r == '"' {
			quote = r
			continue
		}
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			if b.Len() > 0 {
				args = append(args, b.String())
				b.Reset()
			}
			continue
		}
		b.WriteRune(r)
	}
	if escaped {
		b.WriteRune('\\')
	}
	if quote != 0 {
		return nil, fmt.Errorf("unterminated quote")
	}
	if b.Len() > 0 {
		args = append(args, b.String())
	}
	return args, nil
}
