package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	timeflip "github.com/mitchellrj/timeflip-go"
)

func TestFormatterPrintsManualActionAndOperationError(t *testing.T) {
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	formatter := NewTextFormatter(out, errOut)
	formatter.PrintManualAction(&timeflip.ManualAction{
		Kind:        timeflip.ManualActionOSPair,
		Description: "pair in settings",
		Inputs:      map[string]string{"device_id": "tf"},
	})
	formatter.PrintError(&timeflip.OperationError{
		Operation: "pair",
		DeviceID:  "tf",
		Stage:     "os_pair",
		Err:       timeflip.ErrUnsupportedOperation,
	})
	if !strings.Contains(out.String(), "1. Keep the TimeFlip2 powered on") ||
		!strings.Contains(out.String(), "System Settings > Bluetooth") ||
		!strings.Contains(out.String(), "connect tf") ||
		!strings.Contains(out.String(), "device_id=tf") {
		t.Fatalf("missing manual action output: %q", out.String())
	}
	if !strings.Contains(errOut.String(), "operation=pair") || !strings.Contains(errOut.String(), "adapter note") {
		t.Fatalf("missing operation error output: %q", errOut.String())
	}
}

func TestFormatterPrintsManualActionStageAsGuidance(t *testing.T) {
	out := &bytes.Buffer{}
	formatter := NewTextFormatter(out, &bytes.Buffer{})
	formatter.PrintStageResults([]timeflip.StageResult{{
		Stage:     "os_pair",
		Completed: false,
		Err:       timeflip.ErrUnsupportedOperation,
		ManualAction: &timeflip.ManualAction{
			Kind:   timeflip.ManualActionOSPair,
			Inputs: map[string]string{"device_id": "tf"},
		},
	}})
	if strings.Contains(out.String(), "error: unsupported operation") {
		t.Fatalf("manual action stage should not print bare unsupported error: %q", out.String())
	}
	if !strings.Contains(out.String(), "automatic os_pair is not available") || !strings.Contains(out.String(), "System Settings > Bluetooth") {
		t.Fatalf("missing guided manual stage output: %q", out.String())
	}
}

func TestFormatterPrintsEventVariants(t *testing.T) {
	out := &bytes.Buffer{}
	formatter := NewTextFormatter(out, &bytes.Buffer{})
	formatter.PrintEvent(timeflip.Event{
		Kind:       timeflip.EventFacet,
		DeviceID:   "tf",
		Source:     timeflip.CharacteristicID("facet-source"),
		ReceivedAt: time.Unix(0, 0).UTC(),
		Payload:    timeflip.FacetEvent{Facet: 3},
		Raw:        []byte{3},
	})
	formatter.PrintEvent(timeflip.Event{
		Kind:       timeflip.EventDoubleTap,
		DeviceID:   "tf",
		Source:     timeflip.CharacteristicID("tap-source"),
		ReceivedAt: time.Unix(0, 0).UTC(),
		Payload:    timeflip.DoubleTapEvent{Facet: 4, Pause: true},
	})
	if !strings.Contains(out.String(), "event: orientation / facet changed") ||
		!strings.Contains(out.String(), "source: facet-source") ||
		!strings.Contains(out.String(), "facet: 3") ||
		!strings.Contains(out.String(), "raw_hex: 03") ||
		!strings.Contains(out.String(), "pause_encoded: true") {
		t.Fatalf("missing event output: %q", out.String())
	}
}

func TestFormatterPrintsStreamDecodeWarning(t *testing.T) {
	errOut := &bytes.Buffer{}
	formatter := NewTextFormatter(&bytes.Buffer{}, errOut)
	formatter.PrintError(&timeflip.OperationError{
		Operation: "events",
		DeviceID:  "tf",
		Stage:     "history",
		Err:       timeflip.ErrProtocol,
	})
	if !strings.Contains(errOut.String(), "stream warning:") ||
		!strings.Contains(errOut.String(), "could not decode history notification") ||
		!strings.Contains(errOut.String(), "streaming continues") {
		t.Fatalf("missing stream warning: %q", errOut.String())
	}
}

func TestFormatterPrintsMalformedCommandAcknowledgement(t *testing.T) {
	out := &bytes.Buffer{}
	formatter := NewTextFormatter(out, &bytes.Buffer{})
	formatter.PrintCommandResult(timeflip.CommandResult{
		Command: timeflip.Command{Code: timeflip.CommandCode(0x15)},
		Status:  timeflip.CommandStatus{Code: timeflip.CommandCode(0x15), Raw: []byte{0x15, 0x00}},
		Payload: []byte{0x15, 0x00},
	})
	output := out.String()
	for _, want := range []string{
		"command: 0x15",
		"acknowledgement: unexpected",
		"status_code: 0x00",
		"expected_status: 0x02 OK or 0x01 rejected",
		"raw_payload_hex: 1500",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("malformed command output missing %q in %q", want, output)
		}
	}
	if strings.Contains(output, "ok: false") {
		t.Fatalf("malformed command output should not look like a normal false result: %q", output)
	}
}

func TestFormatterPrintsCommandProtocolErrorGuidance(t *testing.T) {
	errOut := &bytes.Buffer{}
	formatter := NewTextFormatter(&bytes.Buffer{}, errOut)
	formatter.PrintError(&timeflip.OperationError{
		Operation: "send_command",
		DeviceID:  "tf",
		Command:   timeflip.CommandCode(0x15),
		Err:       timeflip.ErrProtocol,
	})
	output := errOut.String()
	for _, want := range []string{
		"command error:",
		"unexpected acknowledgement",
		"command 0x15",
		"status 0x02 (OK) or 0x01 (rejected)",
		"device names must be 1-18 ASCII characters",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("command guidance missing %q in %q", want, output)
		}
	}
}

func TestFormatterHandlesGenericError(t *testing.T) {
	errOut := &bytes.Buffer{}
	formatter := NewTextFormatter(&bytes.Buffer{}, errOut)
	formatter.PrintError(errors.New("boom"))
	if !strings.Contains(errOut.String(), "boom") {
		t.Fatalf("missing generic error: %q", errOut.String())
	}
}

func TestFormatterPrintsSuggestions(t *testing.T) {
	out := &bytes.Buffer{}
	formatter := NewTextFormatter(out, &bytes.Buffer{})
	formatter.PrintSuggestions([]string{"pair tf", "connect tf"})
	if !strings.Contains(out.String(), "next:") || !strings.Contains(out.String(), "pair tf") || !strings.Contains(out.String(), "connect tf") {
		t.Fatalf("missing suggestions: %q", out.String())
	}
}

func TestFormatterCanColorSuggestions(t *testing.T) {
	out := &bytes.Buffer{}
	formatter := NewTextFormatterWithColor(out, &bytes.Buffer{}, true)
	formatter.PrintSuggestions([]string{"pair tf"})
	if !strings.Contains(out.String(), "\033[") || !strings.Contains(out.String(), "pair tf") {
		t.Fatalf("missing colored suggestions: %q", out.String())
	}
}
