param([Parameter(Mandatory=$true)][string]$AppPath,[Parameter(Mandatory=$true)][string]$DataPath,[Parameter(Mandatory=$true)][string]$OwnerSid,[switch]$Remove)
$ErrorActionPreference='Stop'
if($OwnerSid -notmatch '^S-1-5-21-[0-9-]+$'){throw 'Invalid user SID'}
$group='TwinGlide-'+$OwnerSid
if($Remove){Get-NetFirewallRule -Group $group -ErrorAction SilentlyContinue | Remove-NetFirewallRule;exit}
$agent=Join-Path $AppPath 'twinglide-agent.exe'
$inputEnginePath=Join-Path $DataPath 'runtime\engine\deskflow-core.exe'
if(!(Test-Path -LiteralPath $agent)){throw 'TwinGlide agent missing'}
Get-NetFirewallRule -Group $group -ErrorAction SilentlyContinue | Remove-NetFirewallRule
foreach($entry in @(@('Discovery','UDP',59120,$agent),@('Pairing','TCP',59121,$agent),@('Input','TCP',59122,$inputEnginePath))){
 New-NetFirewallRule -DisplayName ('TwinGlide '+$entry[0]+' '+$OwnerSid) -Group $group -Direction Inbound -Action Allow -Protocol $entry[1] -LocalPort $entry[2] -Program $entry[3] -RemoteAddress LocalSubnet -Profile Any | Out-Null
}
