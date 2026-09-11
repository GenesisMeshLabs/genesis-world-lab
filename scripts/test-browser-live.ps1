[CmdletBinding()]
param()
. "$PSScriptRoot/common.ps1"
$base='http://127.0.0.1:8789'
$password=[IO.File]::ReadAllText("$LabRoot/credentials/alice.password").Trim()
$login=Invoke-RestMethod "$base/play/api/login" -Method Post -ContentType application/json -Body (@{player='alice';password=$password}|ConvertTo-Json)
$headers=@{Authorization="Bearer $($login.token)"}
$operator=[IO.File]::ReadAllText("$LabRoot/credentials/authority-a.token").Trim()
$operatorHeaders=@{Authorization="Bearer $operator"}
$results=[Collections.Generic.List[object]]::new()
$grants=@()
function WebState {Invoke-RestMethod "$base/play/api/state" -Headers $headers}
function Check($name,$ok){$results.Add(@{name=$name;pass=[bool]$ok});if(-not $ok){throw "Browser acceptance failed: $name"};Write-Output "PASS $name"}
function Command($action,$x=0,$z=0,$material=''){
 $body=@{action=$action;x=$x;z=$z;material=$material}|ConvertTo-Json
 $q=Invoke-RestMethod "$base/play/api/command" -Method Post -Headers $headers -ContentType application/json -Body $body
 for($i=0;$i -lt 25;$i++){Start-Sleep -Milliseconds 120;$s=WebState;$result=$s.frame.results|Where-Object id -eq $q.id;if($result){return $result}}
 throw 'Engine did not acknowledge browser command'
}
function Grant($cap){Invoke-RestMethod "$base/v1/delegate" -Method Post -Headers $operatorHeaders -ContentType application/json -Body (@{player_name='alice';capability=$cap;area='demo-area';ttl_seconds=60}|ConvertTo-Json)}
function Revoke($g){Invoke-RestMethod "$base/v1/revoke" -Method Post -Headers $operatorHeaders -ContentType application/json -Body (@{grant_id=$g.id}|ConvertTo-Json)|Out-Null}
try {
 $s=WebState;Check 'live world frame and authenticated player' ($s.ready -and $s.engine_online -and $s.player -eq 'alice' -and $s.frame.layers.Count -eq 4)
 $null=Command 'lobby';Start-Sleep -Milliseconds 400
 $s=WebState;$before=($s.frame.players|Where-Object name -eq 'alice').x
 $r=Command 'move' 1 0;Start-Sleep -Milliseconds 300;$s=WebState
 Check 'browser move changes actual Luanti position' ($r.ok -and ($s.frame.players|Where-Object name -eq 'alice').x -gt $before)
 # A nearby clear tile in the commons. Do not overwrite existing world content.
 $tile=$null;foreach($x in 2..4){$idx=12*49+$x+8;if($s.frame.layers[1][$idx] -eq 'a'){$tile=$x;break}}
 if($null -eq $tile){throw 'No empty test tile near lobby'}
 $r=Command 'place' $tile 0 'wood';Start-Sleep -Milliseconds 300;$s=WebState
 Check 'browser placement updates real world blocks' ($r.ok -and $s.frame.layers[1][12*49+$tile+8] -eq 'w')
 $r=Command 'dig' $tile 0;Check 'browser removal uses normal engine digging' $r.ok
 for($i=0;$i -lt 25;$i++){$r=Command 'move' 1 0;if(-not $r.ok){break}}
 $s=WebState;Check 'court entry denied without signed grant' (-not $r.ok -and $r.message -like 'Entry denied*' -and ($s.frame.players|Where-Object name -eq 'alice').x -lt 20)
 $r=Command 'place' 21 0 'stone';Check 'court building denied without grant' (-not $r.ok)
 $entry=Grant 'region.demo.enter';$grants+=$entry;$build=Grant 'region.demo.build';$grants+=$build
 Start-Sleep -Seconds 2
 $r=Command 'move' 1 0;Check 'signed grant allows browser court entry' $r.ok
 $s=WebState;$courtX=$null;foreach($x in 22..24){if($s.frame.layers[1][12*49+$x+8] -eq 'a'){$courtX=$x;break}}
 if($null -eq $courtX){throw 'No empty test tile in court'}
 $r=Command 'place' $courtX 0 'stone';Check 'signed grant allows browser court building' $r.ok
 Revoke $build;Start-Sleep -Seconds 2
 $r=Command 'dig' $courtX 0;Check 'revocation blocks browser removal' (-not $r.ok)
 # Clean up only the block this acceptance run created, with a fresh signed grant.
 $cleanup=Grant 'region.demo.build';$grants+=$cleanup;Start-Sleep -Seconds 2
 $r=Command 'dig' $courtX 0;Check 'fresh authorized grant permits test cleanup' $r.ok
 Revoke $entry;Start-Sleep -Seconds 2
 $s=WebState;Check 'revocation ejects browser-controlled player' (($s.frame.players|Where-Object name -eq 'alice').x -lt 20)
} finally {
 foreach($g in $grants){try{Revoke $g}catch{Write-Warning "Revoke cleanup failed for grant $($g.id)"}}
 [IO.File]::WriteAllText("$LabRoot/browser-acceptance.json",(@{checked_at=(Get-Date).ToUniversalTime().ToString('o');results=$results.ToArray()}|ConvertTo-Json -Depth 5))
}
