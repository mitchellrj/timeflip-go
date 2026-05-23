package main

import (
	"bytes"
	"testing"
)

func TestTerminalPrompterFallsBackForNonTTY(t *testing.T) {
	input := bytes.NewBufferString("list\n")
	output := &bytes.Buffer{}
	prompter := NewTerminalPrompter(input, output)
	got, err := prompter.Prompt("timeflip> ")
	if err != nil {
		t.Fatal(err)
	}
	if got != "list" {
		t.Fatalf("Prompt()=%q want list", got)
	}
	if prompter.editor != nil {
		t.Fatal("non-TTY prompter should not enable line editor")
	}
}
