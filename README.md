# timeflip-go

`timeflip-go` is a small Go library for integrating with TimeFlip2 devices over BLE without depending on the mobile app or cloud API.

The library is intentionally stateless: it does not store device IDs, passwords, task labels, cached device values, or event history. Consuming applications own any persistence they need. Active pairing, unpairing, and connection objects may keep only the in-progress state needed to complete the current operation.

## Current Status

The core API, protocol model, workflow orchestration, fake transport, examples, and tests are in place. The `macos` package currently provides the adapter boundary and manual-action behavior for OS pairing/unpairing; a concrete CoreBluetooth-backed implementation can fill in scan/connect/read/write/subscribe behavior behind the same transport interface.

## Basic Usage

```go
client, err := timeflip.NewClient(transport, timeflip.Config{
    CommunicationTimeout: 10 * time.Second,
})
devices, err := client.ListDevices(ctx, timeflip.ScanFilter{})
session, err := client.Connect(ctx, timeflip.ConnectRequest{DeviceID: devices[0].ID})
_, err = session.Authorize(ctx, "000000")
events, errs, err := session.Events(ctx, timeflip.EventOptions{Buffer: 16})
```

## Pairing

Pairing is a staged workflow for new or reset devices. It can include TimeFlip password authorization, optional password changes, verification reads, and OS-level pairing where the active OS adapter directly supports it. If OS pairing is not directly supported, the result can include a `ManualAction` with the device ID and instructions for caller- or user-initiated action.

## Unpairing

Unpairing is also staged. When the device is reachable and a password is supplied, callers can request device-side reset behavior such as factory reset. OS unpairing is attempted only where the adapter supports it; otherwise the library returns a manual action.

## Timeouts

The client has one global communication timeout. Commands may provide a `CommandOptions.Timeout` override for that command only.

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

Inside the prompt, use `help` to see commands. A typical smoke-test path is:

```text
list
select DEVICE_ID
pair
connect
authorize
read info
read battery
stream
stop
close
unpair
exit
```

The demo also exposes `read system`, `read history`, `read task FACET`, `read tap`, writable configuration through `write ...`, and reset commands through `command ...`. Destructive operations such as password changes, task reset, factory reset, and unpairing ask for confirmation.

The current `macos` package is still an adapter boundary: it may report unsupported scan/connect behavior until a concrete CoreBluetooth-backed implementation is added. Those unsupported results are surfaced by the demo as adapter limitations, not hidden as generic failures.

## Examples

- `examples/basic`: create a client and list devices.
- `examples/pairing`: run pairing and print staged/manual-action results.
