[CmdletBinding()]
param([Parameter(Mandatory)][string]$BackupPath,[Parameter(Mandatory)][string]$Destination,[int]$BridgePort=28789,[int]$GamePort=30001)
. "$PSScriptRoot/common.ps1"
$source=(Resolve-Path -LiteralPath $BackupPath).Path
$target=[IO.Path]::GetFullPath($Destination)
if(-not $target.StartsWith([IO.Path]::GetFullPath($LabRoot)+'\',[StringComparison]::OrdinalIgnoreCase)){throw 'Restore target must be a new directory within this repository .local folder'}
if(Test-Path -LiteralPath $target){throw 'Restore never overwrites an existing directory'}
if($BridgePort -lt 1024 -or $BridgePort -gt 65535 -or $GamePort -lt 1024 -or $GamePort -gt 65535){throw 'Invalid restore ports'}
$manifest=Get-Content -LiteralPath "$source/manifest.json" -Raw|ConvertFrom-Json
if(Get-ChildItem -LiteralPath $source -Force -Recurse|Where-Object {$_.Attributes -band [IO.FileAttributes]::ReparsePoint}){throw 'Backup must not contain links'}
foreach($file in $manifest.files){
 $path=[IO.Path]::GetFullPath((Join-Path $source $file.path))
 if(-not $path.StartsWith($source+'\',[StringComparison]::OrdinalIgnoreCase)){throw 'Backup path escapes its root'}
 if((Get-FileHash -LiteralPath $path).Hash -ne $file.sha256){throw "Backup hash mismatch: $($file.path)"}
}
New-Item -ItemType Directory -Path $target|Out-Null
Protect-LabDirectory $target
foreach($file in $manifest.files){$path=Join-Path $target $file.path;New-Item -ItemType Directory -Force (Split-Path $path -Parent)|Out-Null;Copy-Item -LiteralPath (Join-Path $source $file.path) -Destination $path}
$c=Get-Content "$target/config.json" -Raw|ConvertFrom-Json
$c.address="127.0.0.1:$BridgePort";$c.state_file="$target/state/bridge.bolt"
foreach($a in $c.authorities){
 $a.root_file=Join-Path "$target/credentials" ([IO.Path]::GetFileName($a.root_file))
 $operatorKey=Join-Path "$target/credentials" ([IO.Path]::GetFileName($a.operator_key_file))
 if(Test-Path -LiteralPath $operatorKey){$a.operator_key_file=$operatorKey}
}
foreach($p in $c.players){$p.identity_file=Join-Path "$target/credentials" ([IO.Path]::GetFileName($p.identity_file))}
[IO.File]::WriteAllText("$target/config.json",($c|ConvertTo-Json -Depth 8))
$server=[IO.File]::ReadAllText("$target/server.conf") -replace '(?m)^port\s*=.*$',"port = $GamePort" -replace '(?m)^genesismesh.bridge_url\s*=.*$',"genesismesh.bridge_url = http://127.0.0.1:$BridgePort"
[IO.File]::WriteAllText("$target/server.conf",$server)
Write-Output "Verified restore created at $target. Start it explicitly with its config and world paths; original deployment is unchanged."
