// Package timeflip provides a Go integration layer for TimeFlip2 BLE devices.
package timeflip

import (
	"context"
	"time"
)

// DeviceID identifies a BLE peripheral as reported by the active transport.
type DeviceID string

// CharacteristicID identifies a BLE GATT characteristic.
type CharacteristicID string

// ServiceID identifies a BLE GATT service.
type ServiceID string

// CommandCode identifies a TimeFlip2 command byte.
type CommandCode byte

// FacetID identifies a TimeFlip2 facet.
type FacetID uint8

// DefaultPassword is the factory TimeFlip2 password, encoded as ASCII bytes.
const DefaultPassword = "000000"

// Config configures a Client.
type Config struct {
	CommunicationTimeout time.Duration
}

// CommandOptions provides per-command behavior.
type CommandOptions struct {
	Timeout time.Duration
}

// EventOptions configures event streaming.
type EventOptions struct {
	Buffer     int
	IncludeRaw bool
}

// ScanFilter configures BLE discovery.
type ScanFilter struct {
	IncludeUnsupported bool
}

// Peripheral is a platform-neutral BLE peripheral description.
type Peripheral struct {
	ID                 DeviceID
	Name               string
	RSSI               int
	AdvertisedServices []ServiceID
	ManufacturerData   []byte
	Metadata           map[string]string
}

// DiscoveredDevice is returned by ListDevices.
type DiscoveredDevice struct {
	ID        DeviceID
	Name      string
	RSSI      int
	Supported bool
	Metadata  map[string]string
}

// Notification is a BLE characteristic notification.
type Notification struct {
	Characteristic CharacteristicID
	Payload        []byte
}

// ManualActionKind describes user or caller action needed outside the library.
type ManualActionKind string

const (
	ManualActionOSPair   ManualActionKind = "os_pair"
	ManualActionOSUnpair ManualActionKind = "os_unpair"
)

// ManualAction describes action a caller or user must perform manually.
type ManualAction struct {
	Kind        ManualActionKind
	Description string
	Inputs      map[string]string
}

// OSActionResult describes OS pairing/unpairing work.
type OSActionResult struct {
	Performed    bool
	Unsupported  bool
	ManualAction *ManualAction
}

// Transport provides platform BLE operations.
type Transport interface {
	Scan(context.Context, ScanFilter) ([]Peripheral, error)
	Connect(context.Context, DeviceID) (Connection, error)
	PairOS(context.Context, DeviceID) (OSActionResult, error)
	UnpairOS(context.Context, DeviceID) (OSActionResult, error)
}

// Connection provides active BLE connection operations.
type Connection interface {
	Read(context.Context, CharacteristicID) ([]byte, error)
	Write(context.Context, CharacteristicID, []byte) error
	Subscribe(context.Context, CharacteristicID) (<-chan Notification, error)
	Close(context.Context) error
}

// PairRequest starts pairing for a new or reset TimeFlip2 device.
type PairRequest struct {
	DeviceID       DeviceID
	Password       string
	NewPassword    string
	AllowOSPairing bool
	Timeout        time.Duration
}

// UnpairRequest starts device reset and OS unpairing work.
type UnpairRequest struct {
	DeviceID         DeviceID
	Password         string
	FactoryReset     bool
	AllowOSUnpairing bool
	Timeout          time.Duration
}

// ConnectRequest opens a connection to a TimeFlip2 device.
type ConnectRequest struct {
	DeviceID DeviceID
	Timeout  time.Duration
}

// PairingStage identifies a pairing workflow stage.
type PairingStage string

const (
	PairingStageConnect   PairingStage = "connect"
	PairingStageOSPair    PairingStage = "os_pair"
	PairingStageAuthorize PairingStage = "authorize"
	PairingStagePassword  PairingStage = "set_password"
	PairingStageVerify    PairingStage = "verify"
	PairingStageComplete  PairingStage = "complete"
)

// UnpairingStage identifies an unpairing workflow stage.
type UnpairingStage string

const (
	UnpairingStageConnect     UnpairingStage = "connect"
	UnpairingStageAuthorize   UnpairingStage = "authorize"
	UnpairingStageDeviceReset UnpairingStage = "device_reset"
	UnpairingStageOSUnpair    UnpairingStage = "os_unpair"
	UnpairingStageComplete    UnpairingStage = "complete"
)

// StageResult records one workflow stage result.
type StageResult struct {
	Stage        string
	Completed    bool
	Err          error
	ManualAction *ManualAction
}

// PairingResult records the pairing workflow outcome.
type PairingResult struct {
	DeviceID     DeviceID
	Completed    bool
	Stage        PairingStage
	Stages       []StageResult
	ManualAction *ManualAction
}

// UnpairingResult records the unpairing workflow outcome.
type UnpairingResult struct {
	DeviceID            DeviceID
	Completed           bool
	DeviceResetComplete bool
	OSUnpairComplete    bool
	Stage               UnpairingStage
	Stages              []StageResult
	ManualAction        *ManualAction
}

// AuthorizationResult records password authorization state.
type AuthorizationResult struct {
	Authorized bool
}

// DeviceInfo contains readable BLE device information.
type DeviceInfo struct {
	Name             string
	ManufacturerName string
	ModelNumber      string
	HardwareRevision string
	FirmwareRevision string
	SystemID         string
	Raw              map[CharacteristicID][]byte
}

// BatteryStatus is the current battery percentage.
type BatteryStatus struct {
	Percentage uint8
	Raw        []byte
}

// SystemState describes TimeFlip2 sync and hardware state.
type SystemState struct {
	StatusCode          uint16
	StatusDescription   string
	HardwareCode        uint16
	HardwareDescription string
	SyncRequired        bool
	SyncReason          string
	Reset               bool
	HardwareIssue       bool
	Raw                 []byte
}

// TrackerStatus describes lock, pause, and auto-pause state.
type TrackerStatus struct {
	LockEnabled      bool
	PauseEnabled     bool
	AutoPauseMinutes uint16
	Raw              []byte
}

// CommandStatus is a command acknowledgement.
type CommandStatus struct {
	Code CommandCode
	OK   bool
	Raw  []byte
}

// CommandResult is returned after command writes.
type CommandResult struct {
	Command Command
	Status  CommandStatus
	Payload []byte
}

// Command is a supported TimeFlip2 command.
type Command struct {
	Code    CommandCode
	Payload []byte
}

// RGB contains a facet color value.
type RGB struct {
	R uint16
	G uint16
	B uint16
}

// TaskParameters describes a facet task configuration.
type TaskParameters struct {
	Facet                FacetID
	Assigned             bool
	Mode                 uint8
	PomodoroLimitSeconds uint32
	ElapsedSeconds       uint32
	Raw                  []byte
}

// TapSettings describes double-tap accelerometer register values.
type TapSettings struct {
	Configured bool
	Threshold  uint8
	Limit      uint8
	Latency    uint8
	Window     uint8
	Raw        []byte
}

// HistoryRequest requests history from a starting event number.
type HistoryRequest struct {
	StartEvent uint32
	All        bool
}

// HistoryEntry describes one decoded history entry.
type HistoryEntry struct {
	EventNumber         uint32
	Facet               FacetID
	UndefinedFacet      bool
	AccelerometerError  bool
	Pause               bool
	MomentUnix          uint64
	DurationSeconds     uint32
	PreviousEventNumber uint32
	Raw                 []byte
}

// HistoryStreamState describes whether a history packet terminates a stream.
type HistoryStreamState struct {
	Complete            bool
	PreviousEventNumber uint32
}

// EventKind describes the type of a technical device event.
type EventKind string

const (
	EventConnectionState EventKind = "connection_state"
	EventFacet           EventKind = "facet"
	EventDoubleTap       EventKind = "double_tap"
	EventBattery         EventKind = "battery"
	EventSystemState     EventKind = "system_state"
	EventHistory         EventKind = "history"
	EventCommandResult   EventKind = "command_result"
	EventRaw             EventKind = "raw"
)

// Event is emitted by a Session.
type Event struct {
	Kind       EventKind
	DeviceID   DeviceID
	Source     CharacteristicID
	ReceivedAt time.Time
	Payload    any
	Raw        []byte
}

// FacetEvent describes a facet notification.
type FacetEvent struct {
	Facet         FacetID
	Undefined     bool
	WrongPassword bool
	Raw           []byte
}

// DoubleTapEvent describes a double-tap notification.
type DoubleTapEvent struct {
	Facet FacetID
	Pause bool
	Raw   []byte
}
