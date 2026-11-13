@echo off
setlocal
cd /d "%~dp0"
where node >nul 2>&1
if errorlevel 1 (
  echo Node.js is required. See README.md for the supported version.
  pause
  exit /b 1
)
where go >nul 2>&1
if errorlevel 1 (
  echo Go is required. See go.mod for the supported version.
  pause
  exit /b 1
)
node scripts/dev.mjs %*
set "DEV_RESULT=%ERRORLEVEL%"
if not "%DEV_RESULT%"=="0" pause
exit /b %DEV_RESULT%
