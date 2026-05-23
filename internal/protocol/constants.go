package protocol

const (
	GenericAccessService     = "0x1800"
	DeviceNameCharacteristic = "0x2A00"
	DeviceInformationService = "0x180A"
	ManufacturerNameString   = "0x2A29"
	ModelNumberString        = "0x2A24"
	HardwareRevisionString   = "0x2A27"
	FirmwareRevisionString   = "0x2A26"
	SystemID                 = "0x2A23"
	BatteryService           = "0x180F"
	BatteryLevel             = "0x2A19"
	TimeFlipService          = "F1196F50-71A4-11E6-BDF4-0800200C9A66"
	TimeFlipEvents           = "F1196F51-71A4-11E6-BDF4-0800200C9A66"
	Facets                   = "F1196F52-71A4-11E6-BDF4-0800200C9A66"
	CommandResultOutput      = "F1196F53-71A4-11E6-BDF4-0800200C9A66"
	Command                  = "F1196F54-71A4-11E6-BDF4-0800200C9A66"
	DoubleTap                = "F1196F55-71A4-11E6-BDF4-0800200C9A66"
	SystemState              = "F1196F56-71A4-11E6-BDF4-0800200C9A66"
	PasswordCheck            = "F1196F57-71A4-11E6-BDF4-0800200C9A66"
	HistoryData              = "F1196F58-71A4-11E6-BDF4-0800200C9A66"
)

// RequiredCharacteristics returns characteristics needed for normal operation.
func RequiredCharacteristics() []string {
	return []string{
		TimeFlipEvents,
		Facets,
		CommandResultOutput,
		Command,
		DoubleTap,
		SystemState,
		PasswordCheck,
		HistoryData,
	}
}
