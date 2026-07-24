package server

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"time"

	"dev-env-manager/pathmanager"
	"dev-env-manager/services"
	"dev-env-manager/winnat"
)

type ServerConfig struct {
	RootDir string
}

var cfg ServerConfig
var LastPing time.Time

func StartServer(rootDir string, port string, uiFS fs.FS) {
	cfg.RootDir = rootDir

	// Serve static files from the embedded fs
	http.Handle("/", http.FileServer(http.FS(uiFS)))

	// API endpoints
	http.HandleFunc("/api/status", handleStatus)
	http.HandleFunc("/api/ping", handlePing)
	http.HandleFunc("/api/services/status", handleServicesStatus)
	http.HandleFunc("/api/mariadb/start", handleStartMariaDB)
	http.HandleFunc("/api/mariadb/stop", handleStopMariaDB)
	http.HandleFunc("/api/mailpit/start", handleStartMailpit)
	http.HandleFunc("/api/mailpit/stop", handleStopMailpit)
	http.HandleFunc("/api/pma/start", handleStartPMA)
	http.HandleFunc("/api/pma/stop", handleStopPMA)
	http.HandleFunc("/api/phpini/view", handleViewPHPIni)
	http.HandleFunc("/api/winnat/reset", handleWinnatReset)
	http.HandleFunc("/api/path/fix", handlePathFix)
	http.HandleFunc("/api/versions", handleVersions)
	http.HandleFunc("/api/exit", handleExit)

	fmt.Printf("Server listening on http://localhost:%s\n", port)
	http.ListenAndServe(":"+port, nil)
}

func respondJSON(w http.ResponseWriter, success bool, message string) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": success,
		"message": message,
	})
}

func handleStatus(w http.ResponseWriter, r *http.Request) {
	// A simple mock for now, can be expanded to check process list
	respondJSON(w, true, "Running")
}

func handlePing(w http.ResponseWriter, r *http.Request) {
	LastPing = time.Now()
	w.WriteHeader(http.StatusOK)
}

func handleServicesStatus(w http.ResponseWriter, r *http.Request) {
	status := services.GetServicesStatus()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    status,
	})
}

func handleStartMariaDB(w http.ResponseWriter, r *http.Request) {
	err := services.StartMariaDB(cfg.RootDir)
	if err != nil {
		respondJSON(w, false, err.Error())
		return
	}
	respondJSON(w, true, "")
}

func handleStopMariaDB(w http.ResponseWriter, r *http.Request) {
	err := services.StopMariaDB(cfg.RootDir)
	if err != nil {
		respondJSON(w, false, err.Error())
		return
	}
	respondJSON(w, true, "")
}

func handleStartMailpit(w http.ResponseWriter, r *http.Request) {
	err := services.StartMailpit(cfg.RootDir)
	if err != nil {
		respondJSON(w, false, err.Error())
		return
	}
	respondJSON(w, true, "")
}

func handleStopMailpit(w http.ResponseWriter, r *http.Request) {
	err := services.StopMailpit()
	if err != nil {
		respondJSON(w, false, err.Error())
		return
	}
	respondJSON(w, true, "Mailpit Stopped")
}

func handleStartPMA(w http.ResponseWriter, r *http.Request) {
	err := services.StartPHPMyAdmin(cfg.RootDir)
	if err != nil {
		respondJSON(w, false, err.Error())
		return
	}
	respondJSON(w, true, "")
}

func handleStopPMA(w http.ResponseWriter, r *http.Request) {
	err := services.StopPHPMyAdmin()
	if err != nil {
		respondJSON(w, false, err.Error())
		return
	}
	respondJSON(w, true, "PHPMyAdmin Stopped")
}

func handleViewPHPIni(w http.ResponseWriter, r *http.Request) {
	err := services.ViewPHPIni(cfg.RootDir)
	if err != nil {
		respondJSON(w, false, "Failed to open php.ini")
		return
	}
	respondJSON(w, true, "Opened php.ini")
}

func handleWinnatReset(w http.ResponseWriter, r *http.Request) {
	err := winnat.ResetWinnat()
	if err != nil {
		respondJSON(w, false, err.Error())
		return
	}
	respondJSON(w, true, "")
}

func handlePathFix(w http.ResponseWriter, r *http.Request) {
	err := pathmanager.CleanAndInjectPath(cfg.RootDir)
	if err != nil {
		respondJSON(w, false, err.Error())
		return
	}
	respondJSON(w, true, "PATH updated successfully")
}

func handleVersions(w http.ResponseWriter, r *http.Request) {
	versions := pathmanager.CheckVersions(cfg.RootDir)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    versions,
	})
}

func handleExit(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, true, "Shutting down...")
	
	// Graceful shutdown
	go func() {
		services.StopAll(cfg.RootDir)
		os.Exit(0)
	}()
}
