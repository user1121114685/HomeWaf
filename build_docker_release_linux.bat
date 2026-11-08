@REM HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

@echo off
cd /d "%~dp0"
docker build -t homewaf:local .
exit /b %errorlevel%
