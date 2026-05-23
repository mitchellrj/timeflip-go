//go:build darwin

package main

import (
	"os"

	"golang.org/x/sys/unix"
)

func isTerminalFile(file *os.File) bool {
	if file == nil {
		return false
	}
	_, err := unix.IoctlGetTermios(int(file.Fd()), unix.TIOCGETA)
	return err == nil
}

func makeRawTerminal(file *os.File) (func(), error) {
	fd := int(file.Fd())
	oldState, err := unix.IoctlGetTermios(fd, unix.TIOCGETA)
	if err != nil {
		return func() {}, err
	}
	newState := *oldState
	newState.Iflag &^= unix.BRKINT | unix.ICRNL | unix.INPCK | unix.ISTRIP | unix.IXON
	newState.Oflag &^= unix.OPOST
	newState.Cflag |= unix.CS8
	newState.Lflag &^= unix.ECHO | unix.ICANON | unix.IEXTEN | unix.ISIG
	newState.Cc[unix.VMIN] = 1
	newState.Cc[unix.VTIME] = 0
	if err := unix.IoctlSetTermios(fd, unix.TIOCSETA, &newState); err != nil {
		return func() {}, err
	}
	return func() {
		_ = unix.IoctlSetTermios(fd, unix.TIOCSETA, oldState)
	}, nil
}
