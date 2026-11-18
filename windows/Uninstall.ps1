$ErrorActionPreference='Stop'
if(Get-Process TwinGlide -ErrorAction SilentlyContinue){throw 'Exit TwinGlide first.'}
$target=[IO.Path]::GetFullPath((Join-Path $env:LOCALAPPDATA 'Programs\TwinGlide'))
if([IO.Path]::GetFullPath($PSScriptRoot) -ne $target){throw 'Run the uninstaller from the installed TwinGlide directory.'}
$sid=[Security.Principal.WindowsIdentity]::GetCurrent().User.Value
$arguments=@('-NoProfile','-ExecutionPolicy','Bypass','-File',('"'+(Join-Path $target 'Firewall.ps1')+'"'),'-AppPath',('"'+$target+'"'),'-DataPath',('"'+(Join-Path $env:LOCALAPPDATA 'TwinGlide')+'"'),'-OwnerSid',$sid,'-Remove')
$process=Start-Process powershell.exe -WindowStyle Hidden -Verb RunAs -ArgumentList $arguments -PassThru -Wait
if($process.ExitCode -ne 0){throw 'Could not remove TwinGlide firewall rules.'}
foreach($folder in @('Programs','Startup')){
 $link=Join-Path ([Environment]::GetFolderPath($folder)) 'TwinGlide.lnk'
 if(Test-Path -LiteralPath $link){Remove-Item -LiteralPath $link}
}
# Resolve and verify the single fixed installation path before recursive removal.
if((Split-Path $target -Leaf) -ne 'TwinGlide' -or (Split-Path $target -Parent) -ne [IO.Path]::GetFullPath((Join-Path $env:LOCALAPPDATA 'Programs'))){throw 'Unsafe uninstall target'}
Remove-Item -LiteralPath $target -Recurse -Force
Write-Output 'TwinGlide removed. Pairing data is retained in LocalAppData\TwinGlide for reinstall.'
