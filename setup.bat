@echo off
setlocal

:: Check for Administrator privileges
net session >nul 2>&1
if %errorLevel% == 0 (
    goto :Admin
) else (
    echo Requesting Administrative Privileges...
    echo Set UAC = CreateObject^("Shell.Application"^) > "%temp%\getadmin.vbs"
    echo UAC.ShellExecute "%~s0", "", "", "runas", 1 >> "%temp%\getadmin.vbs"
    "%temp%\getadmin.vbs"
    del "%temp%\getadmin.vbs"
    exit /B
)

:Admin
:: Now running as Admin
cd /d "%~dp0"
echo Starting V-PreCode Setup...

:: Set local paths explicitly for compilation
set GOROOT=%~dp0go\go
set GOPATH=%~dp0go-workspace
set PATH=%~dp0go\go\bin;%PATH%

:: Compile the manager if not already compiled
if not exist "DevEnvManager.exe" (
    echo Compiling the Go core application...
    cd manager-src
    go build -ldflags="-s -w -H windowsgui" -o ..\DevEnvManager.exe
    if %errorlevel% neq 0 (
        echo Error: Compilation failed.
        pause
        exit /b %errorlevel%
    )
    cd ..
)

:: Run the manager
start "" DevEnvManager.exe

echo Setup launched successfully.
exit /B
