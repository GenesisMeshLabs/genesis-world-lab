[CmdletBinding()]
param()
$ErrorActionPreference='Stop'
$repo=Split-Path $PSScriptRoot -Parent
$local=Join-Path $repo '.local'
$base='http://127.0.0.1:8789'
$game=[IO.File]::ReadAllText("$local/credentials/game.token").Trim()
$north=[IO.File]::ReadAllText("$local/credentials/authority-a.token").Trim()
$south=[IO.File]::ReadAllText("$local/credentials/authority-b.token").Trim()
$results=[Collections.Generic.List[object]]::new()
function Api($path,$body,$token){$args=@{Uri="$base$path";Headers=@{Authorization="Bearer $token"};TimeoutSec=15};if($null -ne $body){$args.Method='Post';$args.ContentType='application/json';$args.Body=($body|ConvertTo-Json -Depth 8)};Invoke-RestMethod @args}
function Probe($action,$player='alice'){
 $id=[Guid]::NewGuid().ToString();$q=@{id=$id;action=$action;player=$player;position=@{x=24;y=7;z=0}}
 [IO.File]::WriteAllText("$local/world/.private/probe.json",($q|ConvertTo-Json -Depth 4))
 for($i=0;$i -lt 30;$i++){Start-Sleep -Milliseconds 200;if(Test-Path "$local/world/.private/probe-result.json"){$r=Get-Content "$local/world/.private/probe-result.json" -Raw|ConvertFrom-Json;if($r.id -eq $id){return $r}}};throw 'Engine probe timed out (install tests/engine in acceptance world)'
}
function Check($name,$ok){$results.Add(@{name=$name;pass=[bool]$ok});if(-not $ok){throw "Acceptance failed: $name"};Write-Output "PASS $name"}
function Grant($player,$cap,$ttl,$token){Api '/v1/delegate' @{player_name=$player;capability=$cap;area='demo-area';ttl_seconds=$ttl} $token}
function Revoke($g,$token){$null=Api '/v1/revoke' @{grant_id=$g.id} $token}
try {
 $r=Probe 'sample';Check 'two authenticated real clients connected' (($r.connected -contains 'alice') -and ($r.connected -contains 'bob'))
 $r=Probe 'move';Check 'ungranted player ejected to lobby' ($r.position.x -lt 20 -and $r.protected)
 $r=Probe 'place';Check 'ungranted block placement denied' ($r.before -eq $r.after)
 $entry=Grant 'alice' 'region.demo.enter' 60 $north
 $build=Grant 'alice' 'region.demo.build' 60 $north
 Start-Sleep -Seconds 2
 $r=Probe 'move';Check 'signed grant permits entry' ($r.entry -and $r.position.x -ge 20)
 $r=Probe 'dig';Check 'signed grant permits clearing the test block' ($r.after -eq 'air' -and -not $r.protected)
 $r=Probe 'place';Check 'signed grant permits real block placement' ($r.before -eq 'air' -and $r.after -eq 'mcl_core:stone' -and -not $r.protected)
 Revoke $build $north;Start-Sleep -Seconds 2
 $r=Probe 'dig';Check 'revoked building right prevents real block removal' ($r.protected -and $r.before -eq $r.after)
 Revoke $entry $north;Start-Sleep -Seconds 2
 $r=Probe 'move';Check 'revoked entry right ejects player' (-not $r.entry -and $r.position.x -lt 20)
 $temporary=Grant 'bob' 'region.demo.enter' 4 $south
 Start-Sleep -Seconds 1
 $r=Probe 'move' 'bob';Check 'second independent authority permits its player' ($r.entry -and $r.position.x -ge 20)
 Start-Sleep -Seconds 4
 $r=Probe 'move' 'bob';Check 'expiry ejects player without operator intervention' (-not $r.entry -and $r.position.x -lt 20)
 $outage=Grant 'alice' 'region.demo.enter' 60 $north
 Start-Sleep -Seconds 2
 $r=Probe 'move';Check 'outage test starts with a valid lease' $r.entry
 $pidValue=[int](Get-Content "$local/bridge.pid");$p=Get-CimInstance Win32_Process -Filter "ProcessId=$pidValue"
 if($p.ExecutablePath -ne "$repo\.local\bridge.exe"){throw 'Bridge process ownership mismatch'}
 Stop-Process -Id $pidValue
 Start-Sleep -Seconds 3
 $r=Probe 'move';Check 'bridge outage expires cache and ejects player' (-not $r.entry -and $r.protected -and $r.position.x -lt 20)
 $p=Start-Process -FilePath "$local/bridge.exe" -ArgumentList "-config $local/config.json" -WindowStyle Hidden -RedirectStandardOutput "$local/logs/bridge.out.log" -RedirectStandardError "$local/logs/bridge.err.log" -PassThru;$p.Id|Set-Content "$local/bridge.pid"
 for($i=0;$i -lt 20;$i++){try{if((Invoke-RestMethod "$base/readyz" -TimeoutSec 2).ready){break}}catch{};Start-Sleep -Seconds 1}
 $status=Api '/v1/status' $null $north
 $old=@($status.grants|Where-Object {$_.id -in @($entry.id,$build.id)})
 Check 'revocations survive bridge restart' ($old.Count -eq 2 -and @($old|Where-Object {-not $_.revoked}).Count -eq 0)
 Revoke $outage $north
 $events=Api '/v1/audit?after=0' $null $north
 Check 'durable audit includes grants, revocation and expiry' (($events.action -contains 'capability.delegate') -and ($events.action -contains 'capability.revoke') -and ($events.action -contains 'capability.expire'))
}finally{
 [IO.File]::WriteAllText("$local/acceptance.json",(@{checked_at=(Get-Date).ToUniversalTime().ToString('o');results=$results.ToArray()}|ConvertTo-Json -Depth 6))
}
