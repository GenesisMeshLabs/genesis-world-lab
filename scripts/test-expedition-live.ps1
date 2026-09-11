[CmdletBinding()]
param()
. "$PSScriptRoot/common.ps1"
$base='http://127.0.0.1:8789'
$password=[IO.File]::ReadAllText("$LabRoot/credentials/alice.password").Trim()
$login=Invoke-RestMethod "$base/play/api/login" -Method Post -ContentType application/json -Body (@{player='alice';password=$password}|ConvertTo-Json)
$headers=@{Authorization="Bearer $($login.token)"}
$results=[Collections.Generic.List[object]]::new()
function State {Invoke-RestMethod "$base/play/api/state" -Headers $headers}
function Check($name,$ok){$results.Add(@{name=$name;pass=[bool]$ok});if(-not $ok){throw "Expedition acceptance failed: $name"};Write-Output "PASS $name"}
function Experiment($action){Invoke-RestMethod "$base/play/api/experiment" -Method Post -Headers $headers -ContentType application/json -Body (@{action=$action}|ConvertTo-Json)|Out-Null}
function Command($action,$x=0,$z=0,$material=''){
 $q=Invoke-RestMethod "$base/play/api/command" -Method Post -Headers $headers -ContentType application/json -Body (@{action=$action;x=$x;z=$z;material=$material}|ConvertTo-Json)
 for($i=0;$i -lt 40;$i++){Start-Sleep -Milliseconds 150;$s=State;$r=$s.frame.results|Where-Object id -eq $q.id;if($r){return $r}}
 throw 'Native engine did not acknowledge the command'
}
$placed=$false;$tile=$null
try {
 Experiment 'identity';Experiment 'revoke';$null=Command 'lobby'
 for($i=0;$i -lt 30;$i++){$r=Command 'move' 1 0;if(-not $r.ok){break}}
 Check 'real game gate denies entry before passport' (-not $r.ok -and $r.message -like 'Entry denied*')
 Experiment 'passport';Start-Sleep -Seconds 2
 $s=State;$lease=$s.expedition.records|Where-Object status -eq active|Select-Object -Last 1
 Check 'foreign authority signs visitor passport' ($lease.issuer -ne $s.identity.authority -and $lease.capability -eq 'region.demo.enter')
 $null=Command 'move' 1 0;Start-Sleep -Milliseconds 400
 $s=State;Check 'game confirms cross-authority court entry' (($s.frame.players|Where-Object name -eq alice).x -ge 20)
 Experiment 'build';Start-Sleep -Seconds 2
 foreach($x in 22..24){if($s.frame.layers[1][12*49+$x+8] -eq 'a'){$tile=$x;break}}
 if($null -eq $tile){throw 'No empty canary tile'}
 $r=Command 'place' $tile 0 'wood';$placed=$r.ok;Check 'signed build lease changes real court block' $placed
 # Keep entry while removing the build lease by clearing all experiment leases;
 # enforcement ejects Alice to the lobby, so walk back within reach of the gate.
 Experiment 'revoke';Start-Sleep -Seconds 2
 for($i=0;$i -lt 30;$i++){$r=Command 'move' 1 0;if(-not $r.ok){break}}
 $r=Command 'dig' $tile 0;Check 'game denies editing after lease revocation' (-not $r.ok -and $r.message -like 'Build denied*')
 Experiment 'delegate';Start-Sleep -Seconds 1
 $s=State;Check 'attenuated child lease is active' (@($s.expedition.records|Where-Object {$_.parent -and $_.status -eq 'active'}).Count -gt 0)
 Experiment 'cascade';$s=State;Check 'revoked parent invalidates child lease' (@($s.expedition.records|Where-Object {$_.parent -and $_.status -eq 'active'}).Count -eq 0)
 Experiment 'scope';Experiment 'tamper';Experiment 'recover'
 $s=State;Check 'recovery and authentic rollback replay verified' ($null -ne $s.expedition.proofs.recovered -and $null -ne $s.expedition.proofs.rollback_rejected)
 # Respect the public experiment rate limit before the next lease issuance.
 Start-Sleep -Seconds 45
 Experiment 'expiry';Start-Sleep -Seconds 15
 $s=State;Check 'expiry removes permission without a revoke command' ($s.capabilities -notcontains 'region.demo.enter' -and $null -ne $s.expedition.proofs.expired)
 Check 'all thirteen proofs backed by real checks' (@($s.expedition.proofs.PSObject.Properties).Count -eq 13)
} finally {
 if($placed){try{Experiment 'passport';Experiment 'build';Start-Sleep -Seconds 2;for($i=0;$i -lt 30;$i++){$s=State;if(($s.frame.players|Where-Object name -eq alice).x -ge 20){break};$null=Command 'move' 1 0};$r=Command 'dig' $tile 0;if(-not $r.ok){Write-Warning 'Canary cleanup was denied'}}catch{Write-Warning "Canary cleanup failed: $_"}}
 try{Experiment 'revoke';$null=Command 'lobby'}catch{Write-Warning 'Lease cleanup needs retry'}
 [IO.File]::WriteAllText("$LabRoot/expedition-acceptance.json",(@{checked_at=(Get-Date).ToUniversalTime().ToString('o');results=$results.ToArray()}|ConvertTo-Json -Depth 5))
}
