package macos

import "testing"

func TestNormalizeUUID(t *testing.T) {
	tests := map[string]string{
		"0x2a19":                                 "0x2A19",
		"2a19":                                   "0x2A19",
		"00002a19":                               "0x2A19",
		"00002a19-0000-1000-8000-00805f9b34fb":   "0x2A19",
		"f1196f50-71a4-11e6-bdf4-0800200c9a66":   "F1196F50-71A4-11E6-BDF4-0800200C9A66",
		"{f1196f51-71a4-11e6-bdf4-0800200c9a66}": "F1196F51-71A4-11E6-BDF4-0800200C9A66",
	}
	for input, want := range tests {
		if got := normalizeUUID(input); got != want {
			t.Fatalf("normalizeUUID(%q) = %q, want %q", input, got, want)
		}
	}
}
