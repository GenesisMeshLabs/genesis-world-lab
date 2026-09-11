[CmdletBinding()]
param()
. "$PSScriptRoot/common.ps1"
New-Item -ItemType Directory -Force "$LabRoot/downloads","$LabRoot/runtime","$LabRoot/logs","$LabRoot/state","$LabRoot/credentials"|Out-Null
Protect-LabDirectory $LabRoot
if(Get-LabProcess 'bridge' "$LabRoot/bridge.exe"){throw 'Stop the lab before rebuilding: scripts/stop-lab.ps1'}
function Download-Verified($URL,$Path,$Hash){
 if(-not(Test-Path -LiteralPath $Path)){Invoke-WebRequest -UseBasicParsing $URL -OutFile $Path}
 if((Get-FileHash -LiteralPath $Path).Hash -ne $Hash){throw "Download hash mismatch: $Path"}
}
Download-Verified 'https://github.com/luanti-org/luanti/releases/download/5.17.0/luanti-5.17.0-win64.zip' "$LabRoot/downloads/luanti.zip" '3ce20c77f5c206a988d7a6b883439e2759e3cf67428c9dbcf99ecd2936da631c'
Download-Verified 'https://content.luanti.org/packages/ryvnf/mineclonia/releases/38561/download/' "$LabRoot/downloads/mineclonia.zip" 'abeacf4e202d6e01a63119a36b46751eb6abfd42948a0b06e1ba8f63fb849e59'
if(-not(Test-Path -LiteralPath $LuantiExe)){Expand-Archive "$LabRoot/downloads/luanti.zip" "$LabRoot/runtime"}
if(-not(Test-Path "$LabRoot/runtime/games/mineclonia/game.conf")){Expand-Archive "$LabRoot/downloads/mineclonia.zip" "$LabRoot/runtime/games"}
$gamePath=Join-Path (Split-Path (Split-Path $LuantiExe -Parent) -Parent) 'games/mineclonia'
New-Item -ItemType Directory -Force (Split-Path $gamePath -Parent)|Out-Null
if(-not(Test-Path $gamePath)){New-Item -ItemType Junction -Path $gamePath -Target "$LabRoot/runtime/games/mineclonia"|Out-Null}
go -C "$RepoRoot/bridge" build -trimpath -o "$LabRoot/bridge.exe" .
if($LASTEXITCODE -ne 0){throw 'Go build failed'}
if(-not(Test-Path "$LabRoot/config.json")){& "$PSScriptRoot/configure-local.ps1"}
& "$LabRoot/bridge.exe" -config "$LabRoot/config.json" -provision
if($LASTEXITCODE -ne 0){throw 'Authority onboarding failed'}
New-Item -ItemType Directory -Force "$LabRoot/world/worldmods/genesismesh","$LabRoot/world/.private"|Out-Null
Copy-Item "$RepoRoot/mods/genesismesh/*" "$LabRoot/world/worldmods/genesismesh/" -Force
$accounts=@{}
foreach($name in @('alice','bob','north','south')){$accounts[$name]=@{password=[IO.File]::ReadAllText("$LabRoot/credentials/$name.password").Trim();operator=($name -in @('north','south'))}}
[IO.File]::WriteAllText("$LabRoot/world/.private/accounts.json",($accounts|ConvertTo-Json -Depth 4))
$config=[IO.File]::ReadAllText("$RepoRoot/server/minetest.conf")
$token=[IO.File]::ReadAllText("$LabRoot/credentials/game.token").Trim()
[IO.File]::WriteAllText("$LabRoot/server.conf",$config+"`ngenesismesh.game_token = $token`n")
if(-not(Test-Path "$LabRoot/world/world.mt")){
 [IO.File]::WriteAllText("$LabRoot/world/world.mt","gameid = mineclonia`nbackend = sqlite3`nplayer_backend = sqlite3`nauth_backend = sqlite3`nmod_storage_backend = sqlite3`nload_mod_genesismesh = true`n")
}
Write-Output 'Native runtime and signed identities ready. Run scripts/start-lab.ps1 -Player alice'
