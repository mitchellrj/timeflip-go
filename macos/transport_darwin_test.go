//go:build darwin

package macos

import (
	"testing"

	timeflip "github.com/mitchellrj/timeflip-go"
)

func TestMatchesConnectCandidate(t *testing.T) {
	tests := []struct {
		name        string
		peripheral  timeflip.Peripheral
		requestedID timeflip.DeviceID
		want        bool
	}{
		{
			name:        "exact device id",
			peripheral:  timeflip.Peripheral{ID: "AA:BB", Name: "TIMEFLIP2"},
			requestedID: "AA:BB",
			want:        true,
		},
		{
			name:        "same advertised name rejected by default",
			peripheral:  timeflip.Peripheral{ID: "attacker", Name: "TIMEFLIP2"},
			requestedID: "TIMEFLIP2",
			want:        false,
		},
		{
			name:        "empty requested id rejected",
			peripheral:  timeflip.Peripheral{ID: "AA:BB", Name: "TIMEFLIP2"},
			requestedID: "",
			want:        false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchesConnectCandidate(tt.peripheral, tt.requestedID); got != tt.want {
				t.Fatalf("matchesConnectCandidate()=%v want %v", got, tt.want)
			}
		})
	}
}
