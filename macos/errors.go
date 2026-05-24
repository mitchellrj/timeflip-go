//go:build darwin

package macos

import (
	"context"
	"errors"

	timeflip "github.com/mitchellrj/timeflip-go"
)

func operationErr(operation string, id timeflip.DeviceID, err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		err = timeflip.ErrTimeout
	}
	return &timeflip.OperationError{Operation: operation, DeviceID: id, Err: err}
}
