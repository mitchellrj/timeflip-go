package timeflip

import (
	"context"
	"errors"
)

// Client is the main TimeFlip2 library entrypoint.
type Client struct {
	config    Config
	transport Transport
}

type advertisedNameProvider interface {
	AdvertisedName(context.Context, DeviceID) (string, bool)
}

// NewClient creates a client from a BLE transport and configuration.
func NewClient(transport Transport, config Config) (*Client, error) {
	if transport == nil {
		return nil, &OperationError{Operation: "new_client", Err: ErrInvalidInput}
	}
	config, err := normalizeConfig(config)
	if err != nil {
		return nil, err
	}
	return &Client{transport: transport, config: config}, nil
}

// ListDevices lists supported TimeFlip2 devices in range.
func (c *Client) ListDevices(ctx context.Context, filter ScanFilter) ([]DiscoveredDevice, error) {
	ctx, cancel := timeoutFrom(ctx, c.config.CommunicationTimeout, 0)
	defer cancel()

	peripherals, err := c.transport.Scan(ctx, filter)
	if err != nil {
		return nil, wrapContextErr("list_devices", "", "", 0, err)
	}
	devices := make([]DiscoveredDevice, 0, len(peripherals))
	for _, p := range peripherals {
		supported := IsSupportedPeripheral(p)
		if !supported && !filter.IncludeUnsupported {
			continue
		}
		devices = append(devices, DiscoveredDevice{
			ID:        p.ID,
			Name:      p.Name,
			RSSI:      p.RSSI,
			Supported: supported,
			Metadata:  cloneMap(p.Metadata),
		})
	}
	return devices, nil
}

// Connect opens an active TimeFlip2 session.
func (c *Client) Connect(ctx context.Context, req ConnectRequest) (*Session, error) {
	if req.DeviceID == "" {
		return nil, &OperationError{Operation: "connect", Err: ErrInvalidInput}
	}
	version := req.ProtocolVersion
	if version == ProtocolAuto {
		version = c.config.ProtocolVersion
	}
	if !validProtocolVersion(version) {
		return nil, &OperationError{Operation: "connect", DeviceID: req.DeviceID, Err: ErrInvalidInput}
	}
	ctx, cancel := timeoutFrom(ctx, c.config.CommunicationTimeout, req.Timeout)
	defer cancel()
	conn, err := c.transport.Connect(ctx, req.DeviceID)
	if err != nil {
		return nil, wrapContextErr("connect", req.DeviceID, "", 0, err)
	}
	return &Session{
		deviceID:             req.DeviceID,
		advertisedName:       req.AdvertisedName,
		advertisedNameReader: c.advertisedNameReader(req.DeviceID),
		conn:                 conn,
		defaultTimeout:       c.config.CommunicationTimeout,
		protocol:             version,
		done:                 make(chan struct{}),
	}, nil
}

func (c *Client) advertisedNameReader(id DeviceID) func(context.Context) (string, bool) {
	return func(ctx context.Context) (string, bool) {
		if provider, ok := c.transport.(advertisedNameProvider); ok {
			if name, ok := provider.AdvertisedName(ctx, id); ok {
				return name, true
			}
		}
		peripherals, err := c.transport.Scan(ctx, ScanFilter{IncludeUnsupported: true})
		if err != nil {
			return "", false
		}
		for _, peripheral := range peripherals {
			if peripheral.ID == id && peripheral.Name != "" {
				return peripheral.Name, true
			}
		}
		return "", false
	}
}

// Pair pairs a new or reset TimeFlip2 device.
func (c *Client) Pair(ctx context.Context, req PairRequest) (PairingResult, error) {
	result := PairingResult{DeviceID: req.DeviceID, Stage: PairingStageConnect}
	if req.DeviceID == "" {
		err := &OperationError{Operation: "pair", DeviceID: req.DeviceID, Err: ErrInvalidInput}
		result.Stages = append(result.Stages, stage(string(PairingStageConnect), false, err, nil))
		return result, err
	}
	password, err := passwordOrDefault(req.Password)
	if err != nil {
		err := &OperationError{Operation: "pair", DeviceID: req.DeviceID, Stage: string(PairingStageAuthorize), Err: ErrInvalidInput}
		result.Stages = append(result.Stages, stage(string(PairingStageAuthorize), false, err, nil))
		return result, err
	}
	session, err := c.Connect(ctx, ConnectRequest{DeviceID: req.DeviceID, Timeout: req.Timeout})
	if err != nil {
		result.Stages = append(result.Stages, stage(string(PairingStageConnect), false, err, nil))
		return result, err
	}
	defer func() {
		_ = session.Close(context.Background())
	}()
	result.Stages = append(result.Stages, stage(string(PairingStageConnect), true, nil, nil))

	if req.AllowOSPairing {
		result.Stage = PairingStageOSPair
		osResult, err := c.transport.PairOS(ctx, req.DeviceID)
		result.Stages = append(result.Stages, stage(string(PairingStageOSPair), err == nil && !osResult.Unsupported, err, osResult.ManualAction))
		if osResult.ManualAction != nil {
			result.ManualAction = osResult.ManualAction
		}
		if err != nil && !errors.Is(err, ErrUnsupportedOperation) {
			return result, wrapContextErr("pair", req.DeviceID, string(PairingStageOSPair), 0, err)
		}
	}

	result.Stage = PairingStageAuthorize
	if _, err := session.Authorize(ctx, password); err != nil {
		result.Stages = append(result.Stages, stage(string(PairingStageAuthorize), false, err, nil))
		return result, err
	}
	result.Stages = append(result.Stages, stage(string(PairingStageAuthorize), true, nil, nil))

	if req.NewPassword != "" {
		result.Stage = PairingStagePassword
		if _, err := session.SetPassword(ctx, req.NewPassword, CommandOptions{Timeout: req.Timeout}); err != nil {
			result.Stages = append(result.Stages, stage(string(PairingStagePassword), false, err, nil))
			return result, err
		}
		result.Stages = append(result.Stages, stage(string(PairingStagePassword), true, nil, nil))
	}

	result.Stage = PairingStageVerify
	if err := session.verifyUsable(ctx); err != nil {
		result.Stages = append(result.Stages, stage(string(PairingStageVerify), false, err, nil))
		return result, err
	}
	result.Stages = append(result.Stages, stage(string(PairingStageVerify), true, nil, nil))
	result.Stage = PairingStageComplete
	result.Completed = true
	return result, nil
}

// Unpair runs device reset and OS unpairing where available.
func (c *Client) Unpair(ctx context.Context, req UnpairRequest) (UnpairingResult, error) {
	result := UnpairingResult{DeviceID: req.DeviceID, Stage: UnpairingStageConnect}
	if req.DeviceID == "" {
		err := &OperationError{Operation: "unpair", Err: ErrInvalidInput}
		result.Stages = append(result.Stages, stage(string(UnpairingStageConnect), false, err, nil))
		return result, err
	}
	needsDeviceAccess := req.Password != "" || req.FactoryReset
	password, err := passwordOrDefault(req.Password)
	if needsDeviceAccess && err != nil {
		err := &OperationError{Operation: "unpair", DeviceID: req.DeviceID, Stage: string(UnpairingStageAuthorize), Err: ErrInvalidInput}
		result.Stages = append(result.Stages, stage(string(UnpairingStageAuthorize), false, err, nil))
		return result, err
	}
	if needsDeviceAccess {
		session, err := c.Connect(ctx, ConnectRequest{DeviceID: req.DeviceID, Timeout: req.Timeout})
		if err == nil {
			defer func() {
				_ = session.Close(context.Background())
			}()
			result.Stages = append(result.Stages, stage(string(UnpairingStageConnect), true, nil, nil))
			result.Stage = UnpairingStageAuthorize
			if _, err := session.Authorize(ctx, password); err != nil {
				result.Stages = append(result.Stages, stage(string(UnpairingStageAuthorize), false, err, nil))
				return result, err
			}
			result.Stages = append(result.Stages, stage(string(UnpairingStageAuthorize), true, nil, nil))
			if req.FactoryReset {
				result.Stage = UnpairingStageDeviceReset
				if _, err := session.FactoryReset(ctx, CommandOptions{Timeout: req.Timeout}); err != nil {
					result.Stages = append(result.Stages, stage(string(UnpairingStageDeviceReset), false, err, nil))
					return result, err
				}
				result.DeviceResetComplete = true
				result.Stages = append(result.Stages, stage(string(UnpairingStageDeviceReset), true, nil, nil))
			}
		} else {
			result.Stages = append(result.Stages, stage(string(UnpairingStageConnect), false, err, nil))
		}
	}
	if req.AllowOSUnpairing {
		result.Stage = UnpairingStageOSUnpair
		osResult, err := c.transport.UnpairOS(ctx, req.DeviceID)
		result.OSUnpairComplete = err == nil && osResult.Performed
		result.ManualAction = osResult.ManualAction
		result.Stages = append(result.Stages, stage(string(UnpairingStageOSUnpair), result.OSUnpairComplete, err, osResult.ManualAction))
		if err != nil && !errors.Is(err, ErrUnsupportedOperation) {
			return result, wrapContextErr("unpair", req.DeviceID, string(UnpairingStageOSUnpair), 0, err)
		}
	}
	result.Completed = (req.FactoryReset == result.DeviceResetComplete || !req.FactoryReset) && (result.OSUnpairComplete || result.ManualAction != nil || !req.AllowOSUnpairing)
	result.Stage = UnpairingStageComplete
	return result, nil
}

func stage(name string, completed bool, err error, action *ManualAction) StageResult {
	return StageResult{Stage: name, Completed: completed, Err: err, ManualAction: action}
}

func cloneMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func wrapContextErr(operation string, deviceID DeviceID, stage string, command CommandCode, err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		err = ErrTimeout
	}
	return &OperationError{Operation: operation, DeviceID: deviceID, Stage: stage, Command: command, Err: err}
}
