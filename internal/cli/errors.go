package cli

import (
	"errors"
	"os/exec"
)

// asExit reports an error that carries the exit code of a process.
func asExit(err error, target **exec.ExitError) bool { return errors.As(err, target) }
