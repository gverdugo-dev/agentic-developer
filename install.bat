@echo off
REM adev installer for Windows.
REM Downloads the latest prebuilt binary for your architecture from GitHub
REM Releases and installs it. No Go toolchain required. Uses curl and tar,
REM which ship with Windows 10/11.
REM
REM   curl -fsSL -o install.bat https://raw.githubusercontent.com/gverdugo-dev/agentic-developer/main/install.bat ^&^& install.bat
setlocal enableextensions

set "REPO=gverdugo-dev/agentic-developer"

REM --- detect architecture ---
set "arch=amd64"
if /i "%PROCESSOR_ARCHITECTURE%"=="ARM64" set "arch=arm64"

set "asset=adev_windows_%arch%.zip"
set "url=https://github.com/%REPO%/releases/latest/download/%asset%"

REM --- choose install dir ---
set "dir=%LOCALAPPDATA%\adev\bin"
if not "%ADEV_INSTALL_DIR%"=="" set "dir=%ADEV_INSTALL_DIR%"
if not exist "%dir%" mkdir "%dir%"

set "tmp=%TEMP%\adev-install"
if not exist "%tmp%" mkdir "%tmp%"

echo adev: downloading %asset% ...
curl -fsSL "%url%" -o "%tmp%\%asset%"
if errorlevel 1 (
	echo adev: download failed.>&2
	exit /b 1
)

tar -xf "%tmp%\%asset%" -C "%tmp%"
if errorlevel 1 (
	echo adev: extract failed.>&2
	exit /b 1
)

move /y "%tmp%\adev.exe" "%dir%\adev.exe" >nul
echo adev: installed to %dir%\adev.exe

echo %PATH% | find /i "%dir%" >nul
if errorlevel 1 (
	echo adev: NOTE - %dir% is not on your PATH. Add it via System Properties ^> Environment Variables, or run:
	echo   setx PATH "%%PATH%%;%dir%"
)

echo adev: done. Run 'adev setup' to install adev's skills into your harness.
