[CmdletBinding()]
param([ValidateSet('alice','bob','north','south')][string]$Player,[switch]$Acceptance)
. "$PSScriptRoot/common.ps1"
if(-not(Test-Path "$LabRoot/config.json")){& "$PSScriptRoot/setup.ps1"}
$config=Get-Content "$LabRoot/config.json" -Raw|ConvertFrom-Json
if(-not(Get-LabProcess 'bridge' "$LabRoot/bridge.exe")){
 $p=Start-Process -FilePath "$LabRoot/bridge.exe" -ArgumentList "-config `"$LabRoot/config.json`"" -WindowStyle Hidden -RedirectStandardOutput "$LabRoot/logs/bridge.out.log" -RedirectStandardError "$LabRoot/logs/bridge.err.log" -PassThru
 $p.Id|Set-Content "$LabRoot/bridge.pid"
}
Wait-LabReady "http://$($config.address)"
if(-not(Get-LabProcess 'luanti' $LuantiExe)){
 Copy-Item "$RepoRoot/mods/genesismesh/*" "$LabRoot/world/worldmods/genesismesh/" -Force
 $testMod="$LabRoot/world/worldmods/genesismesh_acceptance"
 if($Acceptance){New-Item -ItemType Directory -Force $testMod|Out-Null;Copy-Item "$RepoRoot/tests/engine/*" "$testMod/" -Force}
 elseif(Test-Path $testMod){
  $resolved=[IO.Path]::GetFullPath($testMod)
  if(-not $resolved.StartsWith([IO.Path]::GetFullPath($LabRoot)+'\',[StringComparison]::OrdinalIgnoreCase)){throw 'Unexpected acceptance mod path'}
  $disabled=Join-Path $LabRoot ('acceptance-disabled-'+[Guid]::NewGuid().ToString('N'))
  Move-Item -LiteralPath $resolved -Destination $disabled
 }
 $p=Start-Process -FilePath $LuantiExe -ArgumentList "--server --config `"$LabRoot/server.conf`" --world `"$LabRoot/world`" --logfile `"$LabRoot/logs/luanti.log`"" -WindowStyle Hidden -RedirectStandardOutput "$LabRoot/logs/luanti.out.log" -RedirectStandardError "$LabRoot/logs/luanti.err.log" -PassThru
 $p.Id|Set-Content "$LabRoot/luanti.pid"
 $listening=$false
 for($i=0;$i -lt 45;$i++){if(Get-NetUDPEndpoint -LocalPort 30000 -ErrorAction SilentlyContinue|Where-Object OwningProcess -eq $p.Id){$listening=$true;break};if($p.HasExited){throw 'Luanti stopped; inspect .local/logs/luanti.log'};Start-Sleep -Seconds 1}
 if(-not $listening){throw 'Luanti did not bind its local UDP port'}
}
if($Player -and -not(Get-LabProcess "client-$Player" $LuantiExe)){
 $clientConfig="$LabRoot/client-$Player.conf"
 if(-not(Test-Path $clientConfig)){[IO.File]::WriteAllText($clientConfig,"enable_sound = false`nscreen_w = 1100`nscreen_h = 760`n")}
 $p=Start-Process -FilePath $LuantiExe -ArgumentList "--go --address 127.0.0.1 --port 30000 --name $Player --password-file `"$LabRoot/credentials/$Player.password`" --config `"$clientConfig`" --logfile `"$LabRoot/logs/client-$Player.log`"" -WindowStyle Hidden -PassThru
 $p.Id|Set-Content "$LabRoot/client-$Player.pid"
}
if(Test-Path "$LabRoot/tunnel.json"){& "$PSScriptRoot/start-tunnel.ps1"}
Write-Output "Bridge ready at http://$($config.address); Mineclonia listening at 127.0.0.1:30000 (UDP)."
