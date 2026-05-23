# SPDD Analysis: TimeFlip2 Golang Library

## Original Business Requirement

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

## Clarifications

* Pairing: all aspects of pairing a new or reset device
* Unpairing: any device-specific reset required (e.g. removing password) and OS unpairing
* Any OS-level BLE device pairing/unpairing is only required so far as directly supportable by the OS APIs - where not supported, the library should at least support returning any inputs for those calls or for user-initiated actions that are required.
* Do not discern between different sensitivity levels of operations (reading or writing). This is a task for consuming applications to consider.
* Any storage or caching of device state: there should be no storage, by the library itself, in files, memory, cache, etc. The only exception is maintaining state of an ongoing process abstracted by the library, e.g. pairing, or a connection object which may have configuration associated with it (device ID, default timeout, etc).
* Timeouts: one global communication timeout, plus optional overrides per command

## Domain Concept Identification

#### Existing Concepts (from codebase)

- TimeFlip2 Protocol Documentation: The repository contains hardware documentation for the BLE protocol, including device information, battery, TimeFlip service characteristics, command behavior, password authorization, system state, facet notification, double-tap notification, and history data. This is not implementation code yet, but it is the authoritative existing domain source.
- TimeFlip Device: A BLE peripheral used for time tracking by physical orientation and tap gestures. It owns on-device state and emits data that consuming applications will receive through the library.
- Facet: A face of the device that represents the currently active tracking surface. Documentation distinguishes current facet notifications, undefined facet state, paused task encodings, and historical side values.
- Device Event: A device-originated signal such as facet/orientation change, double tap, battery notification, system state update, or history/event-log entry. Events are the primary runtime output of the library.
- Command: A protocol operation sent to the device to change or request state, such as lock, pause, auto-pause, time synchronization, LED settings, facet color, task parameters, tap settings, password changes, task reset, factory reset, and history reads.
- Device State and Configuration: Readable or writable on-device values including device identity, firmware, battery level, system state, password authorization state, calibration/synchronization state, timer/lock/pause state, task parameters, color settings, and name.
- History: Stored device intervals/events retained by the device and retrieved through the protocol. History is related to facets, pauses, timestamps, durations, and device error states.
- Pairing and Authorization: The requirement now clarifies pairing as all aspects of pairing a new or reset device. Protocol documentation specifically describes password-based authorization on reconnect, while OS-level BLE pairing is only required where directly supportable by OS APIs.

#### New Concepts Required

- Go Library API: A reusable package boundary that shields integrators from BLE transport details while exposing scanning, pairing, unpairing, reads, writes, commands, events, and communication timeout configuration.
- Pairing Workflow: A library-managed in-progress operation for pairing a new or reset device, covering device discovery, connection, TimeFlip-specific setup/authorization, recoverable stages, and any OS-level pairing steps that MacOS APIs directly support.
- Unpairing Workflow: A library-managed in-progress operation for device-specific reset or password removal plus OS unpairing where directly supported, with fallback outputs for user-initiated OS actions where APIs cannot perform the action directly.
- Connection Session: A lifecycle concept for an active BLE connection, including authorization, service discovery, reads/writes, notification subscriptions, timeout behavior, recovery, and shutdown.
- Protocol Adapter: A conceptual boundary that translates documented TimeFlip2 BLE services, characteristics, commands, responses, and notifications into library-level concepts without leaking raw protocol details as the main integration surface.
- Event Stream: A Go channel-oriented abstraction that carries normalized device events to consuming applications while preserving enough source information for applications to decide their own business behavior.
- Error Model: A consistent way to expose discoverability failures, connection failures, authorization failures, unsupported device/protocol states, malformed device data, command rejection, timeout, cancellation, and recovery guidance.
- Operation State: Ephemeral state held only for an ongoing library-controlled operation, such as a pairing workflow, unpairing workflow, or connection object configuration. This is not a device cache or registry.
- Timeout Policy: A communication timeout model with one global default timeout and optional command-level overrides.
- Integrator Documentation: Usage guidance and examples that define how consuming Go applications should list devices, connect, authorize, subscribe to events, read/write configuration, send commands, configure timeouts, and handle errors.

#### Key Business Rules

- Cloud independence: Core library behavior must not depend on the mobile app or managed cloud API. The TimeFlip2 device and local BLE access are the source of truth.
- Protocol abstraction: Integrators should not need to understand BLE services, characteristics, command bytes, or notification mechanics to use the primary library API.
- Business logic boundary: The library may surface events and state, but it must not decide what task is active, how time is billed, or what a consuming application should do with an orientation or tap event.
- Authorization before protected operations: The protocol documentation states that the device requires password authorization after reconnect and may return no data or reject commands when the password is missing or wrong.
- No library-owned storage or caching: The library must not store device state in files, memory caches, registries, or other persistent/retained structures. Only ephemeral state for an active operation or connection object is allowed.
- Device reset volatility: Protocol documentation states that some values are reset when the battery is removed or replaced. Since the library must not cache device state, consuming applications are responsible for any retained state and its invalidation.
- Pairing/unpairing must expose stages and recoverable errors: The acceptance criteria require stage visibility and recovery where possible, rather than a single opaque success/failure result.
- OS pairing/unpairing is best-effort by API support: Where MacOS APIs cannot directly perform a pairing or unpairing action, the library should return the inputs or instructions needed for caller- or user-initiated action.
- Operation sensitivity belongs to consumers: The library should not classify reads or writes into different sensitivity levels. Consuming applications own policy decisions around whether to expose or permit particular operations.
- Timeout behavior is intentionally simple: There is one global communication timeout, with optional per-command overrides for operations that need a different bound.
- Event delivery must be asynchronous and cancellable: Consuming applications need events through a Go channel, and the library must account for disconnects, blocked consumers, timeout/cancellation, and clean shutdown.
- Firmware update remains out of scope: The protocol includes firmware-loader related behavior, but the requirement explicitly reserves firmware updates for future work.
- MacOS is the initial target, with portability preserved: Platform choices should solve the MacOS BLE need now while keeping public concepts portable enough for later Linux and Windows work.

## Strategic Approach

#### Solution Direction

- Build a small Go module around a clear library API, with MacOS BLE support as the first transport implementation and a protocol adapter that maps TimeFlip2 BLE behavior into device, session, command, state, and event concepts.
- Treat the current repository as greenfield: no `go.mod`, source code, or implemented domain types exist yet. Existing assets are requirements, SPDD templates, hardware protocol docs, and an Android app archive for possible later reference.
- Keep the primary data flow simple: consuming app discovers devices, starts pairing or opens a device session using caller-supplied inputs, authorizes as required, performs reads/writes/commands, and subscribes to an event stream backed by BLE notifications and controlled reads.
- Design the public package around conceptual operations and typed outcomes, not BLE characteristic mechanics. Lower-level protocol details can remain available internally and, where useful, through diagnostic or advanced APIs.
- Keep the library stateless outside active operations and connection objects. Any remembered devices, passwords, labels, task meanings, or last-known device values belong to consuming applications, not the library.
- Use documentation and tests as first-class deliverables because physical BLE hardware and OS behavior are hard to verify deterministically in ordinary CI.

#### Key Design Decisions

- Public API level: A high-level device/session API is easier for integrators and matches the requirement to mask protocol and communication medium, while a raw BLE-style API would be simpler initially but would leak device internals. Recommendation: expose high-level concepts first, with a contained protocol layer underneath.
- BLE portability boundary: MacOS can be implemented first, but binding the whole library to one MacOS-specific BLE package would make later Linux and Windows support expensive. Recommendation: define a transport boundary early and keep MacOS as the first adapter.
- Pairing meaning: Pairing now covers all aspects of bringing a new or reset device into usable state, while OS-level pairing is limited by MacOS API support and TimeFlip-specific authorization remains part of the device workflow. Recommendation: model pairing as a staged workflow that can either perform supported actions or return required user/caller action inputs when direct OS support is unavailable.
- Unpairing meaning: Unpairing now covers device-specific reset, such as password removal or reset behavior where supported, plus OS unpairing where MacOS APIs allow it. Recommendation: model unpairing as a staged workflow that reports device-side completion separately from OS-side completion or required manual action.
- State storage: Caching or registries would make offline unpairing and reconnection easier, but the clarification explicitly prohibits library-owned storage or caching. Recommendation: keep all remembered device state caller-owned and allow only ephemeral state inside active workflows and connection objects.
- Event representation: A single generic byte/event feed preserves protocol fidelity but burdens integrators; highly interpreted business events would violate scope-out. Recommendation: use typed technical events such as facet changed, double tap, battery changed, system state changed, history entry, connection state, and command result, while leaving task interpretation to applications.
- Command coverage: Supporting every documented command immediately may expand scope, but the clarification says the library should not make sensitivity-level distinctions between reads and writes. Recommendation: expose supported non-firmware-update operations consistently and leave operation policy to consumers, while keeping firmware update behavior out of scope.
- Timeout shape: Multiple timeout categories could provide precision but would expand the API surface. Recommendation: use one global communication timeout with optional per-command overrides, as clarified.
- Test strategy: Real BLE hardware is needed for end-to-end confidence, but the protocol adapter and state/session behavior can be exercised without hardware. Recommendation: make protocol parsing and session orchestration testable independently from the MacOS BLE adapter, then provide a hardware smoke-test path for integrators.
- Documentation scope: Minimal examples would be quick, but this library's value depends on clear integration behavior around pairing, authorization, channels, errors, and lifecycle. Recommendation: include integration documentation as part of the initial deliverable, not as a later polish task.

#### Alternatives Considered

- Clone mobile-app/cloud behavior: Rejected because the decision driver is independence from mobile devices and cloud APIs.
- Build a command-line application first: Rejected as the primary deliverable because the requirement is a reusable Go library; a small example CLI can still be useful as documentation and hardware validation.
- Expose only raw BLE primitives: Rejected because it fails the requirement to mask the protocol and communication medium from integrators.
- Implement cross-platform BLE immediately: Rejected for initial scope because MacOS is the target and Linux/Windows are explicitly reserved, though the architecture should preserve portability.
- Interpret task/time-tracking business behavior inside the library: Rejected because the requirement excludes business logic interpreting events.
- Maintain a library-owned device registry or cache: Rejected because the clarification explicitly prohibits storage or caching of device state by the library itself.

## Risk & Gap Analysis

- Requirement ambiguity resolved: Pairing means all aspects of pairing a new or reset device, including TimeFlip-specific setup/authorization and OS-level pairing where directly supportable by OS APIs.
- Requirement ambiguity resolved: Unpairing means device-specific reset required, such as removing password where supported, plus OS unpairing where directly supportable by OS APIs.
- Requirement ambiguity: "Supported devices in range" needs a discovery rule. The protocol docs describe services and characteristics, but the requirement does not define whether matching should use advertised name, service UUID, manufacturer data, or post-connect service discovery.
- Requirement ambiguity: "Reading any available stored data/configuration" is broad. The protocol docs describe identity, battery, facet, command result, system state, and history, but not every value has identical read/notify/write behavior.
- Requirement clarification: The library should not distinguish operation sensitivity levels for reading or writing. Any consumer-facing policy around risky operations belongs to consuming applications.
- Requirement ambiguity: "Sending any supported commands" conflicts with firmware update being out of scope because the protocol includes firmware-loader-related behavior. The REASONS Canvas should define a supported-command policy.
- Requirement ambiguity resolved: The library must not store or cache device state in files, memory, cache, or other retained structures. Only active workflow state and connection object configuration are allowed.
- Requirement ambiguity resolved: Timeout configuration consists of one global communication timeout plus optional per-command overrides.
- Edge case: Device password may be wrong, reset to default after battery removal, changed by another client, or required after every reconnect.
- Edge case: Device may reset or lose RAM-backed state after battery removal. Because the library does not cache state, any stale retained state belongs to the consuming application.
- Edge case: Multiple TimeFlip2 devices may be in range with similar names, changing signal strength, or incomplete advertisements.
- Edge case: OS APIs may not directly support every desired BLE pairing or unpairing action. The library still needs to surface actionable inputs or instructions for caller/user action.
- Edge case: Unpairing a device that is not in range may limit device-specific reset behavior, leaving only OS-level or caller-owned cleanup possible where supported.
- Edge case: The device may disconnect mid-command, mid-history read, or while events are being streamed.
- Edge case: Consuming applications may stop reading from the event channel, cancel the context, or request writes while notifications are active.
- Edge case: History retrieval may produce partial streams, zero terminators, pause-encoded sides, undefined sides, accelerometer/flash errors, and duplicate or out-of-order entries after reconnect.
- Edge case: System state can report synchronization requirements or hardware errors, which may affect whether reads/writes should proceed.
- Technical risk: MacOS BLE access from Go depends on library support, OS permissions, Bluetooth availability, and sometimes event-loop constraints. This is the main platform risk.
- Technical risk: The repository has no Go module or implementation conventions yet, so package layout, dependency choices, testing strategy, linting, and examples must be established carefully in the next phase.
- Technical risk: Physical hardware testing is likely needed for confidence, but automated CI may only be able to cover protocol parsing and mocked transport behavior.
- Technical risk: The Android app archive may contain useful behavior, but reverse engineering it is not needed for the first strategic pass and could add legal/process complexity if relied on heavily.
- Technical risk: Avoiding all library-owned storage simplifies privacy and lifecycle responsibilities, but it requires consuming applications to provide any remembered identifiers, passwords, labels, or prior state needed across sessions.
- Acceptance Criteria coverage: AC1 is addressable through BLE discovery and TimeFlip2 identification, but the matching criteria need definition.
- Acceptance Criteria coverage: AC2 is addressable through a staged pairing workflow for new or reset devices, including TimeFlip-specific authorization/setup and OS-level pairing where APIs support it.
- Acceptance Criteria coverage: AC3 is addressable through device-specific reset where the device is reachable and OS unpairing where APIs support it; otherwise the library should return required inputs or user-action guidance.
- Acceptance Criteria coverage: AC4 is addressable through a read/state API backed by documented services and characteristics, but the exact supported data set should be enumerated.
- Acceptance Criteria coverage: AC5 is addressable through write/configuration and command flows, with operation policy left to consuming applications rather than sensitivity classification inside the library.
- Acceptance Criteria coverage: AC6 is addressable with a channel-backed event stream, but buffering, cancellation, reconnect, and backpressure behavior need definition.
- Acceptance Criteria coverage: AC7 is addressable by including integration documentation and examples in the initial deliverable.
