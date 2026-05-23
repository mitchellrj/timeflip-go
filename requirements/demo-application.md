# Timeflip2 demo application

## Background

I have a TimeFlip2 physical device which is a piece of hardware that is used to track time spent on tasks. The hardware itself is a regular dodecahedron shape containing accelerometers that detect the orientation of the device, and when it is phsyically tapped against a hard surface.

The device can communicate with the laptop over BLE (Bluetooth Low Energy) via a library/module in this repository. We require a demo app that allows a user to exercise most if not all of the features of the library from the command line with a real device.

We want a demo, interactive command-line application to prove the library functions as desired and serve as an example for other developers, following obvious / expected user journeys.

### Supporting resources

* [Library documentation](../README.md)

## Decision drivers

1. Example code for developers to work from.
2. Empirical proof of functionality of the library/module.

## Scope In

* Interactive CLI application.
* Pairing a new device.
* Unpairing a device.
* Reading any available stored data / configuration on the device.
* Writing any writable configuration.
* Sending any supported device commands.
* Receiving events from the device and displaying them in the CLI.
* Configuration of communication timeouts with command-line flags.

## Scope Out

* Windows support. Reserved as a future task.
* Linux support. Reserved as a future task.
* Business logic interpreting any events.
* Updating device firmware. Reserved as a future task.

## Acceptance Criteria (ACs)

1. User can list supported devices in range.
   **Given** A TimeFlip2 device or devices is/are in BLE range and broadcasting.
   **When** User requests a list of supported devices.
   **Then** User is shown details of the TimeFlip2 device(s).

2. User can pair supported devices in range.
   **Given** A TimeFlip2 device is in range.
   **When** User initiates pairing.
   **Then** The pairing process (specific to the TimeFlip2 device where appropriate) is initiated, and user guided through it.

3. User can unpair supported devices.
   **Given** A TimeFlip2 device is already paired (not necessarily in range).
   **When** User initiates unpairing.
   **Then** The user is guided through the unpairing process and the device is ready to pair again.

4. User can read stored data and configuration, including but not limited to device info, battery status, firmware version, hardware sensor state, etc.
   **Given** A TimeFlip2 device is paired and in range.
   **When** A user requests to read data from the device.
   **Then** User receives details of the requested data, or an appropriate error.

5. User can write writable configuration, including but not limited to device password, tap calibration, etc.
   **Given** A TimeFlip2 device is paired and in range.
   **When** User requests to write configuration by device ID.
   **Then** The app writes the configuration to the device, and the user receives either a success status after confirming the state or appropriate error.

6. User can stream events from the device.
   **Given** A TimeFlip2 device is paired and in range, the app is running, and the user is streaming device events.
   **When** An event is emitted by the device (e.g. orientation change).
   **Then** The user receives the event.
