package macos

import (
	"fmt"
	"strings"
)

const bluetoothBaseSuffix = "-0000-1000-8000-00805F9B34FB"

func normalizeUUID(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return ""
	}
	id = strings.TrimPrefix(id, "{")
	id = strings.TrimSuffix(id, "}")
	id = strings.ToUpper(id)
	if strings.HasPrefix(id, "0X") {
		return fmt.Sprintf("0x%04s", id[2:])
	}
	if len(id) == 4 && isHex(id) {
		return "0x" + id
	}
	if len(id) == 8 && strings.HasPrefix(id, "0000") && isHex(id[4:]) {
		return "0x" + id[4:]
	}
	if strings.HasPrefix(id, "0000") && strings.HasSuffix(id, bluetoothBaseSuffix) {
		return "0x" + id[4:8]
	}
	return id
}

func isHex(s string) bool {
	for _, r := range s {
		if (r < '0' || r > '9') && (r < 'A' || r > 'F') {
			return false
		}
	}
	return true
}
