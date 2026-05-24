package main

import (
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"

	timeflip "github.com/mitchellrj/timeflip-go"
)

type TextFormatter struct {
	out   io.Writer
	err   io.Writer
	color bool
	mu    sync.Mutex
}

func NewTextFormatter(out io.Writer, err io.Writer) *TextFormatter {
	return &TextFormatter{out: out, err: err}
}

func NewTextFormatterWithColor(out io.Writer, err io.Writer, color bool) *TextFormatter {
	return &TextFormatter{out: out, err: err, color: color}
}

func (f *TextFormatter) PrintLine(line string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fmt.Fprintln(f.out, line)
}

func (f *TextFormatter) Printf(format string, args ...any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fmt.Fprintf(f.out, format, args...)
}

func (f *TextFormatter) PrintDevices(devices []timeflip.DiscoveredDevice) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(devices) == 0 {
		fmt.Fprintln(f.out, "no devices found")
		return
	}
	fmt.Fprintln(f.out, f.style("ID\tNAME\tRSSI\tSUPPORTED\tMETADATA", ansiBold))
	for _, d := range devices {
		supported := fmt.Sprintf("%v", d.Supported)
		if d.Supported {
			supported = f.style(supported, ansiGreen)
		}
		fmt.Fprintf(f.out, "%s\t%s\t%d\t%s\t%s\n", d.ID, d.Name, d.RSSI, supported, formatMap(d.Metadata))
	}
}

func (f *TextFormatter) PrintStageResults(stages []timeflip.StageResult) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(stages) == 0 {
		fmt.Fprintln(f.out, "stages: none")
		return
	}
	for _, s := range stages {
		status := "failed"
		style := ansiRed
		if s.Completed {
			status = "completed"
			style = ansiGreen
		}
		fmt.Fprintf(f.out, "stage %s: %s\n", s.Stage, f.style(status, style))
		if s.Err != nil {
			if timeflip.IsUnsupported(s.Err) && s.ManualAction != nil {
				fmt.Fprintf(f.out, "  %s automatic %s is not available; follow the manual action below.\n", f.style("note:", ansiYellow), s.Stage)
			} else {
				fmt.Fprintf(f.out, "  %s %v\n", f.style("error:", ansiRed), s.Err)
			}
		}
		if s.ManualAction != nil {
			printManualAction(f.out, s.ManualAction)
		}
	}
}

func (f *TextFormatter) PrintManualAction(action *timeflip.ManualAction) {
	f.mu.Lock()
	defer f.mu.Unlock()
	printManualAction(f.out, action)
}

func (f *TextFormatter) PrintReadResult(value any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch v := value.(type) {
	case timeflip.DeviceInfo:
		fmt.Fprintf(f.out, "name: %s\nmanufacturer: %s\nmodel: %s\nhardware: %s\nfirmware: %s\nsystem_id: %s\n",
			v.Name, v.ManufacturerName, v.ModelNumber, v.HardwareRevision, v.FirmwareRevision, v.SystemID)
	case timeflip.BatteryStatus:
		fmt.Fprintf(f.out, "%s %d%%\n", f.style("battery:", ansiCyan), v.Percentage)
	case timeflip.SystemState:
		printSystemState(f.out, v, "")
	case timeflip.TrackerStatus:
		currentFacet := "unknown"
		if v.CurrentFacetKnown {
			currentFacet = fmt.Sprintf("%d", v.CurrentFacet)
		}
		fmt.Fprintf(f.out, "lock: %v\npause: %v\nautopause_minutes: %d\ncurrent_facet: %s\n", v.LockEnabled, v.PauseEnabled, v.AutoPauseMinutes, currentFacet)
		if v.CurrentFacetKnown {
			fmt.Fprintf(f.out, "current_facet_undefined: %v\n", v.CurrentFacetUndefined)
		}
	case []timeflip.HistoryEntry:
		if len(v) == 0 {
			fmt.Fprintln(f.out, "history: no entries")
			return
		}
		for _, e := range v {
			fmt.Fprintf(f.out, "event=%d facet=%d pause=%v undefined=%v accel_error=%v moment=%d duration=%ds previous=%d\n",
				e.EventNumber, e.Facet, e.Pause, e.UndefinedFacet, e.AccelerometerError, e.MomentUnix, e.DurationSeconds, e.PreviousEventNumber)
		}
	case timeflip.TaskParameters:
		fmt.Fprintf(f.out, "facet: %d\nassigned: %v\n", v.Facet, v.Assigned)
		if !v.Assigned {
			fmt.Fprintln(f.out, "state: unassigned")
			printRawResponse(f.out, v.Raw)
			return
		}
		fmt.Fprintf(f.out, "mode: %d (%s)\npomodoro_seconds: %d\nelapsed_seconds: %d\n",
			v.Mode, taskModeLabel(v.Mode), v.PomodoroLimitSeconds, v.ElapsedSeconds)
	case timeflip.TapSettings:
		fmt.Fprintf(f.out, "configured: %v\n", v.Configured)
		if !v.Configured {
			fmt.Fprintln(f.out, "state: unassigned")
			printRawResponse(f.out, v.Raw)
			return
		}
		fmt.Fprintf(f.out, "threshold: %d\nlimit: %d\nlatency: %d\nwindow: %d\n", v.Threshold, v.Limit, v.Latency, v.Window)
	default:
		fmt.Fprintf(f.out, "%+v\n", value)
	}
}

func (f *TextFormatter) PrintCommandResult(result timeflip.CommandResult) {
	f.mu.Lock()
	defer f.mu.Unlock()
	payload := result.Payload
	if len(payload) == 0 {
		payload = result.Status.Raw
	}
	fmt.Fprintf(f.out, "command: 0x%02X\n", byte(result.Command.Code))
	if !commandStatusRecognized(payload) {
		fmt.Fprintln(f.out, "acknowledgement: unexpected")
		if len(payload) >= 2 {
			fmt.Fprintf(f.out, "status_code: 0x%02X\n", payload[1])
		}
		fmt.Fprintln(f.out, "expected_status: 0x02 OK or 0x01 rejected")
		fmt.Fprintf(f.out, "payload_bytes: %d\n", len(payload))
		if len(payload) > 0 {
			fmt.Fprintf(f.out, "raw_payload_hex: %s\n", strings.ToUpper(hex.EncodeToString(payload)))
		}
		return
	}
	acknowledgement := "rejected"
	if result.Status.OK {
		acknowledgement = "ok"
	}
	fmt.Fprintf(f.out, "acknowledgement: %s\nstatus_code: 0x%02X\npayload_bytes: %d\n",
		acknowledgement, payload[1], len(payload))
	if len(payload) > 0 {
		fmt.Fprintf(f.out, "raw_payload_hex: %s\n", strings.ToUpper(hex.EncodeToString(payload)))
	}
}

func (f *TextFormatter) PrintEvent(event timeflip.Event) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fmt.Fprintf(f.out, "\n%s %s\n", f.style("event:", ansiGreen), eventTitle(event.Kind))
	fmt.Fprintf(f.out, "  device: %s\n", event.DeviceID)
	fmt.Fprintf(f.out, "  source: %s\n", eventSourceLabel(event))
	fmt.Fprintf(f.out, "  received: %s\n", event.ReceivedAt.Format("2006-01-02T15:04:05Z07:00"))
	switch v := event.Payload.(type) {
	case timeflip.FacetEvent:
		status := "valid"
		if v.Undefined {
			status = "undefined"
		}
		if v.WrongPassword {
			status = "wrong_password_or_locked"
		}
		fmt.Fprintf(f.out, "  facet: %d\n  state: %s\n", v.Facet, status)
	case timeflip.DoubleTapEvent:
		fmt.Fprintf(f.out, "  facet: %d\n  pause_encoded: %v\n", v.Facet, v.Pause)
	case timeflip.BatteryStatus:
		fmt.Fprintf(f.out, "  battery: %d%%\n", v.Percentage)
	case timeflip.SystemState:
		printSystemState(f.out, v, "  ")
	case []timeflip.HistoryEntry:
		fmt.Fprintf(f.out, "  history_entries: %d\n", len(v))
	case []byte:
		fmt.Fprintf(f.out, "  payload_bytes: %d\n  payload_hex: %s\n", len(v), strings.ToUpper(hex.EncodeToString(v)))
	default:
		if event.Payload != nil {
			fmt.Fprintf(f.out, "  payload: %+v\n", event.Payload)
		}
	}
	if len(event.Raw) > 0 {
		fmt.Fprintf(f.out, "  raw_hex: %s\n", strings.ToUpper(hex.EncodeToString(event.Raw)))
	}
}

func (f *TextFormatter) PrintError(err error) {
	if err == nil {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	var opErr *timeflip.OperationError
	if errors.As(err, &opErr) {
		if opErr.Operation == "events" && errors.Is(err, timeflip.ErrProtocol) {
			source := opErr.Stage
			if source == "" {
				source = "unknown"
			}
			fmt.Fprintf(f.err, "%s could not decode %s notification from device %s; streaming continues.\n", f.style("stream warning:", ansiYellow), source, opErr.DeviceID)
			fmt.Fprintf(f.err, "  detail: %v\n", opErr.Err)
			return
		}
		if opErr.Operation == "send_command" && errors.Is(err, timeflip.ErrProtocol) {
			fmt.Fprintf(f.err, "%s device returned an unexpected acknowledgement", f.style("command error:", ansiRed))
			if opErr.DeviceID != "" {
				fmt.Fprintf(f.err, " from %s", opErr.DeviceID)
			}
			if opErr.Command != 0 {
				fmt.Fprintf(f.err, " for command 0x%02X", byte(opErr.Command))
			}
			fmt.Fprintln(f.err, ".")
			fmt.Fprintln(f.err, "  expected: command result status 0x02 (OK) or 0x01 (rejected)")
			fmt.Fprintln(f.err, "  meaning: the command reached the device, but its acknowledgement was not one this library can treat as success")
			fmt.Fprintf(f.err, "  detail: %v\n", opErr.Err)
			if byte(opErr.Command) == 0x15 {
				fmt.Fprintln(f.err, "  name note: device names must be 1-18 ASCII characters; if the name already fits, try authorize, read system, then retry with a simple test name")
			} else {
				fmt.Fprintln(f.err, "  next: check authorization/lock state with read system, then retry the command")
			}
			return
		}
		if (opErr.Operation == "send_command" || isCommandBackedRead(opErr.Operation)) && errors.Is(err, timeflip.ErrAuthorizationFailed) {
			fmt.Fprintf(f.err, "%s device is not accepting commands", f.style("authorization error:", ansiRed))
			if opErr.DeviceID != "" {
				fmt.Fprintf(f.err, " from %s", opErr.DeviceID)
			}
			if opErr.Command != 0 {
				fmt.Fprintf(f.err, " for command 0x%02X", byte(opErr.Command))
			}
			fmt.Fprintln(f.err, ".")
			fmt.Fprintln(f.err, "  device_response: password check failed (0x01)")
			fmt.Fprintln(f.err, "  meaning: the command was sent while the device still considers this session unauthorized")
			fmt.Fprintln(f.err, "  next: run authorize with the current six-character password; if the device was factory reset, try 000000 and confirm the reset really completed")
			return
		}
		if isCommandBackedRead(opErr.Operation) && errors.Is(err, timeflip.ErrProtocol) {
			var payloadErr *timeflip.ProtocolPayloadError
			if errors.As(err, &payloadErr) && strings.Contains(payloadErr.Expected, "command result beginning") {
				fmt.Fprintf(f.err, "%s no response for command 0x%02X", f.style("read warning:", ansiYellow), byte(opErr.Command))
				if opErr.DeviceID != "" {
					fmt.Fprintf(f.err, " from %s", opErr.DeviceID)
				}
				fmt.Fprintln(f.err, ".")
				if len(payloadErr.Payload) > 0 {
					fmt.Fprintf(f.err, "  last_command_result: 0x%s\n", strings.ToUpper(hex.EncodeToString(payloadErr.Payload)))
				}
				fmt.Fprintln(f.err, "  meaning: the command-result characteristic did not change to the response for the command just sent before the timeout")
				fmt.Fprintln(f.err, "  next: try authorize, read system, then retry; if this persists, this firmware may not support this read command")
				return
			}
		}
		fmt.Fprintf(f.err, "%s operation=%s", f.style("error:", ansiRed), opErr.Operation)
		if opErr.Stage != "" {
			fmt.Fprintf(f.err, " stage=%s", opErr.Stage)
		}
		if opErr.DeviceID != "" {
			fmt.Fprintf(f.err, " device=%s", opErr.DeviceID)
		}
		if opErr.Command != 0 {
			fmt.Fprintf(f.err, " command=0x%02X", byte(opErr.Command))
		}
		if opErr.Err != nil {
			fmt.Fprintf(f.err, ": %v", opErr.Err)
		}
		fmt.Fprintln(f.err)
		if timeflip.IsUnsupported(err) {
			fmt.Fprintln(f.err, "adapter note: this operation may require a concrete MacOS BLE adapter or manual OS action.")
		}
		return
	}
	fmt.Fprintf(f.err, "%s %v\n", f.style("error:", ansiRed), err)
}

func isCommandBackedRead(operation string) bool {
	switch operation {
	case "read_task_parameters", "read_tap_settings":
		return true
	default:
		return false
	}
}

func printSystemState(out io.Writer, state timeflip.SystemState, prefix string) {
	fmt.Fprintf(out, "%sstatus_code: 0x%04X\n", prefix, state.StatusCode)
	if state.StatusDescription != "" {
		fmt.Fprintf(out, "%sstatus: %s\n", prefix, state.StatusDescription)
	}
	fmt.Fprintf(out, "%shardware_code: 0x%04X\n", prefix, state.HardwareCode)
	if state.HardwareDescription != "" {
		fmt.Fprintf(out, "%shardware: %s\n", prefix, state.HardwareDescription)
	}
	fmt.Fprintf(out, "%ssync_required: %v\n", prefix, state.SyncRequired)
	if state.SyncRequired {
		fmt.Fprintf(out, "%ssync_reason: %s\n", prefix, systemSyncReasonLabel(state.SyncReason))
		actions := systemSyncActions(state.SyncReason)
		if len(actions) > 0 {
			fmt.Fprintf(out, "%ssync_actions:\n", prefix)
			for _, action := range actions {
				fmt.Fprintf(out, "%s  %s\n", prefix, action)
			}
		}
	}
	fmt.Fprintf(out, "%sreset: %v\n%shardware_issue: %v\n", prefix, state.Reset, prefix, state.HardwareIssue)
}

func systemSyncReasonLabel(reason string) string {
	switch reason {
	case "time":
		return "time synchronization"
	case "facet_color":
		return "facet color synchronization"
	case "led_brightness":
		return "LED brightness synchronization"
	case "blink_interval":
		return "blink interval synchronization"
	case "task_parameters":
		return "task parameter synchronization"
	case "auto_pause":
		return "auto-pause synchronization"
	case "unknown":
		return "unknown synchronization"
	default:
		return reason
	}
}

func systemSyncActions(reason string) []string {
	switch reason {
	case "time":
		return []string{"time sync is not exposed by this demo yet", "try reconnecting or use the official app to set device time"}
	case "facet_color":
		return []string{"write color FACET R G B"}
	case "led_brightness":
		return []string{"write led BRIGHTNESS_PERCENT BLINK_SECONDS"}
	case "blink_interval":
		return []string{"write led BRIGHTNESS_PERCENT BLINK_SECONDS"}
	case "task_parameters":
		return []string{"write task FACET MODE POMODORO_SECONDS", "MODE: 0 normal, 1 pomodoro"}
	case "auto_pause":
		return []string{"write autopause MINUTES"}
	case "unknown":
		return []string{"read system again after any pending configuration write", "check whether this firmware reports an undocumented sync code"}
	default:
		return nil
	}
}

func taskModeLabel(mode uint8) string {
	switch mode {
	case 0:
		return "normal"
	case 1:
		return "pomodoro"
	default:
		return "unknown"
	}
}

func printRawResponse(out io.Writer, payload []byte) {
	if len(payload) == 0 {
		return
	}
	fmt.Fprintf(out, "raw_response: 0x%s\n", strings.ToUpper(hex.EncodeToString(payload)))
}

func commandStatusRecognized(payload []byte) bool {
	if len(payload) < 2 {
		return false
	}
	return payload[1] == 0x02 || payload[1] == 0x01
}

func eventTitle(kind timeflip.EventKind) string {
	switch kind {
	case timeflip.EventFacet:
		return "orientation / facet changed"
	case timeflip.EventDoubleTap:
		return "double tap"
	case timeflip.EventBattery:
		return "battery update"
	case timeflip.EventSystemState:
		return "system state update"
	case timeflip.EventHistory:
		return "history update"
	case timeflip.EventRaw:
		return "raw TimeFlip notification"
	default:
		return string(kind)
	}
}

func eventSourceLabel(event timeflip.Event) string {
	if event.Source == "" {
		return "unknown"
	}
	return timeflip.NotificationSourceName(event.Source)
}

func (f *TextFormatter) PrintSuggestions(commands []string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(commands) == 0 {
		return
	}
	fmt.Fprintln(f.out, f.style("next:", ansiCyan))
	for _, command := range commands {
		if strings.TrimSpace(command) == "" {
			continue
		}
		fmt.Fprintf(f.out, "  %s\n", f.style(command, ansiBold))
	}
}

const (
	ansiReset  = "\033[0m"
	ansiBold   = "\033[1m"
	ansiRed    = "\033[31m"
	ansiGreen  = "\033[32m"
	ansiYellow = "\033[33m"
	ansiCyan   = "\033[36m"
)

func (f *TextFormatter) style(value string, code string) string {
	if !f.color || value == "" {
		return value
	}
	return code + value + ansiReset
}

func printManualAction(w io.Writer, action *timeflip.ManualAction) {
	if action == nil {
		return
	}
	fmt.Fprintf(w, "manual action: %s\n", action.Kind)
	printManualActionSteps(w, action)
	if len(action.Inputs) > 0 {
		keys := make([]string, 0, len(action.Inputs))
		for k := range action.Inputs {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(w, "  %s=%s\n", k, action.Inputs[k])
		}
	}
}

func printManualActionSteps(w io.Writer, action *timeflip.ManualAction) {
	steps := manualActionSteps(action)
	if len(steps) == 0 && action.Description != "" {
		for _, line := range strings.Split(action.Description, "\n") {
			if strings.TrimSpace(line) != "" {
				fmt.Fprintf(w, "  %s\n", strings.TrimSpace(line))
			}
		}
		return
	}
	for i, step := range steps {
		fmt.Fprintf(w, "  %d. %s\n", i+1, step)
	}
}

func manualActionSteps(action *timeflip.ManualAction) []string {
	if action == nil {
		return nil
	}
	deviceID := action.Inputs["device_id"]
	switch action.Kind {
	case timeflip.ManualActionOSPair:
		return []string{
			"Keep the TimeFlip2 powered on and close to this Mac.",
			"If macOS shows a Bluetooth pairing prompt while you connect, authorize, read, or write, approve it.",
			"If no prompt appears, open System Settings > Bluetooth.",
			"Find the TimeFlip2 device" + deviceSuffix(deviceID) + " and click Connect or Pair.",
			"Return to this demo and run: connect " + commandDeviceID(deviceID),
			"Then run: authorize, followed by read info, read battery, or read system.",
		}
	case timeflip.ManualActionOSUnpair:
		return []string{
			"Open System Settings > Bluetooth.",
			"Find the TimeFlip2 device" + deviceSuffix(deviceID) + ".",
			"Open the device details and choose Forget This Device or Remove.",
			"Return to this demo and run: list, then pair " + commandDeviceID(deviceID) + " if you want to pair again.",
		}
	default:
		return nil
	}
}

func deviceSuffix(deviceID string) string {
	if deviceID == "" {
		return ""
	}
	return " with device ID " + deviceID
}

func commandDeviceID(deviceID string) string {
	if deviceID == "" {
		return "DEVICE_ID"
	}
	return deviceID
}

func formatMap(values map[string]string) string {
	if len(values) == 0 {
		return "-"
	}
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+values[k])
	}
	return strings.Join(parts, ",")
}
