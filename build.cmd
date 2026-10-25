@echo off
setlocal
cd /d "%~dp0"
set CGO_ENABLED=0
if not exist bin mkdir bin
if errorlevel 1 exit /b 1
go build -trimpath -ldflags "-s -w" -o bin\alx-tui.exe .
if errorlevel 1 exit /b 1
if not exist "..\Lx-Cli\bin\Lx-Cli.exe" goto done
copy /y /b "..\Lx-Cli\bin\Lx-Cli.exe" "bin\Lx-Cli.exe" >nul
if errorlevel 1 exit /b 1
:done
echo Built bin\alx-tui.exe. Place Lx-Cli.exe beside it to run.
