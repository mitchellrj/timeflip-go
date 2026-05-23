package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	timeflip "timeflip-go"
)

func TestParseFlags(t *testing.T) {
	cfg, err := parseFlags([]string{"-timeout", "3s", "-command-timeout", "2s", "-event-buffer", "4", "-include-raw", "-include-unsupported"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.CommunicationTimeout != 3*time.Second || cfg.CommandTimeout != 2*time.Second || cfg.EventBuffer != 4 || !cfg.IncludeRawEvents || !cfg.IncludeUnsupportedDevices {
		t.Fatalf("unexpected config: %+v", cfg)
	}
	if _, err := parseFlags([]string{"-event-buffer", "-1"}); err == nil {
		t.Fatal("expected invalid event buffer")
	}
}

func TestSplitArgs(t *testing.T) {
	got, err := splitArgs(`write name "Desk Timer"`)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"write", "name", "Desk Timer"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("splitArgs()=%q want %q", got, want)
	}
	if _, err := splitArgs(`write name "Desk Timer`); err == nil {
		t.Fatal("expected unterminated quote error")
	}
}

func TestCommandDispatchAndPreconditions(t *testing.T) {
	app, _, out, errOut := newTestApp(t, &fakeDemoTransport{})
	if !app.Execute(context.Background(), "wat") {
		t.Fatal("unknown command should keep running")
	}
	if !strings.Contains(out.String(), "unknown command") {
		t.Fatalf("missing unknown command output: %q", out.String())
	}
	app.Execute(context.Background(), "read info")
	if !strings.Contains(errOut.String(), "no active session") {
		t.Fatalf("missing precondition error: %q", errOut.String())
	}
	if app.Execute(context.Background(), "exit") {
		t.Fatal("exit should stop loop")
	}
}

func TestListCanSelectSingleSupportedDevice(t *testing.T) {
	transport := &fakeDemoTransport{peripherals: []timeflip.Peripheral{
		{ID: "tf", Name: "TimeFlip2", RSSI: -42, AdvertisedServices: []timeflip.ServiceID{timeflip.TimeFlipService}},
	}}
	app, input, out, _ := newTestApp(t, transport)
	input.answers = []string{"y"}
	app.Execute(context.Background(), "list")
	if app.state.SelectedDeviceID != "tf" {
		t.Fatalf("selected device=%q", app.state.SelectedDeviceID)
	}
	if !strings.Contains(out.String(), "TimeFlip2") {
		t.Fatalf("missing list output: %q", out.String())
	}
}

func TestPairAndUnpairManualActionOutput(t *testing.T) {
	conn := &fakeDemoConnection{}
	transport := &fakeDemoTransport{
		connections: map[timeflip.DeviceID]*fakeDemoConnection{"tf": conn},
		pairResult: timeflip.OSActionResult{
			Unsupported: true,
			ManualAction: &timeflip.ManualAction{
				Kind:        timeflip.ManualActionOSPair,
				Description: "pair manually",
				Inputs:      map[string]string{"device_id": "tf"},
			},
		},
		pairErr: timeflip.ErrUnsupportedOperation,
		unpairResult: timeflip.OSActionResult{
			Unsupported:  true,
			ManualAction: &timeflip.ManualAction{Kind: timeflip.ManualActionOSUnpair, Inputs: map[string]string{"device_id": "tf"}},
		},
		unpairErr: timeflip.ErrUnsupportedOperation,
	}
	app, input, out, errOut := newTestApp(t, transport)
	input.answers = []string{"000000", "n", "y"}
	app.Execute(context.Background(), "pair tf")
	if !strings.Contains(out.String(), "pairing_completed: true") || !strings.Contains(out.String(), "manual action") {
		t.Fatalf("missing pairing output: out=%q err=%q", out.String(), errOut.String())
	}
	input.answers = []string{"n", "n", "y"}
	app.Execute(context.Background(), "unpair tf")
	if !strings.Contains(out.String(), "unpairing_completed: true") || !strings.Contains(out.String(), "os_unpair") {
		t.Fatalf("missing unpair output: out=%q err=%q", out.String(), errOut.String())
	}
}

func TestSessionLifecycleAndStreamCancellation(t *testing.T) {
	conn := &fakeDemoConnection{}
	app, input, out, errOut := newTestApp(t, &fakeDemoTransport{connections: map[timeflip.DeviceID]*fakeDemoConnection{"tf": conn}})
	app.Execute(context.Background(), "select tf")
	app.Execute(context.Background(), "connect")
	if app.state.ActiveSession == nil {
		t.Fatal("expected active session")
	}
	input.answers = []string{"000000"}
	app.Execute(context.Background(), "authorize")
	if !app.state.Authorized {
		t.Fatal("expected authorized state")
	}
	app.Execute(context.Background(), "stream")
	if !app.streamActive() {
		t.Fatal("expected active stream")
	}
	app.Execute(context.Background(), "stop")
	if app.streamActive() {
		t.Fatal("expected stopped stream")
	}
	app.Execute(context.Background(), "close")
	if app.state.ActiveSession != nil || app.state.Authorized {
		t.Fatal("expected closed session")
	}
	if errOut.Len() != 0 {
		t.Fatalf("unexpected errors: %q", errOut.String())
	}
	if !strings.Contains(out.String(), "stream: stopped") {
		t.Fatalf("missing stream output: %q", out.String())
	}
}

func TestReadAndWriteValidation(t *testing.T) {
	app, _, _, errOut := newTestApp(t, &fakeDemoTransport{connections: map[timeflip.DeviceID]*fakeDemoConnection{"tf": {}}})
	app.Execute(context.Background(), "select tf")
	app.Execute(context.Background(), "connect")
	app.Execute(context.Background(), "read task nope")
	if !strings.Contains(errOut.String(), "facet must be") {
		t.Fatalf("missing facet validation error: %q", errOut.String())
	}
	app.Execute(context.Background(), "write led 0 2")
	if !strings.Contains(errOut.String(), "invalid input") {
		t.Fatalf("missing library validation error: %q", errOut.String())
	}
}

func newTestApp(t *testing.T, transport timeflip.Transport) (*DemoApp, *scriptedPrompter, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	client, err := timeflip.NewClient(transport, timeflip.Config{})
	if err != nil {
		t.Fatal(err)
	}
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	input := &scriptedPrompter{}
	app := NewDemoApp(client, DemoConfig{CommunicationTimeout: 10 * time.Second, EventBuffer: 4}, input, NewTextFormatter(out, errOut))
	return app, input, out, errOut
}

type scriptedPrompter struct {
	answers []string
}

func (p *scriptedPrompter) Prompt(string) (string, error) {
	if len(p.answers) == 0 {
		return "", io.EOF
	}
	answer := p.answers[0]
	p.answers = p.answers[1:]
	return answer, nil
}

func (p *scriptedPrompter) PromptSecret(label string) (string, error) {
	return p.Prompt(label)
}

func (p *scriptedPrompter) Confirm(label string) (bool, error) {
	answer, err := p.Prompt(label)
	if err != nil {
		return false, err
	}
	switch strings.ToLower(answer) {
	case "y", "yes", "true":
		return true, nil
	default:
		return false, nil
	}
}

type fakeDemoTransport struct {
	peripherals  []timeflip.Peripheral
	connections  map[timeflip.DeviceID]*fakeDemoConnection
	pairResult   timeflip.OSActionResult
	unpairResult timeflip.OSActionResult
	scanErr      error
	connectErr   error
	pairErr      error
	unpairErr    error
}

func (f *fakeDemoTransport) Scan(context.Context, timeflip.ScanFilter) ([]timeflip.Peripheral, error) {
	if f.scanErr != nil {
		return nil, f.scanErr
	}
	return append([]timeflip.Peripheral(nil), f.peripherals...), nil
}

func (f *fakeDemoTransport) Connect(context.Context, timeflip.DeviceID) (timeflip.Connection, error) {
	if f.connectErr != nil {
		return nil, f.connectErr
	}
	if len(f.connections) == 0 {
		return nil, errors.New("connection not found")
	}
	for _, conn := range f.connections {
		return conn, nil
	}
	return nil, errors.New("connection not found")
}

func (f *fakeDemoTransport) PairOS(context.Context, timeflip.DeviceID) (timeflip.OSActionResult, error) {
	return f.pairResult, f.pairErr
}

func (f *fakeDemoTransport) UnpairOS(context.Context, timeflip.DeviceID) (timeflip.OSActionResult, error) {
	return f.unpairResult, f.unpairErr
}

type fakeDemoConnection struct {
	mu            sync.Mutex
	subscriptions []chan timeflip.Notification
	closed        bool
}

func (f *fakeDemoConnection) Read(context.Context, timeflip.CharacteristicID) ([]byte, error) {
	return []byte{0, 0, 0, 0}, nil
}

func (f *fakeDemoConnection) Write(context.Context, timeflip.CharacteristicID, []byte) error {
	return nil
}

func (f *fakeDemoConnection) Subscribe(context.Context, timeflip.CharacteristicID) (<-chan timeflip.Notification, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	ch := make(chan timeflip.Notification, 1)
	f.subscriptions = append(f.subscriptions, ch)
	return ch, nil
}

func (f *fakeDemoConnection) Close(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
	for _, ch := range f.subscriptions {
		close(ch)
	}
	f.subscriptions = nil
	return nil
}
