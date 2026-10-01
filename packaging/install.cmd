@echo off
rem Copies Plips to your user folder and puts a Plips icon on your desktop.
set "DEST=%LOCALAPPDATA%\Plips"
if not exist "%DEST%" mkdir "%DEST%"
copy /y "%~dp0plips.exe" "%DEST%\plips.exe" >nul
powershell -NoProfile -Command "$s=(New-Object -ComObject WScript.Shell).CreateShortcut([Environment]::GetFolderPath('Desktop')+'\Plips.lnk'); $s.TargetPath='%DEST%\plips.exe'; $s.WorkingDirectory='%DEST%'; $s.Save()"
echo Plips is on your desktop.
start "" "%DEST%\plips.exe"
