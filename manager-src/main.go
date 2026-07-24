package main

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"dev-env-manager/pathmanager"
	"dev-env-manager/server"
	"dev-env-manager/services"
	"dev-env-manager/shortcut"
)

//go:embed ui/*
var uiFS embed.FS

func main() {
	// Determine root directory (one level above where the executable or main.go is)
	exePath, err := os.Executable()
	if err != nil {
		fmt.Println("Error getting executable path:", err)
		return
	}
	
	// If running via setup.bat, the executable is now in the root directory.
	rootDir := filepath.Dir(exePath)

	// Create Desktop Shortcut
	err = shortcut.CreateDesktopShortcut(rootDir)
	if err != nil {
		fmt.Println("Warning: Could not create shortcut:", err)
	}

	// Auto-fix PATH on startup
	err = pathmanager.CleanAndInjectPath(rootDir)
	if err != nil {
		fmt.Println("Warning: Could not fix PATH:", err)
	} else {
		fmt.Println("PATH injected successfully.")
	}

	// Setup Graceful Shutdown via OS signals (e.g. Ctrl+C or kill)
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		services.StopAll(rootDir)
		os.Exit(0)
	}()

	// Setup Heartbeat Watcher (Fallback if UI crashes without sending /api/exit)
	server.LastPing = time.Now()
	go func() {
		time.Sleep(10 * time.Second) // allow 10s for initial startup
		for {
			time.Sleep(5 * time.Second)
			// Increase timeout to 10 minutes (600s) because Edge throttles background timers
			// when the window loses focus, which caused false positive shutdowns!
			if time.Since(server.LastPing) > 600*time.Second {
				services.StopAll(rootDir)
				os.Exit(0)
			}
		}
	}()

	// Open the UI in Edge App mode (if Edge exists, fallback to Chrome)
	port := "54321"
	url := "http://localhost:" + port
	
	go func() {
		time.Sleep(1 * time.Second) // wait for server to start
		paths := []string{
			`C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`,
			`C:\Program Files\Google\Chrome\Application\chrome.exe`,
			`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
		}
		
		profileDir := filepath.Join(os.TempDir(), "DevEnvUIProfile")

		launched := false
		for _, p := range paths {
			if _, err := os.Stat(p); err == nil {
				// Run browser directly with dedicated profile and app ID for taskbar identity
				exec.Command(p, "--app="+url, "--app-id=VPreCode", "--window-size=940,195", "--user-data-dir="+profileDir).Start()
				launched = true
				break
			}
		}

		if !launched {
			// fallback to default browser
			exec.Command("cmd", "/c", "start", "", url).Start()
		}
	}()

	// Extract the embedded UI subdirectory
	subFS, err := fs.Sub(uiFS, "ui")
	if err != nil {
		fmt.Println("Error loading embedded UI:", err)
		return
	}

	// Start Server (blocks)
	server.StartServer(rootDir, port, subFS)
}
