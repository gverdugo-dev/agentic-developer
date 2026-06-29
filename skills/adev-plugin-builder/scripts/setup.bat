@echo off
REM Preflight for adev-plugin-builder: verify the skill's requirements exist
REM before any work. Exits non-zero with a clear message if something is missing.

set "missing=0"

where adev >nul 2>nul
if %errorlevel% neq 0 (
	echo ERROR: 'adev' CLI not found on PATH. Install it: https://github.com/gverdugo-dev/agentic-developer>&2
	set "missing=1"
)

if "%missing%" neq "0" (
	echo Preflight failed: fix the items above, then re-run.>&2
	exit /b 1
)

echo Preflight OK: adev is available.>&2
