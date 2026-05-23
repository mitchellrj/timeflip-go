//go:build !darwin

package main

import "os"

func isTerminalFile(*os.File) bool {
	return false
}

func makeRawTerminal(*os.File) (func(), error) {
	return func() {}, nil
}
