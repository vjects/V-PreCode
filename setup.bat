@echo off
setlocal enabledelayedexpansion
title V-PreCode v3.0.0 Environment Setup

echo ===================================================
echo          V-PreCode v3.0.0 Setup Engine
echo ===================================================
echo.

:: Get current directory root
set "ROOT=%~dp0"
if "%ROOT:~-1%"=="\" set "ROOT=%ROOT:~0,-1%"

:: Detect Best Available High-Speed Archive Extractor
set "EXTRACTOR_TYPE=PowerShell"
set "EXTRACTOR_EXE="

if exist "%SystemRoot%\System32\tar.exe" (
    set "EXTRACTOR_TYPE=WindowsTar"
    set "EXTRACTOR_EXE=%SystemRoot%\System32\tar.exe"
) else if exist "C:\Program Files\7-Zip\7z.exe" (
    set "EXTRACTOR_TYPE=7-Zip"
    set "EXTRACTOR_EXE=C:\Program Files\7-Zip\7z.exe"
) else if exist "C:\Program Files (x86)\7-Zip\7z.exe" (
    set "EXTRACTOR_TYPE=7-Zip"
    set "EXTRACTOR_EXE=C:\Program Files (x86)\7-Zip\7z.exe"
) else if exist "C:\Program Files\WinRAR\WinRAR.exe" (
    set "EXTRACTOR_TYPE=WinRAR"
    set "EXTRACTOR_EXE=C:\Program Files\WinRAR\WinRAR.exe"
) else if exist "C:\Program Files (x86)\WinRAR\WinRAR.exe" (
    set "EXTRACTOR_TYPE=WinRAR"
    set "EXTRACTOR_EXE=C:\Program Files (x86)\WinRAR\WinRAR.exe"
)

echo [Info] High-Speed Extraction Engine: !EXTRACTOR_TYPE!
echo.

:: Step 1: Idempotency Check (Unpack assets into runtimes if missing)
if not exist "%ROOT%\runtimes\php\php.exe" (
    echo [Step 1/8] Creating runtimes directory structure...
    if not exist "%ROOT%\runtimes" mkdir "%ROOT%\runtimes"
    if not exist "%ROOT%\runtimes\php" mkdir "%ROOT%\runtimes\php"
    if not exist "%ROOT%\runtimes\node" mkdir "%ROOT%\runtimes\node"
    if not exist "%ROOT%\runtimes\go" mkdir "%ROOT%\runtimes\go"
    if not exist "%ROOT%\runtimes\composer" mkdir "%ROOT%\runtimes\composer"
    if not exist "%ROOT%\runtimes\mariadb" mkdir "%ROOT%\runtimes\mariadb"
    if not exist "%ROOT%\runtimes\redis" mkdir "%ROOT%\runtimes\redis"
    if not exist "%ROOT%\runtimes\mailpit" mkdir "%ROOT%\runtimes\mailpit"
    if not exist "%ROOT%\runtimes\phpmyadmin" mkdir "%ROOT%\runtimes\phpmyadmin"

    echo [Step 2/8] Strictly unpacking clean vendor archives...

    :: Unpack PHP (Synchronous)
    echo  -> Unpacking PHP...
    for %%F in ("%ROOT%\assets\php\*.zip") do call :ExtractArchive "%%F" "%ROOT%\runtimes\php"
    
    :: Unpack Node.js (Synchronous)
    echo  -> Unpacking Node.js...
    for %%F in ("%ROOT%\assets\node\*.zip") do call :ExtractArchive "%%F" "%ROOT%\runtimes\node"
    
    :: Unpack Go Compiler (Synchronous)
    echo  -> Unpacking Go Compiler...
    for %%F in ("%ROOT%\assets\go\*.zip") do call :ExtractArchive "%%F" "%ROOT%\runtimes\go"
    
    :: Unpack MariaDB Database Server (Synchronous)
    echo  -> Unpacking MariaDB Server...
    for %%F in ("%ROOT%\assets\mariadb\*.zip") do call :ExtractArchive "%%F" "%ROOT%\runtimes\mariadb"
    
    :: Unpack Redis Cache Server (Synchronous)
    echo  -> Unpacking Redis Server...
    for %%F in ("%ROOT%\assets\redis\*.zip") do call :ExtractArchive "%%F" "%ROOT%\runtimes\redis"
    
    :: Unpack Mailpit (Synchronous)
    echo  -> Unpacking Mailpit...
    for %%F in ("%ROOT%\assets\mailpit\*.zip") do call :ExtractArchive "%%F" "%ROOT%\runtimes\mailpit"
    
    :: Unpack phpMyAdmin (Synchronous)
    echo  -> Unpacking phpMyAdmin...
    for %%F in ("%ROOT%\assets\phpmyadmin\*.zip") do call :ExtractArchive "%%F" "%ROOT%\runtimes\phpmyadmin"

    echo [Step 3/8] Configuring Composer wrapper...
    if exist "%ROOT%\assets\composer\composer.phar" (
        copy /Y "%ROOT%\assets\composer\composer.phar" "%ROOT%\runtimes\composer\composer.phar" >nul
        (
            echo @php "%%~dp0composer.phar" %%*
        ) > "%ROOT%\runtimes\composer\composer.bat"
    )

    echo [Step 4/8] Generating and tuning php.ini and phpMyAdmin config...
    if exist "%ROOT%\runtimes\php\php.ini-development" (
        copy /Y "%ROOT%\runtimes\php\php.ini-development" "%ROOT%\runtimes\php\php.ini" >nul
        powershell -NoProfile -Command "$c = Get-Content '%ROOT%\runtimes\php\php.ini'; $c = $c -replace ';extension_dir = \"ext\"', 'extension_dir = \"ext\"' -replace '; extension_dir = \"ext\"', 'extension_dir = \"ext\"' -replace ';extension=mysqli', 'extension=mysqli' -replace ';extension=pdo_mysql', 'extension=pdo_mysql' -replace ';extension=mbstring', 'extension=mbstring' -replace ';extension=curl', 'extension=curl' -replace ';extension=gd', 'extension=gd' -replace ';extension=zip', 'extension=zip' -replace 'memory_limit = 128M', 'memory_limit = 1024M' -replace 'post_max_size = 8M', 'post_max_size = 256M' -replace 'upload_max_filesize = 2M', 'upload_max_filesize = 256M'; Set-Content '%ROOT%\runtimes\php\php.ini' $c" >nul 2>&1
    )

    echo [Step 5/8] Checking Visual C++ Redistributable...
    if exist "%ROOT%\assets\vcredist\VC_redist.x64.exe" (
        start /wait "" "%ROOT%\assets\vcredist\VC_redist.x64.exe" /passive /norestart
    )

    echo [Step 6/8] Initializing MariaDB system database...
    set "MYSQL_INIT="
    for /f "delims=" %%I in ('dir /b /s "%ROOT%\runtimes\mariadb\mysql_install_db.exe" 2^>nul') do set "MYSQL_INIT=%%I"
    if exist "!MYSQL_INIT!" (
        for %%D in ("!MYSQL_INIT!\..\..") do set "MARIADB_BASE=%%~fD"
        "!MYSQL_INIT!" "--datadir=!MARIADB_BASE!\data" >nul 2>&1
    )
) else (
    echo [Step 1/8] Clean runtimes environment detected. Skipping unpack.
)

:: Ensure php.ini exists even if runtimes directory was pre-extracted
if not exist "%ROOT%\runtimes\php\php.ini" (
    if exist "%ROOT%\runtimes\php\php.ini-development" (
        copy /Y "%ROOT%\runtimes\php\php.ini-development" "%ROOT%\runtimes\php\php.ini" >nul
        powershell -NoProfile -Command "$c = Get-Content '%ROOT%\runtimes\php\php.ini'; $c = $c -replace ';extension_dir = \"ext\"', 'extension_dir = \"ext\"' -replace '; extension_dir = \"ext\"', 'extension_dir = \"ext\"' -replace ';extension=mysqli', 'extension=mysqli' -replace ';extension=pdo_mysql', 'extension=pdo_mysql' -replace ';extension=mbstring', 'extension=mbstring' -replace ';extension=curl', 'extension=curl' -replace ';extension=gd', 'extension=gd' -replace ';extension=zip', 'extension=zip' -replace 'memory_limit = 128M', 'memory_limit = 1024M' -replace 'post_max_size = 8M', 'post_max_size = 256M' -replace 'upload_max_filesize = 2M', 'upload_max_filesize = 256M'; Set-Content '%ROOT%\runtimes\php\php.ini' $c" >nul 2>&1
    )
)

:: Ensure phpMyAdmin config.inc.php exists for 1-click passwordless auto-login
set "PMA_INDEX="
for /f "delims=" %%I in ('dir /b /s "%ROOT%\runtimes\phpmyadmin\index.php" 2^>nul') do set "PMA_INDEX=%%I"
if exist "!PMA_INDEX!" (
    for %%D in ("!PMA_INDEX!\..") do set "PMA_DIR=%%~fD"
    if not exist "!PMA_DIR!\config.inc.php" (
        (
            echo ^<?php
            echo /* phpMyAdmin Configuration for V-PreCode */
            echo $cfg['blowfish_secret'] = 'vprecode_secret_cookie_key_32bytes_long^!';
            echo $i = 0;
            echo $i++;
            echo $cfg['Servers'][$i]['auth_type'] = 'config';
            echo $cfg['Servers'][$i]['user'] = 'root';
            echo $cfg['Servers'][$i]['password'] = '';
            echo $cfg['Servers'][$i]['host'] = '127.0.0.1';
            echo $cfg['Servers'][$i]['compress'] = false;
            echo $cfg['Servers'][$i]['AllowNoPassword'] = true;
        ) > "!PMA_DIR!\config.inc.php"
    )
)

:: Step 7: Compile Control Panel Executable using extracted Go compiler
if not exist "%ROOT%\DevEnvManager.exe" (
    echo [Step 7/8] Compiling DevEnvManager.exe using extracted Go compiler...
    set "GO_BIN="
    for /f "delims=" %%I in ('dir /b /s "%ROOT%\runtimes\go\go.exe" 2^>nul') do set "GO_BIN=%%I"
    if not exist "!GO_BIN!" set "GO_BIN=go"

    cd /d "%ROOT%\manager-src"
    "!GO_BIN!" build -buildvcs=false -ldflags="-s -w -H windowsgui" -o "%ROOT%\DevEnvManager.exe"
    cd /d "%ROOT%"
)

:: Step 8: Launch GUI
echo [Step 8/8] Launching V-PreCode Manager...
start "" "%ROOT%\DevEnvManager.exe"

echo ===================================================
echo             Setup Completed Successfully!
echo ===================================================
goto :EOF

:: Helper Routine: Strictly Synchronous High-Speed Extraction
:ExtractArchive
set "ARCHIVE_FILE=%~1"
set "TARGET_DIR=%~2"

if "!EXTRACTOR_TYPE!"=="WindowsTar" (
    "%EXTRACTOR_EXE%" -xf "%ARCHIVE_FILE%" -C "%TARGET_DIR%"
) else if "!EXTRACTOR_TYPE!"=="7-Zip" (
    "%EXTRACTOR_EXE%" x -y "%ARCHIVE_FILE%" -o"%TARGET_DIR%" >nul
) else if "!EXTRACTOR_TYPE!"=="WinRAR" (
    start /wait "" "%EXTRACTOR_EXE%" x -o+ -ibck "%ARCHIVE_FILE%" "%TARGET_DIR%\"
) else (
    powershell -NoProfile -Command "Expand-Archive -Path '%ARCHIVE_FILE%' -DestinationPath '%TARGET_DIR%' -Force"
)
goto :EOF
