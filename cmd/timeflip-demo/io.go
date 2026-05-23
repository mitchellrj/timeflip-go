package main

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

type InputPrompter interface {
	Prompt(label string) (string, error)
	PromptSecret(label string) (string, error)
	Confirm(label string) (bool, error)
}

type TerminalPrompter struct {
	reader *bufio.Reader
	out    io.Writer
}

func NewTerminalPrompter(in io.Reader, out io.Writer) *TerminalPrompter {
	return &TerminalPrompter{reader: bufio.NewReader(in), out: out}
}

func (p *TerminalPrompter) Prompt(label string) (string, error) {
	if label != "" {
		if _, err := fmt.Fprint(p.out, label); err != nil {
			return "", err
		}
	}
	line, err := p.reader.ReadString('\n')
	if err != nil {
		if err == io.EOF && line != "" {
			return strings.TrimSpace(line), nil
		}
		return "", err
	}
	return strings.TrimSpace(line), nil
}

func (p *TerminalPrompter) PromptSecret(label string) (string, error) {
	// Standard-library only: this intentionally uses the same reader path as
	// Prompt. The help text warns that terminal echo is not disabled here.
	return p.Prompt(label)
}

func (p *TerminalPrompter) Confirm(label string) (bool, error) {
	for {
		answer, err := p.Prompt(label + " [y/N]: ")
		if err != nil {
			return false, err
		}
		switch strings.ToLower(strings.TrimSpace(answer)) {
		case "y", "yes":
			return true, nil
		case "", "n", "no":
			return false, nil
		default:
			if _, err := fmt.Fprintln(p.out, "please answer yes or no"); err != nil {
				return false, err
			}
		}
	}
}
