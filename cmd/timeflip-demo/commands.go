package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	timeflip "github.com/mitchellrj/timeflip-go"
)

type demoCommand struct {
	name        string
	usage       string
	description string
	run         func(context.Context, *DemoApp, []string) error
}

func buildCommands() map[string]demoCommand {
	commands := []demoCommand{
		{"help", "help [command]", "Show command list or command usage.", runHelp},
		{"status", "status", "Show selected device, session, stream, and timeout state.", runStatus},
		{"list", "list", "Scan for TimeFlip2 devices.", runList},
		{"select", "select DEVICE_ID", "Set the current target device.", runSelect},
		{"pair", "pair [DEVICE_ID]", "Run guided pairing for a new or reset device.", runPair},
		{"unpair", "unpair [DEVICE_ID]", "Run guided unpairing and optional reset.", runUnpair},
		{"connect", "connect [DEVICE_ID]", "Open an active session.", runConnect},
		{"authorize", "authorize", "Authorize the active session with a six-character password.", runAuthorize},
		{"read", "read info|battery|system|history|task|tap [args]", "Read data or configuration from the active session.", runRead},
		{"write", "write password|name|lock|pause|autopause|led|color|task|tap [args]", "Write supported configuration.", runWrite},
		{"command", "command reset-task-info|factory-reset", "Execute supported device commands.", runCommand},
		{"stream", "stream", "Print technical device events until stop, close, or exit.", runStream},
		{"stop", "stop", "Stop the active event stream.", runStop},
		{"close", "close", "Close the active session.", runClose},
		{"exit", "exit", "Close resources and quit.", runExit},
	}
	out := make(map[string]demoCommand, len(commands))
	for _, cmd := range commands {
		out[cmd.name] = cmd
	}
	return out
}

func runHelp(_ context.Context, app *DemoApp, args []string) error {
	if len(args) > 0 {
		name := strings.ToLower(args[0])
		if name == "quit" {
			name = "exit"
		}
		cmd, ok := app.commands[name]
		if !ok {
			return fmt.Errorf("unknown command: %s", args[0])
		}
		app.out.PrintLine(cmd.usage)
		app.out.PrintLine("  " + cmd.description)
		return nil
	}
	names := []string{"help", "status", "list", "select", "pair", "connect", "authorize", "read", "write", "command", "stream", "stop", "unpair", "close", "exit"}
	app.out.PrintLine("Commands:")
	for _, name := range names {
		cmd := app.commands[name]
		app.out.Printf("  %-12s %s\n", cmd.name, cmd.description)
	}
	app.out.PrintLine("Password prompts use standard input; terminal echo is not disabled in this dependency-free demo.")
	app.out.PrintLine("When running in a supported terminal, use up/down arrows for command history. Use -no-color to disable color output.")
	return nil
}

func runStatus(_ context.Context, app *DemoApp, _ []string) error {
	selected := string(app.state.SelectedDeviceID)
	if selected == "" {
		selected = "(none)"
	}
	session := "closed"
	if app.state.ActiveSession != nil {
		session = "open"
	}
	stream := "stopped"
	if app.streamActive() {
		stream = "active"
	}
	app.out.Printf("selected_device: %s\nsession: %s\nauthorized: %v\nstream: %s\ntimeout: %s\ncommand_timeout: %s\nevent_buffer: %d\ninclude_raw: %v\ninclude_unsupported: %v\n",
		selected, session, app.state.Authorized, stream, app.cfg.CommunicationTimeout, app.cfg.CommandTimeout, app.cfg.EventBuffer, app.cfg.IncludeRawEvents, app.cfg.IncludeUnsupportedDevices)
	return nil
}

func runList(ctx context.Context, app *DemoApp, _ []string) error {
	devices, err := app.client.ListDevices(ctx, timeflip.ScanFilter{IncludeUnsupported: app.cfg.IncludeUnsupportedDevices})
	if err != nil {
		return err
	}
	app.out.PrintDevices(devices)
	supported := make([]timeflip.DiscoveredDevice, 0, 1)
	for _, d := range devices {
		if d.Supported {
			supported = append(supported, d)
		}
	}
	if app.state.SelectedDeviceID == "" && len(supported) == 1 {
		ok, err := app.in.Confirm("Select " + string(supported[0].ID) + "?")
		if err != nil {
			return err
		}
		if ok {
			app.state.SetSelectedDevice(supported[0].ID)
			app.out.PrintLine("selected_device: " + string(supported[0].ID))
			app.out.PrintSuggestions(deviceSelectedSuggestions(supported[0].ID))
			return nil
		}
	}
	if app.state.SelectedDeviceID == "" && len(supported) > 0 {
		app.out.PrintSuggestions([]string{"select DEVICE_ID", "pair DEVICE_ID"})
		return nil
	}
	if app.state.SelectedDeviceID != "" {
		app.out.PrintSuggestions(deviceSelectedSuggestions(app.state.SelectedDeviceID))
	}
	return nil
}

func runSelect(_ context.Context, app *DemoApp, args []string) error {
	if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
		return fmt.Errorf("usage: select DEVICE_ID")
	}
	app.state.SetSelectedDevice(timeflip.DeviceID(args[0]))
	app.out.PrintLine("selected_device: " + args[0])
	app.out.PrintSuggestions(deviceSelectedSuggestions(timeflip.DeviceID(args[0])))
	return nil
}

func runPair(ctx context.Context, app *DemoApp, args []string) error {
	id, err := app.selectedDevice(args)
	if err != nil {
		return err
	}
	password, err := app.in.PromptSecret("Current password if previously set (leave blank if none): ")
	if err != nil {
		return err
	}
	if password != "" && len(password) != 6 {
		return fmt.Errorf("password must be six characters")
	}
	var newPassword string
	if ok, err := app.in.Confirm("Set a new password?"); err != nil {
		return err
	} else if ok {
		newPassword, err = app.in.PromptSecret("New password (six characters): ")
		if err != nil {
			return err
		}
		if len(newPassword) != 6 {
			return fmt.Errorf("new password must be six characters")
		}
		confirm, err := app.in.PromptSecret("Confirm new password: ")
		if err != nil {
			return err
		}
		if confirm != newPassword {
			return fmt.Errorf("new passwords do not match")
		}
	}
	allowOS, err := app.in.Confirm("Allow OS pairing if supported?")
	if err != nil {
		return err
	}
	result, err := app.client.Pair(ctx, timeflip.PairRequest{
		DeviceID:       id,
		Password:       password,
		NewPassword:    newPassword,
		AllowOSPairing: allowOS,
		Timeout:        app.cfg.CommandTimeout,
	})
	app.state.SetSelectedDevice(id)
	app.out.Printf("pairing_completed: %v\npairing_stage: %s\n", result.Completed, result.Stage)
	app.out.PrintStageResults(result.Stages)
	app.out.PrintManualAction(result.ManualAction)
	if result.ManualAction != nil {
		app.out.PrintSuggestions([]string{"connect " + string(id), "authorize", "read info"})
	} else if result.Completed {
		app.out.PrintSuggestions(afterPairSuggestions(id))
	}
	return err
}

func runUnpair(ctx context.Context, app *DemoApp, args []string) error {
	id, err := app.selectedDevice(args)
	if err != nil {
		return err
	}
	var password string
	if ok, err := app.in.Confirm("Provide password for device-side reset?"); err != nil {
		return err
	} else if ok {
		password, err = app.in.PromptSecret("Password (six characters): ")
		if err != nil {
			return err
		}
		if len(password) != 6 {
			return fmt.Errorf("password must be six characters")
		}
	}
	factoryReset := false
	if ok, err := app.in.Confirm("Factory reset device?"); err != nil {
		return err
	} else if ok {
		typed, err := app.in.Prompt("Type device ID to confirm factory reset: ")
		if err != nil {
			return err
		}
		if typed != string(id) {
			return fmt.Errorf("factory reset confirmation did not match device ID")
		}
		factoryReset = true
	}
	allowOS, err := app.in.Confirm("Allow OS unpairing if supported?")
	if err != nil {
		return err
	}
	result, err := app.client.Unpair(ctx, timeflip.UnpairRequest{
		DeviceID:         id,
		Password:         password,
		FactoryReset:     factoryReset,
		AllowOSUnpairing: allowOS,
		Timeout:          app.cfg.CommandTimeout,
	})
	app.out.Printf("unpairing_completed: %v\ndevice_reset_complete: %v\nos_unpair_complete: %v\nunpairing_stage: %s\n",
		result.Completed, result.DeviceResetComplete, result.OSUnpairComplete, result.Stage)
	app.out.PrintStageResults(result.Stages)
	app.out.PrintManualAction(result.ManualAction)
	if app.state.ActiveSession != nil && app.state.SelectedDeviceID == id {
		app.cleanup(context.Background())
	}
	return err
}

func runConnect(ctx context.Context, app *DemoApp, args []string) error {
	id, err := app.selectedDevice(args)
	if err != nil {
		return err
	}
	if app.state.ActiveSession != nil {
		ok, err := app.in.Confirm("Close current session first?")
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("connect canceled")
		}
		if err := runClose(ctx, app, nil); err != nil {
			return err
		}
	}
	session, err := app.client.Connect(ctx, timeflip.ConnectRequest{DeviceID: id, Timeout: app.cfg.CommandTimeout})
	if err != nil {
		return err
	}
	app.state.SetSelectedDevice(id)
	app.state.SetSession(session)
	app.out.PrintLine("session: open")
	app.out.PrintSuggestions([]string{"authorize", "read info"})
	return nil
}

func runAuthorize(ctx context.Context, app *DemoApp, _ []string) error {
	session, err := app.requireSession()
	if err != nil {
		return err
	}
	password, err := app.in.PromptSecret("Password (six characters): ")
	if err != nil {
		return err
	}
	if len(password) != 6 {
		return fmt.Errorf("password must be six characters")
	}
	result, err := session.Authorize(ctx, password)
	if err != nil {
		return err
	}
	app.state.Authorized = result.Authorized
	app.out.Printf("authorized: %v\n", result.Authorized)
	if result.Authorized {
		app.out.PrintSuggestions(afterAuthorizeSuggestions())
	}
	return nil
}

func runClose(ctx context.Context, app *DemoApp, _ []string) error {
	app.stopStream()
	if app.state.ActiveSession == nil {
		app.out.PrintLine("session: already closed")
		app.state.ClearSession()
		return nil
	}
	err := app.state.ActiveSession.Close(ctx)
	app.state.ClearSession()
	app.out.PrintLine("session: closed")
	if app.state.SelectedDeviceID != "" {
		app.out.PrintSuggestions([]string{"connect " + string(app.state.SelectedDeviceID), "unpair " + string(app.state.SelectedDeviceID)})
	}
	return err
}

func runRead(ctx context.Context, app *DemoApp, args []string) error {
	session, err := app.requireSession()
	if err != nil {
		return err
	}
	if len(args) == 0 {
		return fmt.Errorf("usage: read info|battery|system|history|task|tap [args]")
	}
	switch strings.ToLower(args[0]) {
	case "info":
		value, err := session.ReadDeviceInfo(ctx)
		if err == nil {
			app.out.PrintReadResult(value)
			app.out.PrintSuggestions([]string{"read battery", "read system", "stream"})
		}
		return err
	case "battery":
		value, err := session.ReadBattery(ctx)
		if err == nil {
			app.out.PrintReadResult(value)
			app.out.PrintSuggestions([]string{"read system", "stream"})
		}
		return err
	case "system":
		value, err := session.ReadSystemState(ctx)
		if err == nil {
			app.out.PrintReadResult(value)
			app.out.PrintSuggestions([]string{"read tap", "stream", "write name NAME"})
		}
		return err
	case "history":
		req, err := parseHistoryRequest(args[1:])
		if err != nil {
			return err
		}
		value, err := session.ReadHistory(ctx, req)
		if err == nil {
			app.out.PrintReadResult(value)
			app.out.PrintSuggestions([]string{"stream", "read info"})
		}
		return err
	case "task":
		if len(args) < 2 {
			return fmt.Errorf("usage: read task FACET")
		}
		facet, err := parseFacet(args[1])
		if err != nil {
			return err
		}
		value, err := session.ReadTaskParameters(ctx, facet, app.commandOptions())
		if err == nil {
			app.out.PrintReadResult(value)
			app.out.PrintSuggestions([]string{"read tap", "write task FACET MODE POMODORO_SECONDS"})
		}
		return err
	case "tap":
		value, err := session.ReadTapSettings(ctx, app.commandOptions())
		if err == nil {
			app.out.PrintReadResult(value)
			app.out.PrintSuggestions([]string{"write tap THRESHOLD LIMIT LATENCY WINDOW", "stream"})
		}
		return err
	default:
		return fmt.Errorf("unknown read kind: %s", args[0])
	}
}

func runWrite(ctx context.Context, app *DemoApp, args []string) error {
	session, err := app.requireSession()
	if err != nil {
		return err
	}
	if len(args) == 0 {
		return fmt.Errorf("usage: write password|name|lock|pause|autopause|led|color|task|tap [args]")
	}
	var result timeflip.CommandResult
	switch strings.ToLower(args[0]) {
	case "password":
		ok, err := app.in.Confirm("Change device password?")
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("password change canceled")
		}
		password, err := app.in.PromptSecret("New password (six characters): ")
		if err != nil {
			return err
		}
		if len(password) != 6 {
			return fmt.Errorf("password must be six characters")
		}
		confirm, err := app.in.PromptSecret("Confirm new password: ")
		if err != nil {
			return err
		}
		if confirm != password {
			return fmt.Errorf("passwords do not match")
		}
		result, err = session.SetPassword(ctx, password, app.commandOptions())
	case "name":
		if len(args) < 2 {
			return fmt.Errorf("usage: write name NAME")
		}
		result, err = session.SetName(ctx, args[1], app.commandOptions())
	case "lock":
		if len(args) < 2 {
			return fmt.Errorf("usage: write lock on|off")
		}
		enabled, parseErr := parseOnOff(args[1])
		if parseErr != nil {
			return parseErr
		}
		result, err = session.SetLock(ctx, enabled, app.commandOptions())
	case "pause":
		if len(args) < 2 {
			return fmt.Errorf("usage: write pause on|off")
		}
		enabled, parseErr := parseOnOff(args[1])
		if parseErr != nil {
			return parseErr
		}
		result, err = session.SetPause(ctx, enabled, app.commandOptions())
	case "autopause":
		if len(args) < 2 {
			return fmt.Errorf("usage: write autopause MINUTES")
		}
		minutes, parseErr := parseUint16(args[1], "minutes")
		if parseErr != nil {
			return parseErr
		}
		result, err = session.SetAutoPause(ctx, minutes, app.commandOptions())
	case "led":
		if len(args) < 3 {
			return fmt.Errorf("usage: write led BRIGHTNESS_PERCENT BLINK_SECONDS")
		}
		brightness, parseErr := parseUint8(args[1], "brightness")
		if parseErr != nil {
			return parseErr
		}
		blink, parseErr := parseUint8(args[2], "blink seconds")
		if parseErr != nil {
			return parseErr
		}
		result, err = session.SetLED(ctx, brightness, blink, app.commandOptions())
	case "color":
		if len(args) < 5 {
			return fmt.Errorf("usage: write color FACET R G B")
		}
		facet, parseErr := parseFacet(args[1])
		if parseErr != nil {
			return parseErr
		}
		r, parseErr := parseUint16(args[2], "red")
		if parseErr != nil {
			return parseErr
		}
		g, parseErr := parseUint16(args[3], "green")
		if parseErr != nil {
			return parseErr
		}
		b, parseErr := parseUint16(args[4], "blue")
		if parseErr != nil {
			return parseErr
		}
		result, err = session.SetFacetColor(ctx, facet, timeflip.RGB{R: r, G: g, B: b}, app.commandOptions())
	case "task":
		if len(args) < 4 {
			return fmt.Errorf("usage: write task FACET MODE POMODORO_SECONDS")
		}
		facet, parseErr := parseFacet(args[1])
		if parseErr != nil {
			return parseErr
		}
		mode, parseErr := parseUint8(args[2], "mode")
		if parseErr != nil {
			return parseErr
		}
		seconds, parseErr := parseUint32(args[3], "pomodoro seconds")
		if parseErr != nil {
			return parseErr
		}
		result, err = session.SetTaskParameters(ctx, timeflip.TaskParameters{Facet: facet, Mode: mode, PomodoroLimitSeconds: seconds}, app.commandOptions())
	case "tap":
		if len(args) < 5 {
			return fmt.Errorf("usage: write tap THRESHOLD LIMIT LATENCY WINDOW")
		}
		threshold, parseErr := parseUint8(args[1], "threshold")
		if parseErr != nil {
			return parseErr
		}
		limit, parseErr := parseUint8(args[2], "limit")
		if parseErr != nil {
			return parseErr
		}
		latency, parseErr := parseUint8(args[3], "latency")
		if parseErr != nil {
			return parseErr
		}
		window, parseErr := parseUint8(args[4], "window")
		if parseErr != nil {
			return parseErr
		}
		result, err = session.SetTapSettings(ctx, timeflip.TapSettings{Threshold: threshold, Limit: limit, Latency: latency, Window: window}, app.commandOptions())
	default:
		return fmt.Errorf("unknown write kind: %s", args[0])
	}
	if result.Command.Code != 0 || len(result.Payload) > 0 {
		app.out.PrintCommandResult(result)
		app.out.PrintSuggestions([]string{"read system", "read info", "stream"})
	}
	return err
}

func runCommand(ctx context.Context, app *DemoApp, args []string) error {
	session, err := app.requireSession()
	if err != nil {
		return err
	}
	if len(args) == 0 {
		return fmt.Errorf("usage: command reset-task-info|factory-reset")
	}
	var result timeflip.CommandResult
	switch strings.ToLower(args[0]) {
	case "reset-task-info":
		ok, err := app.in.Confirm("Reset task information?")
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("reset task info canceled")
		}
		result, err = session.ResetTaskInfo(ctx, app.commandOptions())
	case "factory-reset":
		if app.state.SelectedDeviceID == "" {
			return fmt.Errorf("selected device is required for factory reset confirmation")
		}
		typed, err := app.in.Prompt("Type device ID to confirm factory reset: ")
		if err != nil {
			return err
		}
		if typed != string(app.state.SelectedDeviceID) {
			return fmt.Errorf("factory reset confirmation did not match device ID")
		}
		result, err = session.FactoryReset(ctx, app.commandOptions())
	default:
		return fmt.Errorf("unknown command kind: %s", args[0])
	}
	if result.Command.Code != 0 || len(result.Payload) > 0 {
		app.out.PrintCommandResult(result)
		app.out.PrintSuggestions([]string{"read system", "read info"})
	}
	return err
}

func runStream(ctx context.Context, app *DemoApp, _ []string) error {
	session, err := app.requireSession()
	if err != nil {
		return err
	}
	if app.streamActive() {
		return fmt.Errorf("event stream already active")
	}
	streamCtx, cancel := context.WithCancel(ctx)
	events, errs, err := session.Events(streamCtx, app.eventOptions())
	if err != nil {
		cancel()
		return err
	}
	app.setStreamCancel(cancel)
	app.out.PrintLine("stream: active (run stop to cancel)")
	app.out.PrintSuggestions([]string{"stop", "close"})
	go func() {
		defer app.clearStream()
		for events != nil || errs != nil {
			select {
			case event, ok := <-events:
				if !ok {
					events = nil
					continue
				}
				app.out.PrintEvent(event)
			case err, ok := <-errs:
				if !ok {
					errs = nil
					continue
				}
				app.out.PrintError(err)
			case <-streamCtx.Done():
				return
			}
		}
		app.out.PrintLine("stream: ended")
	}()
	return nil
}

func runStop(_ context.Context, app *DemoApp, _ []string) error {
	if !app.streamActive() {
		app.out.PrintLine("stream: already stopped")
		return nil
	}
	app.stopStream()
	app.out.PrintLine("stream: stopped")
	app.out.PrintSuggestions([]string{"read info", "stream", "close"})
	return nil
}

func deviceSelectedSuggestions(id timeflip.DeviceID) []string {
	return []string{"pair " + string(id), "connect " + string(id)}
}

func afterPairSuggestions(id timeflip.DeviceID) []string {
	return []string{"connect " + string(id), "authorize", "read info", "read battery", "read system"}
}

func afterAuthorizeSuggestions() []string {
	return []string{"read info", "read battery", "read system", "stream"}
}

func runExit(_ context.Context, app *DemoApp, _ []string) error {
	app.out.PrintLine("bye")
	return nil
}

func parseHistoryRequest(args []string) (timeflip.HistoryRequest, error) {
	req := timeflip.HistoryRequest{}
	for _, arg := range args {
		if arg == "--all" {
			req.All = true
			continue
		}
		value, err := parseUint32(arg, "start event")
		if err != nil {
			return req, err
		}
		req.StartEvent = value
	}
	return req, nil
}

func parseOnOff(value string) (bool, error) {
	switch strings.ToLower(value) {
	case "on", "true", "1", "yes":
		return true, nil
	case "off", "false", "0", "no":
		return false, nil
	default:
		return false, fmt.Errorf("expected on or off, got %q", value)
	}
}

func parseFacet(value string) (timeflip.FacetID, error) {
	n, err := parseUint8(value, "facet")
	if err != nil {
		return 0, err
	}
	return timeflip.FacetID(n), nil
}

func parseUint8(value string, label string) (uint8, error) {
	n, err := strconv.ParseUint(value, 10, 8)
	if err != nil {
		return 0, fmt.Errorf("%s must be an unsigned 8-bit integer", label)
	}
	return uint8(n), nil
}

func parseUint16(value string, label string) (uint16, error) {
	n, err := strconv.ParseUint(value, 10, 16)
	if err != nil {
		return 0, fmt.Errorf("%s must be an unsigned 16-bit integer", label)
	}
	return uint16(n), nil
}

func parseUint32(value string, label string) (uint32, error) {
	n, err := strconv.ParseUint(value, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("%s must be an unsigned 32-bit integer", label)
	}
	return uint32(n), nil
}
