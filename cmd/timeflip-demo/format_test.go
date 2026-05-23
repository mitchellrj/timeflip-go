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
		ReceivedAt: time.Unix(0, 0).UTC(),
		Payload:    timeflip.FacetEvent{Facet: 3},
		Raw:        []byte{3},
	})
	formatter.PrintEvent(timeflip.Event{
		Kind:       timeflip.EventDoubleTap,
		DeviceID:   "tf",
		ReceivedAt: time.Unix(0, 0).UTC(),
		Payload:    timeflip.DoubleTapEvent{Facet: 4, Pause: true},
	})
	if !strings.Contains(out.String(), "facet=3") || !strings.Contains(out.String(), "raw=03") || !strings.Contains(out.String(), "pause=true") {
		t.Fatalf("missing event output: %q", out.String())
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
