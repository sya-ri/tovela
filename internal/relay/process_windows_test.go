package relay

import (
	"errors"
	"os"
	"testing"

	"golang.org/x/sys/windows"
)

func TestFrpcExecutionError(t *testing.T) {
	for _, tc := range []struct {
		status  error
		blocked bool
	}{
		{windows.ERROR_ACCESS_DENIED, true},
		{windows.ERROR_VIRUS_INFECTED, true},
		{windows.ERROR_VIRUS_DELETED, true},
		{windows.ERROR_FILE_NOT_FOUND, false},
	} {
		err := frpcExecutionError(&os.PathError{Op: "fork/exec", Path: "frpc.exe", Err: tc.status})
		if errors.Is(err, ErrFrpcBlocked) != tc.blocked || !errors.Is(err, tc.status) {
			t.Errorf("Windows status %v classified as %v", tc.status, err)
		}
	}
}
