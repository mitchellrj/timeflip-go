# timeflip-go

[![CI](https://github.com/mitchellrj/timeflip-go/actions/workflows/ci.yml/badge.svg)](https://github.com/mitchellrj/timeflip-go/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/mitchellrj/timeflip-go)](https://goreportcard.com/report/github.com/mitchellrj/timeflip-go)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/mitchellrj/timeflip-go/badge)](https://scorecard.dev/viewer/?uri=github.com/mitchellrj/timeflip-go)
[![SLSA provenance](https://img.shields.io/badge/SLSA%20provenance-L3-blue)](https://github.com/mitchellrj/timeflip-go/actions/workflows/release.yml)

`timeflip-go` is a small Go library for integrating with TimeFlip2 devices over BLE without depending on the mobile app or cloud API.

The library is intentionally stateless: it does not store device IDs, passwords, task labels, cached device values, or event history. Consuming applications own any persistence they need. Active pairing, unpairing, and connection objects may keep only the in-progress state needed to complete the current operation.

## Current Status

The core API, protocol model, workflow orchestration, fake transport, examples, and tests are in place. The `macos` package provides the first concrete BLE transport using CoreBluetooth through `tinygo.org/x/bluetooth`, so MacOS callers can scan, connect, read, write, and subscribe through the same platform-neutral transport interface.

MacOS may ask for Bluetooth permission the first time an app or terminal process uses the adapter. If access is denied, enable Bluetooth access for the calling app in System Settings and retry. The adapter keeps discovered peripheral lookup only in the active process memory and does not persist device IDs, passwords, payloads, or authorization state.

## Basic Usage

```go
client, err := timeflip.NewClient(transport, timeflip.Config{
    CommunicationTimeout: 10 * time.Second,
})
devices, err := client.ListDevices(ctx, timeflip.ScanFilter{})
session, err := client.Connect(ctx, timeflip.ConnectRequest{
    DeviceID:       devices[0].ID,
    AdvertisedName: devices[0].Name,
})
_, err = session.Authorize(ctx, "")
events, errs, err := session.Events(ctx, timeflip.EventOptions{Buffer: 16})
```

## Pairing

Pairing is a staged workflow for new or reset devices. TimeFlip2 always requires password authorization before device commands; the factory default is ASCII `000000`, encoded as bytes `0x30 0x30 0x30 0x30 0x30 0x30`. Passing an empty password to `Authorize`, `Pair`, or device-side unpair/reset flows uses that default. Pairing can include TimeFlip password authorization, optional password changes, verification reads, and OS-level pairing where the active OS adapter directly supports it. If OS pairing is not directly supported, the result can include a `ManualAction` with the device ID and instructions for caller- or user-initiated action.

## Security Notes

The library assumes the consuming application is trusted, but BLE advertisements and device payloads are untrusted. Device support detection can use advertised names or metadata as a convenience heuristic; this is not authenticated identity. Applications that need stronger identity continuity should remember and reconnect by the stable device ID/address reported by their transport.

The TimeFlip password is written as six bytes to the device password characteristic. Treat passwords as sensitive, avoid logging them, and use an empty password only when intentionally authorizing with the factory default `000000` for a new or reset device. The library does not persist passwords, device IDs, authorization state, event history, or payloads across sessions.

Protocol errors redact raw payload bytes from default error strings. Trusted diagnostic code can inspect `ProtocolPayloadError.Payload` when full bytes are needed for hardware troubleshooting. Event raw bytes for typed events are emitted only when `EventOptions.IncludeRaw` is true; raw/unknown events still carry their raw payload as the event payload because that is the event content.

## Unpairing

Unpairing is also staged. When device-side reset behavior such as factory reset is requested, the client authorizes with the supplied password or the default password when the request password is empty. OS unpairing is attempted only where the adapter supports it; otherwise the library returns a manual action.

## Timeouts

The client has one global communication timeout. Commands may provide a `CommandOptions.Timeout` override for that command only.

## Command Responses

TimeFlip2 command acknowledgement is read back from the command characteristic after a command write. The value is NUL-terminated; after trimming at the first NUL byte, only the first four bytes are command acknowledgement data and any remaining bytes are ignored. The acknowledgement begins as `0xXXYY`, where `XX` is the command and `YY` is `0x02` for executed or `0x01` for error. Command output data, when a command has output, is read separately from the command-result output characteristic.

## Protocol Versions

The hardware documents in `docs/Hardware` describe two protocol families. The v3-style protocol uses the command-result output characteristic for history packages and packs seven 3-byte history blocks into each 21-byte package. The v4-style protocol adds TimeFlip events, system state, and the separate history data characteristic. The client supports an explicit protocol version in `Config` or `ConnectRequest`; automatic mode uses firmware guidance where available (`FW_v3.47` and newer behaves as v4) and otherwise prefers v4 behavior with a v3 fallback where practical.

For v3 devices, the friendly device name is not exposed through the documented readable characteristics. `ReadDeviceInfo` refreshes the current broadcast name through transport-supported advertised-name lookup or BLE discovery when available, with `ConnectRequest.AdvertisedName` as a fallback. If the device omits the Generic Access Device Name characteristic before the protocol version is known, the advertised name is still used so device info does not show a blank name.

## Events

Events are technical device events delivered through Go channels. The library does not interpret a facet as a task, stop/start time tracking, or perform business decisions for consuming applications.

## Interactive Demo

Run the demo CLI with:

```sh
go run ./cmd/timeflip-demo
```

Useful startup flags:

- `-timeout 10s`: global communication timeout.
- `-command-timeout 5s`: per-command timeout override.
- `-event-buffer 16`: event channel buffer size.
- `-include-raw`: print raw event bytes while streaming.
- `-include-unsupported`: include unsupported BLE devices in scan output.
- `-no-color`: disable ANSI color output.
- `-trace-ble PATH`: write raw BLE operation logs for the CLI session. Use `-trace-ble -` for stderr. The trace includes password characteristic bytes.

Inside the prompt, use `help` to see commands. A typical smoke-test path is:

```text
list
select DEVICE_ID
pair
connect
authorize
read info
read battery
read status
stream
stop
close
unpair
exit
```

When running in a supported terminal, the demo keeps in-process command history for the main prompt and supports up/down arrows. Output uses ANSI colors when stdout is a TTY; use `-no-color` or `NO_COLOR=1` to disable color.

The demo also exposes `read system`, `read history`, `read task FACET`, `read tap`, writable configuration through `write ...`, and reset commands through `command ...`. Use `read status` to inspect current lock, pause, auto-pause, and current facet state; pause is not guaranteed to appear as a streaming event. Destructive operations such as password changes, task reset, factory reset, and unpairing ask for confirmation.

On MacOS, the demo uses the real CoreBluetooth-backed adapter. On other platforms, the `macos` package still compiles but reports unsupported scan/connect behavior. OS-level pairing and unpairing may still require manual action in macOS Bluetooth settings; the library reports those actions explicitly instead of claiming direct OS changes were performed.

## Examples

- `examples/basic`: create a client and list devices.
- `examples/pairing`: run pairing and print staged/manual-action results.

## Development Checks

Install and enable pre-commit hooks before contributing:

```sh
pre-commit install
pre-commit run --all-files
```

The local hooks check Go formatting, module tidiness, `go vet`, tests, and the same pinned `golangci-lint` version used in CI.

GitHub Actions runs the same Go quality script on Ubuntu and macOS, runs race tests on Ubuntu, uploads coverage, runs `golangci-lint`, and runs `govulncheck` for dependency and standard-library vulnerability checks.

Dependabot checks Go modules, GitHub Actions, and pre-commit hook revisions weekly.

The root package includes a native Go fuzz target for protocol payload decoding. Seed corpus entries run with normal `go test`; run an active local fuzzing session with:

```sh
go test -run='^$' -fuzz=FuzzDecodeProtocolPayloads -fuzztime=30s .
```

CI runs a short fuzz-smoke pass on Ubuntu.

## Releases and Provenance

Releases are published from SemVer tags with a leading `v`:

```sh
git tag -a vX.Y.Z -m "vX.Y.Z"
git push origin vX.Y.Z
```

Prerelease tags such as `vX.Y.Z-rc.1` are published as GitHub prereleases. The release workflow rejects malformed tags before publishing any artifacts.

Release artifacts currently include Linux `timeflip-demo` binaries for `amd64` and `arm64`, SHA256 checksums, and SLSA provenance files generated by the official SLSA GitHub Go builder. Darwin binaries are not published by the trusted release workflow yet because the CoreBluetooth dependency requires cgo and is not reliable in the cross-platform SLSA builder path; macOS users can still build from the tagged source with `go build ./cmd/timeflip-demo`.

Verify a release artifact with `slsa-verifier`:

```sh
slsa-verifier verify-artifact ./timeflip-demo-linux-amd64 \
  --provenance-path ./timeflip-demo-linux-amd64.intoto.jsonl \
  --source-uri github.com/mitchellrj/timeflip-go \
  --source-tag vX.Y.Z
```

The provenance verifies artifact origin and build metadata. Consumers should still check the release tag, checksums, and their own deployment requirements.
