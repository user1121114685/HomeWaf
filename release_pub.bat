@REM HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

@echo off
cd /d "%~dp0"
node scripts/build.mjs
exit /b %errorlevel%
