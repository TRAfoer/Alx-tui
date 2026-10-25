@echo off
setlocal
cd /d "%~dp0"
if not exist "bin\alx-tui.exe" goto missing
if not exist "bin\Lx-Cli.exe" goto missing
"%~dp0bin\alx-tui.exe" %*
exit /b %errorlevel%
:missing
echo Build alx-tui and place Lx-Cli.exe in bin first.
exit /b 1
