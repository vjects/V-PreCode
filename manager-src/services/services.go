package services

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Global process references & mutex locks for thread safety
var (
	mariadbCmd    *exec.Cmd
	phpMyAdminCmd *exec.Cmd
	mailpitCmd    *exec.Cmd
	redisCmd      *exec.Cmd

	// Mutexes to prevent concurrent start/stop race conditions
	mariaDbMutex sync.Mutex
	pmaMutex     sync.Mutex
	mailpitMutex sync.Mutex
	redisMutex   sync.Mutex
)

// TIP: Checks if a TCP port is active on localhost (127.0.0.1)
func isPortOpen(port string) bool {
	timeout := 400 * time.Millisecond
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", port), timeout)
	if err != nil {
		return false
	}
	if conn != nil {
		conn.Close()
		return true
	}
	return false
}

// TIP: Synchronously polls until a port opens or timeout (in seconds) is reached
func waitForPortOpen(port string, timeoutSec int) bool {
	deadline := time.Now().Add(time.Duration(timeoutSec) * time.Second)
	for time.Now().Before(deadline) {
		if isPortOpen(port) {
			return true
		}
		time.Sleep(150 * time.Millisecond)
	}
	return isPortOpen(port)
}

// TIP: Synchronously polls until a port closes or timeout (in seconds) is reached
func waitForPortClose(port string, timeoutSec int) bool {
	deadline := time.Now().Add(time.Duration(timeoutSec) * time.Second)
	for time.Now().Before(deadline) {
		if !isPortOpen(port) {
			return true
		}
		time.Sleep(150 * time.Millisecond)
	}
	return !isPortOpen(port)
}

// GetServicesStatus returns active state for all managed microservices
func GetServicesStatus() map[string]bool {
	return map[string]bool{
		"mariadb": isPortOpen("3306"),
		"mailpit": isPortOpen("8025"),
		"pma":     isPortOpen("8080"),
		"redis":   isPortOpen("6379"),
	}
}

func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}

func killOrphanedProcess(exeName string) {
	// Forcefully kill zombie process trees by executable name
	cmd := exec.Command("taskkill", "/F", "/T", "/IM", exeName)
	hideWindow(cmd)
	_ = cmd.Run()
}

func killProcessUsingPort(port string) {
	cmd := exec.Command("cmd", "/c", fmt.Sprintf("netstat -ano | findstr :%s", port))
	hideWindow(cmd)
	out, err := cmd.Output()
	if err != nil {
		return
	}

	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "LISTENING") {
			parts := strings.Fields(line)
			if len(parts) >= 5 {
				localAddr := parts[1]
				if strings.HasSuffix(localAddr, ":"+port) {
					pid := parts[len(parts)-1]
					if pid != "0" {
						killCmd := exec.Command("taskkill", "/F", "/PID", pid)
						hideWindow(killCmd)
						_ = killCmd.Run()
					}
				}
			}
		}
	}
}

// TIP: Resolves tool directory whether extracted flat or inside a nested version subfolder
func findToolDir(baseDir string, targetSubitem string) string {
	if _, err := os.Stat(filepath.Join(baseDir, targetSubitem)); err == nil {
		return baseDir
	}
	entries, err := os.ReadDir(baseDir)
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				candidate := filepath.Join(baseDir, entry.Name())
				if _, err := os.Stat(filepath.Join(candidate, targetSubitem)); err == nil {
					return candidate
				}
			}
		}
	}
	return baseDir
}

// TIP: Starts MariaDB database daemon and waits synchronously for TCP 3306 readiness
func StartMariaDB(rootDir string) error {
	mariaDbMutex.Lock()
	defer mariaDbMutex.Unlock()

	// 1. If MariaDB is already serving on port 3306, return success immediately
	if isPortOpen("3306") {
		return nil
	}

	// 2. Clear any lingering/zombie mysqld processes
	killOrphanedProcess("mysqld.exe")
	killProcessUsingPort("3306")
	time.Sleep(300 * time.Millisecond)

	mariaDbBase := filepath.Join(rootDir, "runtimes", "mariadb")
	mariaDbDir := findToolDir(mariaDbBase, filepath.Join("bin", "mysqld.exe"))
	dataDir := filepath.Join(mariaDbDir, "data")
	mysqlSchemaDir := filepath.Join(dataDir, "mysql")

	// 3. Auto-initialize data directory if mysql database schema is missing
	if _, err := os.Stat(mysqlSchemaDir); os.IsNotExist(err) {
		// Clean up any uninitialized directory or non-database files like .gitkeep that prevent bootstrap
		_ = os.RemoveAll(dataDir)

		installExe := filepath.Join(mariaDbDir, "bin", "mysql_install_db.exe")
		initCmd := exec.Command(installExe, "--datadir="+dataDir)
		hideWindow(initCmd)
		err := initCmd.Run()
		if err != nil {
			return fmt.Errorf("failed to initialize MariaDB data directory: %v", err)
		}
	}

	// 4. Launch mysqld executable
	exe := filepath.Join(mariaDbDir, "bin", "mysqld.exe")
	mariadbCmd = exec.Command(exe, "--datadir="+dataDir)
	hideWindow(mariadbCmd)

	err := mariadbCmd.Start()
	if err != nil {
		return fmt.Errorf("failed to start MariaDB process: %v", err)
	}

	// 5. Synchronous check: Wait up to 8 seconds for socket readiness
	if !waitForPortOpen("3306", 8) {
		if mariadbCmd != nil && mariadbCmd.Process != nil {
			_ = mariadbCmd.Process.Kill()
		}
		return fmt.Errorf("MariaDB process started but failed to open port 3306 within timeout")
	}

	return nil
}

// TIP: Gracefully shuts down MariaDB using mysqladmin shutdown, fallback to force kill
func StopMariaDB(rootDir string) error {
	mariaDbMutex.Lock()
	defer mariaDbMutex.Unlock()

	if !isPortOpen("3306") {
		return nil
	}

	// Graceful shutdown using mysqladmin tool
	mariaDbBase := filepath.Join(rootDir, "runtimes", "mariadb")
	mariaDbDir := findToolDir(mariaDbBase, filepath.Join("bin", "mysqladmin.exe"))
	exe := filepath.Join(mariaDbDir, "bin", "mysqladmin.exe")
	cmd := exec.Command(exe, "-u", "root", "shutdown")
	hideWindow(cmd)
	_ = cmd.Run()

	// Wait for port 3306 to release
	if !waitForPortClose("3306", 5) {
		// Fallback force cleanup if graceful shutdown timed out
		killOrphanedProcess("mysqld.exe")
		killProcessUsingPort("3306")
		waitForPortClose("3306", 3)
	}

	mariadbCmd = nil
	return nil
}

func fixPHPIni(rootDir string) error {
	phpBase := filepath.Join(rootDir, "runtimes", "php")
	phpDir := findToolDir(phpBase, "php.exe")
	iniPath := filepath.Join(phpDir, "php.ini")

	if _, err := os.Stat(iniPath); os.IsNotExist(err) {
		devIni := filepath.Join(phpDir, "php.ini-development")
		data, err := os.ReadFile(devIni)
		if err != nil {
			return err
		}
		_ = os.WriteFile(iniPath, data, 0644)
	}

	data, err := os.ReadFile(iniPath)
	if err != nil {
		return err
	}
	content := string(data)

	// Enable essential development extensions
	content = strings.Replace(content, ";extension_dir = \"ext\"", "extension_dir = \"ext\"", -1)
	content = strings.Replace(content, "; extension_dir = \"ext\"", "extension_dir = \"ext\"", -1)
	content = strings.Replace(content, ";extension=mysqli", "extension=mysqli", -1)
	content = strings.Replace(content, ";extension=pdo_mysql", "extension=pdo_mysql", -1)
	content = strings.Replace(content, ";extension=mbstring", "extension=mbstring", -1)
	content = strings.Replace(content, ";extension=curl", "extension=curl", -1)
	content = strings.Replace(content, ";extension=gd", "extension=gd", -1)
	content = strings.Replace(content, ";extension=zip", "extension=zip", -1)

	// Optimizations for dev limits
	content = strings.Replace(content, "memory_limit = 128M", "memory_limit = 1024M", -1)
	content = strings.Replace(content, "post_max_size = 8M", "post_max_size = 256M", -1)
	content = strings.Replace(content, "upload_max_filesize = 2M", "upload_max_filesize = 256M", -1)
	content = strings.Replace(content, "max_execution_time = 30", "max_execution_time = 300", -1)
	content = strings.Replace(content, "max_input_time = 60", "max_input_time = 300", -1)

	return os.WriteFile(iniPath, []byte(content), 0644)
}

func fixPMAConfig(pmaDir string) error {
	cfgFile := filepath.Join(pmaDir, "config.inc.php")
	if _, err := os.Stat(cfgFile); os.IsNotExist(err) {
		content := `<?php
/* phpMyAdmin Configuration for V-PreCode */
$cfg['blowfish_secret'] = 'vprecode_secret_cookie_key_32bytes_long!';

$i = 0;
$i++;
$cfg['Servers'][$i]['auth_type'] = 'config';
$cfg['Servers'][$i]['user'] = 'root';
$cfg['Servers'][$i]['password'] = '';
$cfg['Servers'][$i]['host'] = '127.0.0.1';
$cfg['Servers'][$i]['compress'] = false;
$cfg['Servers'][$i]['AllowNoPassword'] = true;
`
		return os.WriteFile(cfgFile, []byte(content), 0644)
	}
	return nil
}

// TIP: Starts phpMyAdmin web interface on port 8080 and opens browser
func StartPHPMyAdmin(rootDir string) error {
	pmaMutex.Lock()
	defer pmaMutex.Unlock()

	if !isPortOpen("8080") {
		killProcessUsingPort("8080")
		time.Sleep(300 * time.Millisecond)

		if err := fixPHPIni(rootDir); err != nil {
			return fmt.Errorf("failed to prepare php.ini: %v", err)
		}

		phpBase := filepath.Join(rootDir, "runtimes", "php")
		phpExe := filepath.Join(findToolDir(phpBase, "php.exe"), "php.exe")
		pmaBase := filepath.Join(rootDir, "runtimes", "phpmyadmin")
		pmaDir := findToolDir(pmaBase, "index.php")

		if _, err := os.Stat(phpExe); os.IsNotExist(err) {
			return fmt.Errorf("php.exe not found")
		}

		if err := fixPMAConfig(pmaDir); err != nil {
			return fmt.Errorf("failed to prepare phpMyAdmin config: %v", err)
		}

		phpMyAdminCmd = exec.Command(phpExe, "-S", "127.0.0.1:8080", "-t", pmaDir)
		hideWindow(phpMyAdminCmd)
		err := phpMyAdminCmd.Start()
		if err != nil {
			return fmt.Errorf("failed to start phpMyAdmin server: %v", err)
		}

		// Wait synchronously for port 8080
		if !waitForPortOpen("8080", 5) {
			return fmt.Errorf("phpMyAdmin web server failed to bind to port 8080")
		}
	}

	cmdBrowser := exec.Command("cmd", "/c", "start", "", "http://127.0.0.1:8080")
	hideWindow(cmdBrowser)
	_ = cmdBrowser.Start()
	return nil
}

// TIP: Stops phpMyAdmin PHP server process
func StopPHPMyAdmin() error {
	pmaMutex.Lock()
	defer pmaMutex.Unlock()

	killProcessUsingPort("8080")
	waitForPortClose("8080", 3)
	phpMyAdminCmd = nil
	return nil
}

// TIP: Starts Mailpit SMTP & webmail tool (1025 / 8025) and opens browser UI
func StartMailpit(rootDir string) error {
	mailpitMutex.Lock()
	defer mailpitMutex.Unlock()

	if !isPortOpen("8025") {
		killOrphanedProcess("mailpit.exe")
		killProcessUsingPort("8025")
		killProcessUsingPort("1025")
		time.Sleep(300 * time.Millisecond)

		mailpitBase := filepath.Join(rootDir, "runtimes", "mailpit")
		mailpitDir := findToolDir(mailpitBase, "mailpit.exe")
		exe := filepath.Join(mailpitDir, "mailpit.exe")

		if _, err := os.Stat(exe); os.IsNotExist(err) {
			return fmt.Errorf("mailpit.exe not found at %s", exe)
		}

		mailpitCmd = exec.Command(exe, "-l", "127.0.0.1:8025", "-s", "127.0.0.1:1025")
		hideWindow(mailpitCmd)
		err := mailpitCmd.Start()
		if err != nil {
			return fmt.Errorf("failed to start Mailpit binary: %v", err)
		}

		// Wait synchronously for port 8025
		if !waitForPortOpen("8025", 5) {
			return fmt.Errorf("Mailpit failed to bind to port 8025")
		}
	}

	cmdBrowser := exec.Command("cmd", "/c", "start", "", "http://127.0.0.1:8025")
	hideWindow(cmdBrowser)
	_ = cmdBrowser.Start()
	return nil
}

// TIP: Stops Mailpit background process
func StopMailpit() error {
	mailpitMutex.Lock()
	defer mailpitMutex.Unlock()

	killOrphanedProcess("mailpit.exe")
	killProcessUsingPort("8025")
	killProcessUsingPort("1025")
	waitForPortClose("8025", 3)
	mailpitCmd = nil
	return nil
}

// TIP: Starts Redis cache/key-value store server and waits synchronously for TCP 6379 readiness
func StartRedis(rootDir string) error {
	redisMutex.Lock()
	defer redisMutex.Unlock()

	if isPortOpen("6379") {
		return nil
	}

	killOrphanedProcess("redis-server.exe")
	killProcessUsingPort("6379")
	time.Sleep(300 * time.Millisecond)

	redisBase := filepath.Join(rootDir, "runtimes", "redis")
	redisDir := findToolDir(redisBase, "redis-server.exe")
	exe := filepath.Join(redisDir, "redis-server.exe")

	if _, err := os.Stat(exe); os.IsNotExist(err) {
		return fmt.Errorf("redis-server.exe not found at %s", exe)
	}

	confFile := filepath.Join(redisDir, "redis.windows.conf")
	if _, err := os.Stat(confFile); err == nil {
		redisCmd = exec.Command(exe, confFile)
	} else {
		redisCmd = exec.Command(exe)
	}

	hideWindow(redisCmd)
	err := redisCmd.Start()
	if err != nil {
		return fmt.Errorf("failed to start Redis server: %v", err)
	}

	if !waitForPortOpen("6379", 5) {
		if redisCmd != nil && redisCmd.Process != nil {
			_ = redisCmd.Process.Kill()
		}
		return fmt.Errorf("Redis server failed to bind to port 6379 within timeout")
	}

	return nil
}

// TIP: Gracefully shuts down Redis server using redis-cli shutdown or fallback process kill
func StopRedis(rootDir string) error {
	redisMutex.Lock()
	defer redisMutex.Unlock()

	if !isPortOpen("6379") {
		return nil
	}

	redisBase := filepath.Join(rootDir, "runtimes", "redis")
	redisDir := findToolDir(redisBase, "redis-cli.exe")
	cliExe := filepath.Join(redisDir, "redis-cli.exe")

	if _, err := os.Stat(cliExe); err == nil {
		shutdownCmd := exec.Command(cliExe, "shutdown")
		hideWindow(shutdownCmd)
		_ = shutdownCmd.Run()
	}

	if !waitForPortClose("6379", 3) {
		killOrphanedProcess("redis-server.exe")
		killProcessUsingPort("6379")
		waitForPortClose("6379", 2)
	}

	redisCmd = nil
	return nil
}

// StopAll cleans up all sub-services when DevEnvManager shuts down
func StopAll(rootDir string) {
	_ = StopMariaDB(rootDir)
	_ = StopPHPMyAdmin()
	_ = StopMailpit()
	_ = StopRedis(rootDir)
}

// ViewPHPIni ensures php.ini exists and opens it in Notepad for easy configuration
func ViewPHPIni(rootDir string) error {
	if err := fixPHPIni(rootDir); err != nil {
		return err
	}
	phpBase := filepath.Join(rootDir, "runtimes", "php")
	phpDir := findToolDir(phpBase, "php.exe")
	iniPath := filepath.Join(phpDir, "php.ini")
	cmd := exec.Command("notepad.exe", iniPath)
	return cmd.Start()
}

