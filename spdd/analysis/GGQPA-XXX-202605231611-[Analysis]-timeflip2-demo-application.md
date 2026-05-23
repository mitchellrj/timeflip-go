# SPDD Analysis: TimeFlip2 Demo Application

## Original Business Requirement

# Timeflip2 demo application

## Background

I have a TimeFlip2 physical device which is a piece of hardware that is used to track time spent on tasks. The hardware itself is a regular dodecahedron shape containing accelerometers that detect the orientation of the device, and when it is phsyically tapped against a hard surface.

The device can communicate with the laptop over BLE (Bluetooth Low Energy) via a library/module in this repository. We require a demo app that allows a user to exercise most if not all of the features of the library from the command line with a real device.

We want a demo, interactive command-line application to prove the library functions as desired and serve as an example for other developers, following obvious / expected user journeys.

### Supporting resources

* [Library documentation](./README.md)

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

## Domain Concept Identification

#### Existing Concepts (from codebase)

- Go Library API: The root `timeflip` package exposes the reusable library surface that the demo application should exercise, including client construction, device listing, pairing, unpairing, connection sessions, reads, writes, commands, events, and timeout configuration.
- Client: The primary entrypoint for discovery, connection, pairing, and unpairing. It owns a transport and global communication timeout, which makes it the natural demo application integration boundary.
- Session: The active device connection concept used for authorization, reading device data, writing configuration, sending commands, event streaming, and closing the connection.
- Transport and Connection: Platform-neutral BLE interfaces used by the library. These separate command-line user journeys from the underlying MacOS BLE implementation.
- MacOS Transport Boundary: The `macos` package currently compiles as a placeholder adapter and returns unsupported/manual-action outcomes for BLE scan/connect/pair/unpair behavior until a concrete CoreBluetooth-backed implementation is added.
- Pairing Workflow: Existing staged library behavior for connect, optional OS pairing, authorization, optional password change, verification, manual action reporting, and completion status.
- Unpairing Workflow: Existing staged library behavior for optional connection/authorization, optional factory reset, optional OS unpairing, manual action reporting, and completion status.
- Device Discovery: Existing `ListDevices` and `IsSupportedPeripheral` behavior identifies TimeFlip2 devices using advertised service, name, or metadata heuristics and can include or filter unsupported devices.
- Device Data Reads: Existing session reads include device information, battery status, system state, history, task parameters, and tap settings.
- Writable Configuration and Commands: Existing session methods support password, name, lock, pause, auto-pause, LED, facet color, task parameters, tap settings, task reset, factory reset, and generic supported command dispatch.
- Device Events: Existing event streaming subscribes to BLE notifications and surfaces typed technical events for facet changes, double taps, battery, system state, history, and optional raw payloads.
- Error Model: Existing contextual errors expose operation, device ID, stage, command, timeout, authorization failure, unsupported operation, command rejection, protocol error, and disconnected states.
- Existing Examples: `examples/basic` lists devices, and `examples/pairing` starts pairing. They are useful seed examples but are not an interactive demo shell and do not exercise most library features.

#### New Concepts Required

- Interactive Demo CLI: A command-line application that provides a user-facing shell or menu for exercising the library through realistic device journeys rather than one-off examples.
- Demo User Journey: A guided flow that combines discovery, selection, pairing, connecting, authorization, reads, writes, commands, event streaming, and unpairing in an order a real user can follow.
- Device Selection Context: A demo-level notion of the current target device or explicit device ID entry for operations that require a device. This should remain process-local and user-driven, not library-owned storage.
- Session Lifecycle Presentation: CLI behavior that makes connection state, authorization state, active streaming, cancellation, and closure understandable to the user.
- Command Catalog Presentation: A demo-level grouping of readable values, writable configuration, and supported commands so users can discover available operations without reading source code.
- Input and Confirmation Flow: CLI prompts and validation for passwords, timeouts, facet IDs, names, LED values, tap calibration values, reset commands, and other potentially disruptive operations.
- Event Display Loop: A CLI display mode for streaming technical events until the user cancels, while preserving the library boundary that does not interpret events as business time tracking.
- Demo Output Formatting: A consistent presentation for device lists, workflow stages, manual actions, read results, command results, stream events, and errors.
- Hardware Smoke-Test Path: A documented way to run the demo against a real device, distinguish unsupported adapter behavior from device failures, and capture empirical proof of library functionality.

#### Key Business Rules

- The demo must exercise the library, not bypass it. BLE protocol and transport mechanics should remain behind the existing `timeflip.Client`, `Session`, `Transport`, and `Connection` concepts.
- The demo should guide obvious user journeys, especially discovery before pairing or connection, authorization before protected operations, and clean stream cancellation before exit.
- The demo must not interpret device events as task tracking, billing, productivity state, or business decisions. It may display technical event data only.
- The demo must not introduce Windows or Linux support. It can keep platform integration MacOS-oriented while preserving the library's transport boundary.
- The demo must not update firmware or expose firmware-loader behavior as an ordinary command path.
- Timeout configuration must be surfaced through command-line flags that map to the library's global communication timeout and per-command override behavior.
- User-entered passwords and device IDs can be used during the active CLI process, but the demo should not create persistent storage or a hidden device/password registry unless a later requirement explicitly introduces that behavior.
- Potentially destructive writes, especially password changes, task reset, factory reset, and unpairing, should be clearly confirmed by the user before execution.
- Manual actions returned by the library are part of the user journey and must be displayed as actionable guidance rather than treated as opaque errors.
- Empirical proof depends on a real transport implementation. If the current MacOS adapter still reports unsupported scan/connect behavior, the demo can compile and document the flow but cannot by itself prove hardware behavior.

## Strategic Approach

#### Solution Direction

- Build a dedicated interactive command-line demo application on top of the existing public `timeflip` package. It should act as a thin, user-friendly exercise harness for the library rather than a second implementation of device behavior.
- Organize the demo around the library's natural lifecycle: configure timeout, list devices, choose or enter a device ID, pair if needed, connect and authorize, read data, write configuration, send supported commands, stream events, unpair, and exit.
- Preserve the existing examples as minimal snippets, but introduce a fuller demo application for real-device walkthroughs and developer exploration.
- Keep the demo process-local and stateless by default. Any selected device, active session, and user-entered values should live only for the current run unless future requirements explicitly add persistence.
- Treat manual-action results and unsupported OS adapter behavior as first-class CLI output, since the current MacOS adapter boundary is intentionally incomplete.
- Use the existing fake transport and session test patterns to verify CLI command orchestration where possible, while leaving real BLE validation to hardware smoke tests.

#### Key Design Decisions

- Demo shape: A single interactive CLI shell better satisfies "exercise most if not all features" than adding more one-off examples. The trade-off is a larger command surface, but it gives developers one place to explore the library. Recommendation: create a dedicated demo command while keeping existing examples small.
- User flow style: Menu-driven flows are approachable for hardware testing, while command-style shells are faster for repeated developer use. Recommendation: use clear named commands with help output and guided prompts for operations needing multiple inputs.
- Device context: Requiring a device ID on every command is explicit but repetitive; retaining a current selection is more ergonomic but must not become hidden persistence. Recommendation: allow a current device selection for the active process and always display which device/session an operation will target.
- Session handling: Opening a fresh connection per command is simple but slow and poor for streaming; keeping an active session mirrors real usage but needs visible lifecycle controls. Recommendation: support an active session concept in the CLI with explicit connect, authorize, stream, close, and exit behavior.
- Command coverage: Exposing every low-level command as raw bytes would be comprehensive but not developer-friendly. Recommendation: present named operations for the existing typed session methods and reserve generic command dispatch for advanced/diagnostic use if included at all.
- Output format: Human-readable text is easiest for an interactive demo; structured output would help automation. Recommendation: prioritize readable tables and labeled fields, with any machine-readable mode treated as optional future scope.
- Error handling: Exiting on every error would make the demo frustrating during hardware experimentation. Recommendation: display contextual errors and keep the interactive loop alive when recovery is possible.
- Destructive operations: A frictionless factory reset or password change is dangerous in a demo. Recommendation: require explicit confirmation for disruptive writes and unpairing actions while keeping ordinary reads and event streaming lightweight.
- Hardware validation: Unit tests can verify orchestration and formatting, but only a real MacOS BLE adapter plus physical device can prove the end-to-end user journeys. Recommendation: document the current adapter limitation and include a smoke-test checklist once the concrete adapter is available.

#### Alternatives Considered

- Expand only `examples/basic` and `examples/pairing`: Rejected because separate examples do not provide an interactive journey or exercise most of the library from one process.
- Build a non-interactive command suite only: Rejected as the primary shape because the requirement asks for an interactive CLI and guided pairing/unpairing user journeys, though command flags remain useful for startup configuration.
- Add persistence for paired devices and passwords: Rejected because the library is intentionally stateless and the demo requirement does not ask for a device registry or credential store.
- Interpret facet events as task labels: Rejected because business logic interpreting events remains out of scope.
- Implement a new BLE stack inside the demo: Rejected because it would bypass the library and weaken the demo's value as example code for developers.
- Treat current `macos.Transport` unsupported behavior as sufficient hardware proof: Rejected because empirical proof requires a concrete transport capable of scanning, connecting, reading, writing, and subscribing against a real device.

## Risk & Gap Analysis

- Requirement ambiguity: The command grammar and interaction model are unspecified. The REASONS Canvas should decide the top-level command set, prompt behavior, help output, and whether commands are menu-driven, shell-like, or hybrid.
- Requirement ambiguity: "Most if not all" library features needs a coverage boundary. The current library includes typed methods, generic command dispatch, history, events, manual OS actions, and destructive commands; the demo should identify which are first-class flows.
- Requirement ambiguity: The required password handling experience is not specified. The demo needs a strategy for prompting, masking, confirming changes, and avoiding accidental disclosure in logs or command history.
- Requirement ambiguity: The supporting resource link points to `./README.md` from inside `requirements/demo-application.md`, which may imply `requirements/README.md`; the actual library documentation is at the repository root `README.md`.
- Requirement ambiguity: "Pairing" and "unpairing" at CLI level may include OS-level manual actions, TimeFlip password authorization, password changes, and factory reset choices. The demo should make these stages visible without overpromising direct OS support.
- Edge case: Multiple supported devices may appear in range, disappear between scan and connect, have similar names, or expose incomplete advertising data.
- Edge case: The user may start reads, writes, or streaming without a selected device, without an active session, or before successful authorization.
- Edge case: A device can disconnect during pairing, reading, writing, history retrieval, or streaming. The CLI should report the failure and leave the user in a recoverable state.
- Edge case: Event streaming can block ordinary prompt interaction if the UI model is not planned. The CLI needs a clear cancel/stop behavior.
- Edge case: Long-running streams and command timeouts interact differently. Streaming should respect cancellation while command operations respect global and per-command timeout choices.
- Edge case: Read and command responses may be malformed, rejected, or unsupported by firmware. The CLI should display protocol and command errors plainly.
- Edge case: Destructive operations such as password change, task reset, factory reset, and unpairing may leave the device in a state requiring manual recovery.
- Technical risk: The current `macos` transport cannot yet scan or connect to real hardware. Without a concrete adapter, the demo can compile and show unsupported/manual-action flows but cannot meet the empirical proof driver.
- Technical risk: The existing module has no external CLI framework dependency. Adding one may improve command parsing but increases footprint; using standard library parsing keeps dependencies minimal but may require more local structure.
- Technical risk: Tests can exercise CLI orchestration with fakes, but real BLE behavior, OS permissions, Bluetooth state, and notification delivery still require manual hardware validation.
- Technical risk: The current library has root-package tests and an internal fake transport used for testing. The demo may need its own testable boundary rather than importing test-only helpers.
- Technical risk: Event display formatting must handle heterogeneous payload types without leaking confusing raw protocol details as the primary developer experience.
- Acceptance Criteria coverage: AC1 is addressable through a demo command that calls `Client.ListDevices` and displays discovered TimeFlip2 details, subject to a real MacOS transport implementation.
- Acceptance Criteria coverage: AC2 is addressable through a guided pairing flow using `Client.Pair`, stage results, manual actions, password input, and optional password change behavior.
- Acceptance Criteria coverage: AC3 is addressable through a guided unpairing flow using `Client.Unpair`, device reset options, OS unpairing options, manual actions, and recovery messaging.
- Acceptance Criteria coverage: AC4 is addressable through read commands for existing session methods: device info, battery, system state, history, task parameters, and tap settings. The exact "any available" list should be made explicit in the REASONS Canvas.
- Acceptance Criteria coverage: AC5 is addressable through write/configuration commands for existing typed session methods, with confirmation and follow-up status output. The exact writable set should be enumerated to avoid accidental omission.
- Acceptance Criteria coverage: AC6 is addressable through a stream mode that calls `Session.Events` and prints typed technical events until user cancellation, subject to subscription support in the active transport.
