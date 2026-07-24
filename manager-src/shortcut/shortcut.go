package shortcut

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func CreateDesktopShortcut(rootDir string) error {
	exePath := filepath.Join(rootDir, "DevEnvManager.exe")
	iconPath := filepath.Join(rootDir, "icon.ico")
	
	// Get Desktop folder using environment variables
	userProfile := os.Getenv("USERPROFILE")
	desktopDir := filepath.Join(userProfile, "Desktop")
	shortcutPath := filepath.Join(desktopDir, "V-PreCode.lnk")

	vbsContent := fmt.Sprintf(`
Set oWS = WScript.CreateObject("WScript.Shell")
sLinkFile = "%s"
Set oLink = oWS.CreateShortcut(sLinkFile)
oLink.TargetPath = "%s"
oLink.WorkingDirectory = "%s"
oLink.IconLocation = "%s"
oLink.Save
`, shortcutPath, exePath, rootDir, iconPath)

	vbsPath := filepath.Join(os.TempDir(), "createshortcut.vbs")
	err := os.WriteFile(vbsPath, []byte(vbsContent), 0644)
	if err != nil {
		return err
	}
	defer os.Remove(vbsPath)

	cmd := exec.Command("cscript", "//nologo", vbsPath)
	return cmd.Run()
}
