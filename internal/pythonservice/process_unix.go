//go:build !windows

package pythonservice

import (
	"os"
	"os/exec"
	"syscall"
)

func shellCommand(command string) *exec.Cmd {
	cmd := exec.Command("sh", "-c", command)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return cmd
}

func signalProcess(cmd *exec.Cmd, signal os.Signal) error {
	sig, ok := signal.(syscall.Signal)
	if !ok {
		return cmd.Process.Signal(signal)
	}
	return syscall.Kill(-cmd.Process.Pid, sig)
}

func killProcess(cmd *exec.Cmd) error {
	return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
}
