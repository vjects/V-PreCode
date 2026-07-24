package pathmanager

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows/registry"
)

var toolsToOverride = []string{
	"php.exe",
	"node.exe",
	"go.exe",
	"composer.bat",
	"composer.phar",
	"mysql.exe",
	"mysqld.exe",
	"redis-server.exe",
	"redis-cli.exe",
}

func containsTargetExecutable(dir string) bool {
	for _, exe := range toolsToOverride {
		if _, err := os.Stat(filepath.Join(dir, exe)); err == nil {
			return true
		}
	}
	return false
}

func getLongPath(p string) string {
	p16, err := syscall.UTF16PtrFromString(p)
	if err != nil {
		return p
	}
	b := make([]uint16, 300)
	n, err := syscall.GetLongPathName(p16, &b[0], uint32(len(b)))
	if err != nil || n == 0 {
		return p
	}
	if n > uint32(len(b)) {
		b = make([]uint16, n)
		n, err = syscall.GetLongPathName(p16, &b[0], uint32(len(b)))
		if err != nil || n == 0 {
			return p
		}
	}
	return syscall.UTF16ToString(b[:n])
}

// CleanAndInjectPath updates User or System PATH depending on what is requested
// For safety and best practice, we usually update User PATH to avoid breaking System PATH
// But since the user requested full global access, updating System PATH is an option.
// Here we will update System PATH if we have admin rights.
func CleanAndInjectPath(rootDir string) error {
	// Our exact paths to inject
	ourPaths := []string{
		filepath.Join(rootDir, "php"),
		filepath.Join(rootDir, "node", "node-v24.18.0-win64", "node-v24.18.0-win-x64"),
		filepath.Join(rootDir, "go", "go", "bin"),
		filepath.Join(rootDir, "composer"),
		filepath.Join(rootDir, "mariadb", "bin"),
		filepath.Join(rootDir, "redis"),
	}

	// Read User PATH instead of System PATH to avoid needing Administrator privileges
	k, err := registry.OpenKey(registry.CURRENT_USER, `Environment`, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("could not open registry key (needs admin?): %v", err)
	}
	defer k.Close()

	sysPath, _, err := k.GetStringValue("Path")
	if err != nil {
		return fmt.Errorf("could not read Path value: %v", err)
	}

	parts := strings.Split(sysPath, ";")
	var newParts []string

	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}

		pLong := strings.ToLower(filepath.Clean(getLongPath(p)))
		rootDirLong := strings.ToLower(filepath.Clean(getLongPath(rootDir)))

		// Check if it's one of our own paths, we skip it here so we can prepend them cleanly
		isOurPath := false
		for _, op := range ourPaths {
			opLong := strings.ToLower(filepath.Clean(getLongPath(op)))
			if pLong == opLong {
				isOurPath = true
				break
			}
		}
		if isOurPath {
			continue
		}

		// If the path is anywhere inside our root directory, it's an old incorrect path, so remove it
		if strings.HasPrefix(pLong, rootDirLong+string(filepath.Separator)) || pLong == rootDirLong {
			fmt.Printf("Removing old/garbage path from our root: %s\n", p)
			continue
		}

		// Check if the old path contains the executables we are overriding (globally installed stuff)
		if containsTargetExecutable(p) {
			fmt.Printf("Removing old tool path: %s\n", p)
			continue
		}

		newParts = append(newParts, p)
	}

	// Prepend our paths
	finalPaths := append(ourPaths, newParts...)
	finalPathStr := strings.Join(finalPaths, ";")

	err = k.SetStringValue("Path", finalPathStr)
	if err != nil {
		return fmt.Errorf("could not write Path value: %v", err)
	}

	// Update current process PATH so exec.Command can find them immediately
	// CRITICAL: We MUST preserve the System PATH in the runtime! 
	// finalPathStr only contains User PATH now. We must prepend ourPaths to the existing runtime PATH instead.
	currentRuntimePath := os.Getenv("PATH")
	ourPathsStr := strings.Join(ourPaths, ";")
	os.Setenv("PATH", ourPathsStr+";"+currentRuntimePath)

	// Broadcast WM_SETTINGCHANGE to notify Windows about the PATH change
	broadcastSettingChange()

	return nil
}

func broadcastSettingChange() {
	var (
		user32               = syscall.NewLazyDLL("user32.dll")
		procSendMessageTimeout = user32.NewProc("SendMessageTimeoutW")
	)
	
	const HWND_BROADCAST = 0xffff
	const WM_SETTINGCHANGE = 0x001A
	const SMTO_ABORTIFHUNG = 0x0002

	envStr, _ := syscall.UTF16PtrFromString("Environment")
	procSendMessageTimeout.Call(
		uintptr(HWND_BROADCAST),
		uintptr(WM_SETTINGCHANGE),
		0,
		uintptr(unsafe.Pointer(envStr)),
		uintptr(SMTO_ABORTIFHUNG),
		uintptr(5000),
		0,
	)
}

func CheckVersions(rootDir string) map[string]string {
	versions := make(map[string]string)
	
	// Helper to run and get first line of output
	runCmd := func(name string, args ...string) string {
		cmd := exec.Command(name, args...)
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		out, err := cmd.Output()
		if err != nil {
			return "Not found or error"
		}
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		if len(lines) > 0 {
			return lines[0]
		}
		return "Unknown"
	}

	versions["PHP"] = runCmd(filepath.Join(rootDir, "php", "php.exe"), "-v")
	versions["Node"] = runCmd(filepath.Join(rootDir, "node", "node-v24.18.0-win64", "node-v24.18.0-win-x64", "node.exe"), "-v")
	versions["Go"] = runCmd(filepath.Join(rootDir, "go", "go", "bin", "go.exe"), "version")
	
	// Composer is usually a bat file or phar. Let's use php to execute the phar directly.
	composerPhar := filepath.Join(rootDir, "composer", "composer.phar")
	if _, err := os.Stat(composerPhar); err == nil {
		versions["Composer"] = runCmd(filepath.Join(rootDir, "php", "php.exe"), composerPhar, "-V")
	} else {
		// fallback to cmd /c composer.bat
		versions["Composer"] = runCmd("cmd", "/c", filepath.Join(rootDir, "composer", "composer.bat"), "-V")
	}

	return versions
}
