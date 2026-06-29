@echo off
REM Preflight for adev-plugin-marketplace-builder: verify the skill's requirements
REM exist before any work. Exits non-zero if a required tool is missing; git is a
REM soft recommendation (warning only), needed only at the hosting step.

set "missing=0"

where adev >nul 2>nul
if %errorlevel% neq 0 (
	echo ERROR: 'adev' CLI not found on PATH. Install it: https://github.com/gverdugo-dev/agentic-developer>&2
	set "missing=1"
)

where git >nul 2>nul
if %errorlevel% neq 0 (
	echo WARNING: git not found - you'll need it to host the marketplace on a remote (GitHub/GitLab).>&2
)

if "%missing%" neq "0" (
	echo Preflight failed: fix the items above, then re-run.>&2
	exit /b 1
)

echo Preflight OK: adev is available.>&2
