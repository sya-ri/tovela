//go:build !windows

package relay

import "os/exec"

func hideConsole(_ *exec.Cmd) {}

func frpcExecutionError(err error) error { return err }
