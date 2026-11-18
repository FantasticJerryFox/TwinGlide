$ErrorActionPreference='Stop'
$target=Join-Path $env:LOCALAPPDATA 'Programs\TwinGlide'
if(Get-Process TwinGlide -ErrorAction SilentlyContinue){throw 'Please exit TwinGlide before installing. Other sharing apps are not changed.'}
New-Item -ItemType Directory -Path $target -Force | Out-Null
Get-ChildItem -LiteralPath $PSScriptRoot | Copy-Item -Destination $target -Recurse -Force
$shell=New-Object -ComObject WScript.Shell
$menu=Join-Path ([Environment]::GetFolderPath('Programs')) 'TwinGlide.lnk'
$shortcut=$shell.CreateShortcut($menu);$shortcut.TargetPath=Join-Path $target 'TwinGlide.exe';$shortcut.WorkingDirectory=$target;$shortcut.Save()
$startup=Join-Path ([Environment]::GetFolderPath('Startup')) 'TwinGlide.lnk'
$statePath=Join-Path $env:LOCALAPPDATA 'TwinGlide\state.json'
$startAtLogin=$true
if(Test-Path -LiteralPath $statePath){$state=Get-Content -LiteralPath $statePath -Raw | ConvertFrom-Json;if($null -ne $state.preferences -and $state.preferences.startAtLogin -eq $false){$startAtLogin=$false}}
if($startAtLogin){$shortcut=$shell.CreateShortcut($startup);$shortcut.TargetPath=Join-Path $target 'TwinGlide.exe';$shortcut.Arguments='--background';$shortcut.WorkingDirectory=$target;$shortcut.Save()}
elseif(Test-Path -LiteralPath $startup){Remove-Item -LiteralPath $startup}
$sid=[Security.Principal.WindowsIdentity]::GetCurrent().User.Value
$data=Join-Path $env:LOCALAPPDATA 'TwinGlide'
# Firewall elevation affects only TwinGlide's three LAN ports and exact binaries.
$arguments=@('-NoProfile','-ExecutionPolicy','Bypass','-File',('"'+(Join-Path $target 'Firewall.ps1')+'"'),'-AppPath',('"'+$target+'"'),'-DataPath',('"'+$data+'"'),'-OwnerSid',$sid)
$process=Start-Process powershell.exe -WindowStyle Hidden -Verb RunAs -ArgumentList $arguments -PassThru -Wait
if($process.ExitCode -ne 0){throw 'Firewall setup did not complete. Run the installer again to retry.'}
# This is the interactive settings application, not a background helper.
Start-Process -FilePath (Join-Path $target 'TwinGlide.exe')
Write-Output 'TwinGlide installed. Pairing is confirmed separately on both computers.'
