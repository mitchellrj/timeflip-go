package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
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
	editor *lineEditor
}

func NewTerminalPrompter(in io.Reader, out io.Writer) *TerminalPrompter {
	p := &TerminalPrompter{reader: bufio.NewReader(in), out: out}
	inFile, inOK := in.(*os.File)
	outFile, outOK := out.(*os.File)
	if inOK && outOK && isTerminalFile(inFile) && isTerminalFile(outFile) {
		p.editor = newLineEditor(inFile, outFile)
	}
	return p
}

func (p *TerminalPrompter) Prompt(label string) (string, error) {
	if p.editor != nil {
		return p.editor.ReadLine(label, label == "timeflip> ", true)
	}
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
	if p.editor != nil {
		return p.editor.ReadLine(label, false, false)
	}
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
