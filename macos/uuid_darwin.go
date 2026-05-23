//go:build darwin

package macos

import (
	timeflip "github.com/mitchellrj/timeflip-go"
	"tinygo.org/x/bluetooth"
)

func serviceID(uuid bluetooth.UUID) timeflip.ServiceID {
	return timeflip.ServiceID(normalizeUUID(uuid.String()))
}

func characteristicID(uuid bluetooth.UUID) timeflip.CharacteristicID {
	return timeflip.CharacteristicID(normalizeUUID(uuid.String()))
}
