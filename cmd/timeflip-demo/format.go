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
		fmt.Fprintf(f.out, "status_code: 0x%04X\nhardware_code: 0x%04X\nsync_required: %v\nreset: %v\nhardware_issue: %v\n",
			v.StatusCode, v.HardwareCode, v.SyncRequired, v.Reset, v.HardwareIssue)
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
		fmt.Fprintf(f.out, "facet: %d\nmode: %d\npomodoro_seconds: %d\nelapsed_seconds: %d\n",
			v.Facet, v.Mode, v.PomodoroLimitSeconds, v.ElapsedSeconds)
	case timeflip.TapSettings:
		fmt.Fprintf(f.out, "threshold: %d\nlimit: %d\nlatency: %d\nwindow: %d\n", v.Threshold, v.Limit, v.Latency, v.Window)
	default:
		fmt.Fprintf(f.out, "%+v\n", value)
	}
}

func (f *TextFormatter) PrintCommandResult(result timeflip.CommandResult) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fmt.Fprintf(f.out, "command: 0x%02X\nok: %v\nstatus_code: 0x%02X\npayload_bytes: %d\n",
		byte(result.Command.Code), result.Status.OK, byte(result.Status.Code), len(result.Payload))
}

func (f *TextFormatter) PrintEvent(event timeflip.Event) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fmt.Fprintf(f.out, "event kind=%s device=%s received=%s\n", event.Kind, event.DeviceID, event.ReceivedAt.Format("2006-01-02T15:04:05Z07:00"))
	switch v := event.Payload.(type) {
	case timeflip.FacetEvent:
		fmt.Fprintf(f.out, "  facet=%d undefined=%v wrong_password=%v\n", v.Facet, v.Undefined, v.WrongPassword)
	case timeflip.DoubleTapEvent:
		fmt.Fprintf(f.out, "  facet=%d pause=%v\n", v.Facet, v.Pause)
	case timeflip.BatteryStatus:
		fmt.Fprintf(f.out, "  battery=%d%%\n", v.Percentage)
	case timeflip.SystemState:
		fmt.Fprintf(f.out, "  status=0x%04X hardware=0x%04X sync_required=%v reset=%v hardware_issue=%v\n",
			v.StatusCode, v.HardwareCode, v.SyncRequired, v.Reset, v.HardwareIssue)
	case []timeflip.HistoryEntry:
		fmt.Fprintf(f.out, "  history_entries=%d\n", len(v))
	default:
		if event.Payload != nil {
			fmt.Fprintf(f.out, "  payload=%+v\n", event.Payload)
		}
	}
	if len(event.Raw) > 0 {
		fmt.Fprintf(f.out, "  raw=%s\n", strings.ToUpper(hex.EncodeToString(event.Raw)))
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
