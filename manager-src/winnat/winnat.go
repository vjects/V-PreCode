package winnat

import (
	"os/exec"
	"syscall"
)

func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}

func ResetWinnat() error {
	// Elevate powershell to run net stop winnat and net start winnat

	// We use Start-Process with -Verb RunAs to request Administrator privileges from the user.
	// -Wait ensures we wait for the elevated process to finish.
	cmd := exec.Command("powershell", "-NoProfile", "-WindowStyle", "Hidden", "-Command", 
		"Start-Process powershell -Wait -WindowStyle Hidden -Verb RunAs -ArgumentList '-NoProfile', '-WindowStyle', 'Hidden', '-Command', 'net stop winnat; net start winnat'")
	hideWindow(cmd)
	return cmd.Run()
}
