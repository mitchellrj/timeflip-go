# TimeFlip Go Library Security Red-Team Analysis

## Scope and Assumptions

This assessment covers the reusable `timeflip` library, `internal/protocol`, `internal/transport`, and `macos` transport packages. The demo CLI under `cmd/timeflip-demo/**` and examples under `examples/**` are out of scope.

The consuming application and its callers are trusted. Nearby BLE peripherals, advertisements, GATT characteristic values, command responses, history packets, and notifications are untrusted and may be spoofed or malicious.

## Assets

- TimeFlip password bytes written to the password characteristic.
- Device identity binding between caller-selected device IDs and BLE peripherals.
- Command intent, especially reset, password, task, LED, pause, lock, and factory reset commands.
- History and event payload confidentiality when errors or logs are collected by a trusted app.
- Process availability under malformed or non-terminating peripheral responses.
- Diagnostic clarity without exposing more raw device data than needed by default.

## Attack Surface

- `Client.ListDevices` and `IsSupportedPeripheral` accept untrusted advertisement names, service UUIDs, and metadata.
- `macos.Transport.Connect` maps caller-selected IDs to CoreBluetooth scan results.
- `Session.Authorize`, `Pair`, and `Unpair` write password bytes and interpret command-result payloads.
- `Session.SendCommand`, `readCommandStatusFor`, `readCommandOutputFor`, and history readers loop over peripheral-controlled values.
- `Session.Events` exposes notification payloads through typed event values.
- `OperationError` and `ProtocolPayloadError` may be logged by consuming applications.

## Findings

| ID | Severity | Component | Evidence | Attack Path | Impact | Recommendation | Tests |
| --- | --- | --- | --- | --- | --- | --- | --- |
| SEC-001 | Medium | `protocol.go:IsSupportedPeripheral` | A peripheral is supported when the advertised name contains `timeflip` or metadata model contains `timeflip`, even without the TimeFlip service UUID. | A nearby attacker advertises `TIMEFLIP2` or matching model metadata. | Trusted apps may present spoofed devices as plausible TimeFlip devices. | Document this as heuristic support detection and avoid treating it as authenticated identity. Prefer exact service UUIDs where callers need stronger confidence. | `TestIsSupportedPeripheral` documents name/model fallback as heuristic. |
| SEC-002 | High | `macos/transport_darwin.go:Connect` | When a device is not remembered, connect matching previously accepted `peripheral.ID == id` or `peripheral.Name == id`. | An attacker advertises the requested name before the intended device is found. | The library can connect to the wrong peripheral and send password or commands to it. | Use strict address matching by default; keep explicit compatibility fallback only when opted in by the macOS transport. | `TestMatchesConnectCandidate` covers exact ID, strict name rejection, and explicit name fallback. |
| SEC-003 | High | `session.go:readAuthorizationResult` | Empty, unknown, and read-error command-result payloads were previously treated as successful authorization. | A malicious peripheral returns empty/malformed data or closes/blocks the read after password write. | Trusted apps can believe authorization succeeded and proceed with sensitive commands. | Fail closed on empty/malformed/read-error authorization results; only explicit `0x02` succeeds. Preserve stale `0x01` handling only when followed by fresh success. | Authorization tests cover empty, malformed, read error, stale wrong then success, and persistent wrong result. |
| SEC-004 | Low | `types.go:DefaultPassword`, `session.go:passwordOrDefault` | Empty passwords map to factory default `000000`. | Integrators may accidentally authorize new/reset devices with a weak default. | Normalizes default-password operation if not clearly documented. | Document default use as explicit caller intent for new/reset devices; do not store passwords. | README documents six-byte plaintext writes and default-password behavior. |
| SEC-005 | Medium | `errors.go:ProtocolPayloadError.Error` | Default error strings previously appended full `raw=0x...` payload bytes. | A trusted app logs `err.Error()` from malformed history or event data. | Raw history, event, device info, or command bytes can leak to application logs. | Keep `Payload` for programmatic inspection but redact default error strings and add an explicit helper for trusted diagnostics. | Protocol payload error tests assert redacted default strings and preserved payload access. |
| SEC-006 | Medium | `session.go:decodeNotification` | TimeFlip text events populated `Event.Raw` and typed payload raw fields even when `EventOptions.IncludeRaw` was false. | A peripheral sends sensitive bytes through notification paths and the trusted app logs event values. | Raw notification bytes appear without opt-in. | Gate typed event raw fields and `Event.Raw` on `IncludeRaw`; keep raw bytes as `EventRaw.Payload` when the event kind itself is raw. | Event tests assert default typed events omit raw bytes and `IncludeRaw` preserves them. |
| SEC-007 | Medium | `session.go:readHistoryV3`, `readCommandStatusFor`, `readCommandOutputFor` | Loops are context-timeout bounded but had no explicit packet or poll budget. | A malicious peripheral repeatedly emits plausible non-terminal or mismatched payloads under a long timeout. | Increased CPU/time use and memory growth, especially for history. | Add internal poll and packet budgets that return operation-scoped errors before unbounded growth. | Tests simulate endless mismatched status/output and non-terminal v3 history packets. |
| SEC-008 | Low | `session.go:Authorize`, `SetPassword` | Passwords are Go strings copied to byte slices before writes. | Local process memory inspection may recover password strings. | Defense-in-depth limitation; trusted app assumption makes active local compromise out of scope. | Document limitation; avoid extra copies and avoid logging. Do not add persistent password storage. | README documents password handling limits. |

## Non-Findings and Accepted Risks

- The library does not defend against malicious consuming applications. This is accepted because the consuming application and its callers are trusted by scope.
- The library does not provide cryptographic authentication of the TimeFlip2 device. This appears to be a device protocol limitation rather than a missing library feature.
- The library remains stateless and does not persist known-device identity, password, history, or authorization state. This avoids storage risk but means applications that need stronger identity continuity must implement it themselves.
- Raw byte fields remain available on public result structs for trusted diagnostics and hardware troubleshooting; the mitigation is to redact default string/log paths and require explicit event raw opt-in.

## Hardening Summary

- Make macOS connect matching strict by default and explicit when name fallback is required.
- Treat authorization ambiguity as failure.
- Redact default protocol error strings while keeping structured payload access.
- Gate typed event raw bytes behind `EventOptions.IncludeRaw`.
- Add read-loop budgets for command polling and v3 history streams.
- Clarify password and BLE spoofing limitations in documentation.
