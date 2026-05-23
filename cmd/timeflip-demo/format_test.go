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
	if !strings.Contains(out.String(), "pair in settings") || !strings.Contains(out.String(), "device_id=tf") {
		t.Fatalf("missing manual action output: %q", out.String())
	}
	if !strings.Contains(errOut.String(), "operation=pair") || !strings.Contains(errOut.String(), "adapter note") {
		t.Fatalf("missing operation error output: %q", errOut.String())
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
