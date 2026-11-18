param([string]$Go='go')
$ErrorActionPreference='Stop'
$root=Split-Path $PSScriptRoot -Parent
Push-Location $root
try {
 if (!(Test-Path 'LICENSE')) { throw 'LICENSE is required before packaging.' }
 $out=Join-Path $root 'dist\TwinGlide-windows-x64'
 $cache=Join-Path $root 'engine\cache'
 New-Item -ItemType Directory -Force -Path $out,$cache | Out-Null
 & $Go test ./...
 if($LASTEXITCODE -ne 0){throw 'Tests failed'}
 & $Go build -trimpath -ldflags '-s -w' -o (Join-Path $out 'twinglide-agent.exe') ./cmd/twinglide
 if($LASTEXITCODE -ne 0){throw 'Go build failed'}
 $csc=Join-Path $env:WINDIR 'Microsoft.NET\Framework64\v4.0.30319\csc.exe'
 & $csc /nologo /target:winexe /platform:x64 /optimize+ ('/out:'+(Join-Path $out 'TwinGlide.exe')) ('/win32icon:'+(Join-Path $root 'assets\TwinGlide.ico')) ('/win32manifest:'+(Join-Path $root 'windows\app.manifest')) /reference:System.Windows.Forms.dll /reference:System.Drawing.dll /reference:System.Web.Extensions.dll (Join-Path $root 'windows\Launcher.cs')
 if($LASTEXITCODE -ne 0){throw 'Launcher build failed'}
 Copy-Item -LiteralPath 'windows\TwinGlide.exe.config' -Destination $out -Force
 & $csc /nologo /target:exe /platform:x64 /optimize+ ('/out:'+(Join-Path $out 'TwinGlideAudio.exe')) (Join-Path $root 'windows\Audio.cs')
 if($LASTEXITCODE -ne 0){throw 'Audio helper build failed'}
 & (Join-Path $out 'TwinGlideAudio.exe') --self-test
 if($LASTEXITCODE -ne 0){throw 'Audio self-test failed'}
 $pin=(Get-Content 'engine\pins.json' -Raw | ConvertFrom-Json).'windows-amd64'
 $archive=Join-Path $cache $pin.asset
 if(!(Test-Path $archive)){Invoke-WebRequest -Uri ('https://github.com/deskflow/deskflow/releases/download/v'+$pin.version+'/'+$pin.asset) -OutFile $archive}
 if((Get-FileHash -LiteralPath $archive -Algorithm SHA256).Hash.ToLowerInvariant() -ne $pin.sha256){throw 'Deskflow archive checksum mismatch'}
 $extract=Join-Path $cache ('windows-'+$pin.version)
 Expand-Archive -LiteralPath $archive -DestinationPath $extract -Force
 $core=Get-ChildItem -LiteralPath $extract -Filter deskflow-core.exe -Recurse | Select-Object -First 1
 if(!$core){throw 'Deskflow executable missing from archive'}
 $engine=Join-Path $out 'engine'
 New-Item -ItemType Directory -Path $engine -Force | Out-Null
 Get-ChildItem -LiteralPath $core.DirectoryName | Where-Object { $_.Name -notin @('settings','deskflow.exe','deskflow-daemon.exe') } | Copy-Item -Destination $engine -Recurse -Force
 $hashes=[ordered]@{}
 Get-ChildItem -LiteralPath $engine -File -Recurse | Sort-Object FullName | ForEach-Object {
  if($_.Name -ne 'twinglide-engine.json'){$hashes[$_.FullName.Substring($engine.Length+1).Replace('\','/')]=(Get-FileHash -LiteralPath $_.FullName -Algorithm SHA256).Hash.ToLowerInvariant()}
 }
 $hashes | ConvertTo-Json -Depth 3 | Set-Content -LiteralPath (Join-Path $engine 'twinglide-engine.json') -Encoding ASCII
 Copy-Item -LiteralPath 'LICENSE','THIRD_PARTY_NOTICES.md','README.md','README.en.md','SECURITY.md' -Destination $out -Force
 Copy-Item -LiteralPath 'licenses','docs' -Destination $out -Recurse -Force
 Copy-Item -LiteralPath 'windows\Install.ps1','windows\Install.cmd','windows\Uninstall.ps1','windows\Firewall.ps1' -Destination $out -Force
 $zip=Join-Path $root 'dist\TwinGlide-0.1.0-alpha.5-windows-x64.zip'
 Compress-Archive -Path (Join-Path $out '*') -DestinationPath $zip -Force
 Get-FileHash -LiteralPath $zip -Algorithm SHA256
} finally {Pop-Location}
