//go:build !linux

package main

import (
	"bufio"
	"io"
	"os"
	"time"
)

// readReadyTerminalInput checks bytes buffered after the first terminal line
// and uses read deadlines where the platform supports them.
func readReadyTerminalInput(input *os.File, reader *bufio.Reader) ([]byte, error) {
	var trailing []byte
	if buffered := reader.Buffered(); buffered > 0 {
		remaining, err := reader.Peek(buffered)
		if err != nil {
			return nil, err
		}
		trailing = append(trailing, remaining...)
	}

	if err := input.SetReadDeadline(time.Now().Add(25 * time.Millisecond)); err != nil {
		return trailing, nil
	}
	defer func() { _ = input.SetReadDeadline(time.Time{}) }()

	buffer := make([]byte, 4096)
	for {
		n, err := input.Read(buffer)
		if n > 0 {
			trailing = append(trailing, buffer[:n]...)
		}
		if err != nil {
			if timeout, ok := err.(interface{ Timeout() bool }); ok && timeout.Timeout() {
				return trailing, nil
			}
			if err == io.EOF {
				return trailing, nil
			}
			return nil, err
		}
		if n == 0 {
			return trailing, nil
		}
	}
}
