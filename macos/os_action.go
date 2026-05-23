package macos

import timeflip "github.com/mitchellrj/timeflip-go"

func unsupportedOSAction(kind timeflip.ManualActionKind, id timeflip.DeviceID, description string) (timeflip.OSActionResult, error) {
	return timeflip.OSActionResult{
		Unsupported: true,
		ManualAction: &timeflip.ManualAction{
			Kind:        kind,
			Description: description,
			Inputs: map[string]string{
				"device_id": string(id),
			},
		},
	}, timeflip.ErrUnsupportedOperation
}
