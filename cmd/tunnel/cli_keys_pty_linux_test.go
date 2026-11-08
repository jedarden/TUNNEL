//go:build linux

package main

import (
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

func openTestPTY(t *testing.T) (*os.File, *os.File) {
	t.Helper()

	masterFD, err := unix.Open("/dev/ptmx", unix.O_RDWR|unix.O_NOCTTY|unix.O_CLOEXEC, 0)
	if err != nil {
		t.Skipf("open pseudo-terminal master: %v", err)
	}
	master := os.NewFile(uintptr(masterFD), "/dev/ptmx")
	t.Cleanup(func() { _ = master.Close() })

	if err := unix.IoctlSetPointerInt(masterFD, unix.TIOCSPTLCK, 0); err != nil {
		t.Fatalf("unlock pseudo-terminal: %v", err)
	}
	ptyNumber, err := unix.IoctlGetInt(masterFD, unix.TIOCGPTN)
	if err != nil {
		t.Fatalf("get pseudo-terminal number: %v", err)
	}

	slaveFD, err := unix.Open(fmt.Sprintf("/dev/pts/%d", ptyNumber), unix.O_RDWR|unix.O_NOCTTY|unix.O_CLOEXEC, 0)
	if err != nil {
		t.Fatalf("open pseudo-terminal slave: %v", err)
	}
	slave := os.NewFile(uintptr(slaveFD), "pseudo-terminal slave")
	t.Cleanup(func() { _ = slave.Close() })
	return master, slave
}

func TestReadSingleKeyLineAcceptsOneTerminalLine(t *testing.T) {
	master, slave := openTestPTY(t)
	if _, err := io.WriteString(master, testKeyAliceLaptop+"\n"); err != nil {
		t.Fatalf("write key line to pseudo-terminal: %v", err)
	}

	got, err := readSingleKeyLine(slave)
	if err != nil {
		t.Fatalf("readSingleKeyLine: %v", err)
	}
	if got != testKeyAliceLaptop {
		t.Errorf("readSingleKeyLine() = %q, want one key line", got)
	}
}

func TestReadSingleKeyLineRejectsTerminalPasteWithMultipleKeys(t *testing.T) {
	master, slave := openTestPTY(t)
	if _, err := io.WriteString(master, testKeyAliceLaptop+"\n"+testKeyAlicePhone+"\n"); err != nil {
		t.Fatalf("write pasted keys to pseudo-terminal: %v", err)
	}

	_, err := readSingleKeyLine(slave)
	if err == nil || !strings.Contains(err.Error(), "exactly one SSH public key line") {
		t.Fatalf("readSingleKeyLine error = %v, want multi-key rejection", err)
	}
}
