//go:build darwin

package macos

import timeflip "github.com/mitchellrj/timeflip-go"

func normalizeServiceID(id timeflip.ServiceID) timeflip.ServiceID {
	return timeflip.ServiceID(normalizeUUID(string(id)))
}

func normalizeCharacteristicID(id timeflip.CharacteristicID) timeflip.CharacteristicID {
	return timeflip.CharacteristicID(normalizeUUID(string(id)))
}
