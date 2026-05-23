package main

import (
	"context"
	"fmt"
	"os"
	"time"

	timeflip "timeflip-go"
	"timeflip-go/macos"
)

func main() {
	client, err := timeflip.NewClient(macos.NewTransport(), timeflip.Config{CommunicationTimeout: 10 * time.Second})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	devices, err := client.ListDevices(context.Background(), timeflip.ScanFilter{})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for _, device := range devices {
		fmt.Printf("%s %s supported=%v rssi=%d\n", device.ID, device.Name, device.Supported, device.RSSI)
	}
}
