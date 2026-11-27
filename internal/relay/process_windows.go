package relay

import (
	"errors"
	"fmt"
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

func hideConsole(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}

func frpcExecutionError(err error) error {
	if errors.Is(err, windows.ERROR_ACCESS_DENIED) ||
		errors.Is(err, windows.ERROR_VIRUS_INFECTED) ||
		errors.Is(err, windows.ERROR_VIRUS_DELETED) {
		return fmt.Errorf("%w: %w", ErrFrpcBlocked, err)
	}
	return err
}
