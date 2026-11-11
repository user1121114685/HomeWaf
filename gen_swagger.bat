@REM HomeWaf modification (2026-11-11): local reproducible Swagger generation.
@echo off
setlocal
chcp 65001 >nul
cd /d "%~dp0"
set "SWAG_EXE="
if exist ".cache\bin\swag.exe" set "SWAG_EXE=%CD%\.cache\bin\swag.exe"
if not defined SWAG_EXE if exist "%GOPATH%\bin\swag.exe" set "SWAG_EXE=%GOPATH%\bin\swag.exe"
if not defined SWAG_EXE if exist "%USERPROFILE%\go\bin\swag.exe" set "SWAG_EXE=%USERPROFILE%\go\bin\swag.exe"
if not defined SWAG_EXE for %%I in (swag.exe) do set "SWAG_EXE=%%~$PATH:I"
if not defined SWAG_EXE (
    echo Install the documentation tool: go install github.com/swaggo/swag/cmd/swag@v1.16.6
    exit /b 1
)
"%SWAG_EXE%" init -g cmd/homewaf/main.go -o docs/openapi --outputTypes json --parseDependency --parseInternal
if errorlevel 1 exit /b 1
node scripts/verify-pipeline-openapi.mjs
if errorlevel 1 exit /b 1
echo Generated docs/openapi/swagger.json. No external copy or running service required.
