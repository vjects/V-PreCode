package shortcut

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// EnsureShortcuts verifies and creates shortcuts on both Desktop and Windows Start Menu program folder
func EnsureShortcuts(rootDir string) error {
	exePath := filepath.Join(rootDir, "DevEnvManager.exe")
	iconPath := filepath.Join(rootDir, "docs", "icon.ico")
	if _, err := os.Stat(iconPath); os.IsNotExist(err) {
		iconPath = filepath.Join(rootDir, "icon.ico")
	}

	userProfile := os.Getenv("USERPROFILE")
	appData := os.Getenv("APPDATA")

	// 1. Desktop Shortcut Path
	desktopShortcut := filepath.Join(userProfile, "Desktop", "V-PreCode.lnk")

	// 2. Windows Start Menu Programs Directory Path
	startMenuFolder := filepath.Join(appData, "Microsoft", "Windows", "Start Menu", "Programs", "V-PreCode")
	startMenuShortcut := filepath.Join(startMenuFolder, "V-PreCode.lnk")

	// Create dedicated Start Menu folder if missing
	if err := os.MkdirAll(startMenuFolder, 0755); err != nil {
		fmt.Println("Warning: Could not create Start Menu directory:", err)
	}

	vbsContent := fmt.Sprintf(`
Set oWS = WScript.CreateObject("WScript.Shell")

Sub CreateShortcut(sLinkFile, sTargetPath, sWorkDir, sIconPath)
    Set oLink = oWS.CreateShortcut(sLinkFile)
    oLink.TargetPath = sTargetPath
    oLink.WorkingDirectory = sWorkDir
    oLink.IconLocation = sIconPath
    oLink.Save
End Sub

CreateShortcut "%s", "%s", "%s", "%s"
CreateShortcut "%s", "%s", "%s", "%s"
`, desktopShortcut, exePath, rootDir, iconPath, startMenuShortcut, exePath, rootDir, iconPath)

	vbsPath := filepath.Join(os.TempDir(), "vprecode_shortcuts.vbs")
	err := os.WriteFile(vbsPath, []byte(vbsContent), 0644)
	if err != nil {
		return err
	}
	defer os.Remove(vbsPath)

	cmd := exec.Command("cscript", "//nologo", vbsPath)
	return cmd.Run()
}

// CreateDesktopShortcut maintains backwards compatibility alias
func CreateDesktopShortcut(rootDir string) error {
	return EnsureShortcuts(rootDir)
}
