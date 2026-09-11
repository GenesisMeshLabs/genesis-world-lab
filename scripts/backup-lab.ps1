[CmdletBinding()]
param([switch]$Restart)
. "$PSScriptRoot/common.ps1"
& "$PSScriptRoot/stop-lab.ps1"
$destination=Join-Path $LabRoot ('backups/'+(Get-Date -Format yyyyMMdd-HHmmss))
New-Item -ItemType Directory -Path $destination|Out-Null
Protect-LabDirectory $destination
foreach($name in @('world','state','credentials')){Copy-Item -LiteralPath "$LabRoot/$name" -Destination "$destination/$name" -Recurse}
Copy-Item -LiteralPath "$LabRoot/config.json","$LabRoot/server.conf" -Destination $destination
$files=@(Get-ChildItem $destination -File -Recurse|ForEach-Object {@{path=$_.FullName.Substring($destination.Length+1);sha256=(Get-FileHash -LiteralPath $_.FullName).Hash}})
[IO.File]::WriteAllText("$destination/manifest.json",(@{created_at=(Get-Date).ToUniversalTime().ToString('o');files=$files}|ConvertTo-Json -Depth 5))
Write-Output "Private consistent backup: $destination"
if($Restart){& "$PSScriptRoot/start-lab.ps1"}
