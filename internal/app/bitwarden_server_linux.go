package app

import (
	"os/exec"
	"syscall"
)

// bw serve держит разблокированное хранилище: при аварийном завершении
// приложения ядро гасит его вместе с родителем.
func bindToParent(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGTERM}
}
