# TimeFlip Go Library Complexity Cleanup Assessment

## Scope

This assessment covers cleanup candidates in the reusable library and macOS transport after the security hardening work. The goal is to remove proven duplication, unused complexity, and premature optimization while preserving fail-closed authorization, strict macOS identity matching by default, redacted protocol errors, raw event opt-in behavior, and bounded remote-controlled loops.

## Findings

| ID | Category | Component | Evidence | Decision | Risk |
| --- | --- | --- | --- | --- | --- |
| CLN-001 | Accepted complexity | `session.go` authorization timing variables | `authorizationResultWindow` and `authorizationPollInterval` are used by production code and tests. Wrong-password behavior intentionally waits for stale `0x01` to become fresh `0x02`; deterministic tests need a shorter window to avoid 750 ms sleeps. | Keep the mutable authorization timing variables and test helper. They are testability seams for a real timing behavior, not premature optimization. | Removing them would either slow tests or force tests to assert timeout behavior instead of authorization failure. |
| CLN-002 | Accepted complexity | `session.go` command poll interval | `commandPollInterval` is production behavior and is shortened in budget tests. The command status/output budget constants must remain high enough for real devices. | Keep the mutable poll interval and test helper. | Removing it would make two poll-budget tests wait roughly 12.8 seconds each or require lowering production budgets. |
| CLN-003 | Remove unused compatibility | `macos.Transport.AllowNameFallback` | `rg` shows references only in `macos/transport_darwin.go` and its tests. The option was newly introduced as a compatibility escape hatch but no caller uses it, and name fallback conflicts with strict identity hardening. | Remove `AllowNameFallback`; keep exact ID matching only. | Existing uncommitted callers could no longer opt into name matching, but the field is recent and not part of the core transport interface. |
| CLN-004 | Simplify duplication | `session.go` event raw redaction | `decodeNotification` repeats raw-clearing branches for typed payloads and uses `redactTypedEventRaw` for text events. | Remove `redactTypedEventRaw` by passing raw-inclusion intent into `decodeTimeFlipTextEvent`, so text-event payloads are constructed without raw bytes when raw is not requested. Keep direct raw clearing for typed byte decoders because it is explicit and small. | Over-generalizing this path would obscure event-specific behavior; keep the cleanup local. |
| CLN-005 | Accepted purposeful duplication | `readCommandStatusFor` and `readCommandOutputFor` | Both loops have budgets, timers, and last-payload tracking, but status decoding and output acceptance/password failure rules differ. | Keep separate loops. A shared helper would require callback-heavy control flow and reduce protocol clarity. | Purposeful duplication remains, but each loop stays easy to audit for security behavior. |
| CLN-006 | Remove public helper | `ProtocolPayloadError.RawPayloadHex()` | `rg` shows references only in README and tests. Trusted callers can inspect `Payload` directly and format it however they need. | Remove `RawPayloadHex()` and update docs/tests to use `Payload`. | Minor convenience loss for a newly introduced helper; default error redaction remains intact. |

## Planned Changes

1. Remove `AllowNameFallback` and simplify macOS connect candidate matching to exact `peripheral.ID == requestedID`.
2. Simplify text-event raw redaction by constructing promoted text events with raw bytes only when `IncludeRaw` is true.
3. Remove `ProtocolPayloadError.RawPayloadHex()` and update README/tests to reference `Payload` directly.
4. Keep timing variables and separate command polling loops with this assessment as rationale.
5. Trim tests that only covered removed compatibility/helper behavior while preserving security regression coverage.

## Rejected Simplifications

- Do not convert authorization and polling interval variables to constants in this pass. The variables are used only by tests outside normal production mutation, but removing them would make behavior tests slower or less representative.
- Do not consolidate command polling loops. The duplicate shape is smaller than the abstraction required to share it safely.
- Do not remove raw byte cloning. It is boundary protection, not waste.
