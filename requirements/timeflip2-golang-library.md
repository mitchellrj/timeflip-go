# Timeflip2 golang

## Background

I have a TimeFlip2 device which is a piece of hardware that is used to track time spent on tasks. The hardware itself is a regular dodecahedron shape containing accelerometers that detect the orientation of the device, and when it is phsyically tapped against a hard surface.

The device normally communicates with a mobile phone app over BLE (Bluetooth Low Energy). The app then interprets the orientation and movements of the device as signals to start tracking time, stop tracking time, and what task to track time against. The user can label each face of the dodecahedron with labels or symbols that indicate different tasks, e.g. "documentation", "coding", "admin". The face (facet) that is facing up is considered active. The mobile app also shares events and data with a managed cloud API that supports some integrations.

So as not to be dependent on a mobile device or a cloud API which may eventually be deprecated, I wish to build a Golang library which can run on a laptop (initial target MacOS) and integrate with other Golang applications which consume events from this library.

### Supporting resources

* [Official hardware protocol documentation](./docs/Hardware)
* [Android app version 2.3.17](./android-app/TIMEFLIP2_ Time%26Task tracker_2.3.17_APKPure.xapk)

## Decision drivers

1. Independence from cloud APIs.
2. Platform and architecture portability.
3. Small, simple, reusable library.

## Scope In

* Pairing a new device.
* Unpairing a device.
* Reading any available stored data / configuration on the device.
* Writing any writable configuration.
* Sending any supported commands.
* Receiving events from the device.
* Masking the protocol and communication medium from integrators.
* Documenting how to use the library as an integrator.
* Configuration of communication timeouts.
* Any storage or caching of device state.

## Scope Out

* Windows support. Reserved as a future task.
* Linux support. Reserved as a future task.
* Business logic interpreting any events.
* Updating device firmware. Reserved as a future task.

## Acceptance Criteria (ACs)

1. Consuming app can list supported devices in range.
   **Given** A TimeFlip2 device or devices is/are in BLE range and broadcasting.
   **When** Consuming app requests a list of supported devices.
   **Then** The consuming app receives details of the TimeFlip2 device(s).

2. Consuming app can pair supported devices in range.
   **Given** A TimeFlip2 device is in range.
   **When** A Consuming app initiates pairing by device ID.
   **Then** The pairing process (specific to the TimeFlip2 device where appropriate) is initiated, and following stages can be executed, and error states revealed and recovered from where possible.

3. Consuming app can unpair supported devices.
   **Given** A TimeFlip2 device is already paired (not necessarily in range).
   **When** A Consuming app initiates unpairing by device ID.
   **Then** The unpairing process (specific to the TimeFlip2 device where appropriate) is initiated, and following stages can be executed, and error states revealed and recovered from where possible.

4. Consuming app can read stored data and configuration, including but not limited to device info, battery status, firmware version, hardware sensor state, etc.
   **Given** A TimeFlip2 device is paired and in range.
   **When** A consuming app requests to read data by device ID.
   **Then** The consuming app receives details of the requested data, or an appropriate error.

5. Consuming app can write writable configuration, including but not limited to device password, tap calibration, etc.
   **Given** A TimeFlip2 device is paired and in range.
   **When** A consuming app requests to write configuration by device ID.
   **Then** The consuming app writes the configuration to the device, and receives either a success status after confirming the state or appropriate error.

6. Consuming app can receive events from the device in a Golang channel.
   **Given** A TimeFlip2 device is paired and in range.
   **When** An event is emitted by the device (e.g. orientation change).
   **Then** The consuming app receives the event.

7. Consuming app developers have good quality integration documentation to work from.