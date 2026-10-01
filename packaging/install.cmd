@echo off
rem Installs or updates Plips and puts a Plips icon on your desktop.
set "DEST=%LOCALAPPDATA%\Plips"
if not exist "%DEST%" mkdir "%DEST%"
rem Close a running copy first, or Windows won't let us replace it.
taskkill /im plips.exe /f >nul 2>&1
timeout /t 1 /nobreak >nul
copy /y "%~dp0plips.exe" "%DEST%\plips.exe" >nul
if errorlevel 1 (
  echo.
  echo Could not update Plips. Close it and run this again.
  pause
  exit /b 1
)
powershell -NoProfile -Command "$s=(New-Object -ComObject WScript.Shell).CreateShortcut([Environment]::GetFolderPath('Desktop')+'\Plips.lnk'); $s.TargetPath='%DEST%\plips.exe'; $s.WorkingDirectory='%DEST%'; $s.Save()"
echo Plips is installed and up to date.
start "" "%DEST%\plips.exe"
