//go:build linux

package main

import (
	"bufio"
	"io"
	"os"

	"golang.org/x/sys/unix"
)

// readReadyTerminalInput drains terminal input already pending after the first
// line. A short poll keeps a single-line prompt responsive while catching
// additional lines delivered together by a paste.
func readReadyTerminalInput(input *os.File, reader *bufio.Reader) ([]byte, error) {
	var trailing []byte
	if buffered := reader.Buffered(); buffered > 0 {
		remaining, err := reader.Peek(buffered)
		if err != nil {
			return nil, err
		}
		trailing = append(trailing, remaining...)
	}

	fd := int(input.Fd())
	ready, err := terminalInputReady(fd, 25)
	if err != nil || !ready {
		return trailing, err
	}

	buffer := make([]byte, 4096)
	for {
		n, err := input.Read(buffer)
		if n > 0 {
			trailing = append(trailing, buffer[:n]...)
		}
		if err != nil {
			if err == io.EOF {
				return trailing, nil
			}
			return nil, err
		}
		if n == 0 {
			return trailing, nil
		}

		ready, err = terminalInputReady(fd, 0)
		if err != nil || !ready {
			return trailing, err
		}
	}
}

func terminalInputReady(fd, timeoutMillis int) (bool, error) {
	descriptors := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
	count, err := unix.Poll(descriptors, timeoutMillis)
	if err != nil {
		return false, err
	}
	readyEvents := int16(unix.POLLIN | unix.POLLHUP | unix.POLLERR)
	return count > 0 && descriptors[0].Revents&readyEvents != 0, nil
}
