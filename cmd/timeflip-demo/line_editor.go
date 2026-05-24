package main

import (
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"
)

type lineEditor struct {
	in      *os.File
	out     *os.File
	history []string
}

func newLineEditor(in *os.File, out *os.File) *lineEditor {
	return &lineEditor{in: in, out: out}
}

func (e *lineEditor) ReadLine(prompt string, addHistory bool, echo bool) (string, error) {
	restore, err := makeRawTerminal(e.in)
	if err != nil {
		return "", err
	}
	defer restore()

	if _, err := fmt.Fprint(e.out, prompt); err != nil {
		return "", err
	}

	var line []rune
	pos := 0
	historyIndex := len(e.history)
	var pending []rune
	buf := make([]byte, 1)
	for {
		n, err := e.in.Read(buf)
		if err != nil {
			return "", err
		}
		if n == 0 {
			continue
		}
		switch b := buf[0]; b {
		case 3, 4:
			if len(line) == 0 {
				if _, err := fmt.Fprint(e.out, "\r\n"); err != nil {
					return "", err
				}
				return "", io.EOF
			}
		case '\r', '\n':
			if _, err := fmt.Fprint(e.out, "\r\n"); err != nil {
				return "", err
			}
			value := strings.TrimSpace(string(line))
			if addHistory && value != "" && (len(e.history) == 0 || e.history[len(e.history)-1] != value) {
				e.history = append(e.history, value)
				if len(e.history) > 100 {
					e.history = e.history[len(e.history)-100:]
				}
			}
			return value, nil
		case 127, 8:
			if pos > 0 {
				line = append(line[:pos-1], line[pos:]...)
				pos--
				e.redraw(prompt, line, pos, echo)
			}
		case 27:
			if e.readEscape(&line, &pos, &historyIndex, &pending, prompt, echo) {
				continue
			}
		default:
			r := rune(b)
			if r >= 0x80 {
				// Keep the editor deliberately conservative; command input is
				// ASCII-oriented and non-ASCII bytes are ignored in raw mode.
				continue
			}
			if unicode.IsPrint(r) {
				line = append(line[:pos], append([]rune{r}, line[pos:]...)...)
				pos++
				e.redraw(prompt, line, pos, echo)
			}
		}
	}
}

func (e *lineEditor) readEscape(line *[]rune, pos *int, historyIndex *int, pending *[]rune, prompt string, echo bool) bool {
	seq := make([]byte, 2)
	if _, err := io.ReadFull(e.in, seq); err != nil {
		return false
	}
	if seq[0] != '[' {
		return false
	}
	switch seq[1] {
	case 'A':
		if len(e.history) == 0 || *historyIndex == 0 {
			return true
		}
		if *historyIndex == len(e.history) {
			*pending = append((*pending)[:0], (*line)...)
		}
		*historyIndex--
		*line = []rune(e.history[*historyIndex])
		*pos = len(*line)
		e.redraw(prompt, *line, *pos, echo)
		return true
	case 'B':
		if *historyIndex >= len(e.history) {
			return true
		}
		*historyIndex++
		if *historyIndex == len(e.history) {
			*line = append((*line)[:0], (*pending)...)
		} else {
			*line = []rune(e.history[*historyIndex])
		}
		*pos = len(*line)
		e.redraw(prompt, *line, *pos, echo)
		return true
	case 'C':
		if *pos < len(*line) {
			*pos++
			e.redraw(prompt, *line, *pos, echo)
		}
		return true
	case 'D':
		if *pos > 0 {
			*pos--
			e.redraw(prompt, *line, *pos, echo)
		}
		return true
	default:
		return false
	}
}

func (e *lineEditor) redraw(prompt string, line []rune, pos int, echo bool) {
	display := string(line)
	if !echo {
		display = strings.Repeat("*", len(line))
	}
	writef(e.out, "\r\033[2K%s%s", prompt, display)
	if back := len(line) - pos; back > 0 {
		writef(e.out, "\033[%dD", back)
	}
}
