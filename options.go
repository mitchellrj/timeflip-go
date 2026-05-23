package timeflip

import (
	"context"
	"time"
)

const defaultCommunicationTimeout = 10 * time.Second

func normalizeConfig(config Config) (Config, error) {
	if config.CommunicationTimeout < 0 {
		return Config{}, &OperationError{Operation: "configure", Err: ErrInvalidInput}
	}
	if config.CommunicationTimeout == 0 {
		config.CommunicationTimeout = defaultCommunicationTimeout
	}
	return config, nil
}

func timeoutFrom(ctx context.Context, base time.Duration, override time.Duration) (context.Context, context.CancelFunc) {
	if override > 0 {
		return context.WithTimeout(ctx, override)
	}
	if base > 0 {
		return context.WithTimeout(ctx, base)
	}
	return context.WithCancel(ctx)
}
