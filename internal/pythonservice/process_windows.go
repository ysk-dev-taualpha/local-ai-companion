//go:build windows

package pythonservice

import (
	"os"
	"os/exec"
)

func shellCommand(command string) *exec.Cmd {
	return exec.Command("cmd", "/C", command)
}

func signalProcess(cmd *exec.Cmd, signal os.Signal) error {
	return cmd.Process.Signal(signal)
}

func killProcess(cmd *exec.Cmd) error {
	return cmd.Process.Kill()
}
