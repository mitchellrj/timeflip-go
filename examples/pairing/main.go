package main

import (
	"context"
	"fmt"
	"os"
	"time"

	timeflip "github.com/mitchellrj/timeflip-go"
	"github.com/mitchellrj/timeflip-go/macos"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: pairing DEVICE_ID [CURRENT_PASSWORD]")
		os.Exit(2)
	}
	var password string
	if len(os.Args) > 2 {
		password = os.Args[2]
	}
	client, err := timeflip.NewClient(macos.NewTransport(), timeflip.Config{CommunicationTimeout: 10 * time.Second})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	result, err := client.Pair(context.Background(), timeflip.PairRequest{
		DeviceID:       timeflip.DeviceID(os.Args[1]),
		Password:       password,
		AllowOSPairing: true,
	})
	fmt.Printf("completed=%v stage=%s\n", result.Completed, result.Stage)
	if result.ManualAction != nil {
		fmt.Printf("manual action: %s\n", result.ManualAction.Description)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
