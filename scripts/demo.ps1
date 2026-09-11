[CmdletBinding()]
param([ValidateSet('grant','revoke','audit','status')][string]$Action='status',[ValidateSet('alice','bob')][string]$Player='alice',[ValidateSet('authority-a','authority-b')][string]$Authority='authority-b',[ValidateRange(1,3600)][int]$Seconds=60,[string]$GrantId)
. "$PSScriptRoot/common.ps1"
$c=Get-Content "$LabRoot/config.json" -Raw|ConvertFrom-Json
$token=[IO.File]::ReadAllText("$LabRoot/credentials/$Authority.token").Trim()
$base="http://$($c.address)";$headers=@{Authorization="Bearer $token"}
switch($Action){
 grant {foreach($cap in @('region.demo.enter','region.demo.build')){$body=@{player_name=$Player;capability=$cap;area=$c.area;ttl_seconds=$Seconds}|ConvertTo-Json;Invoke-RestMethod "$base/v1/delegate" -Method Post -Headers $headers -ContentType application/json -Body $body|Select-Object id,player_name,authority,capability,expires_at}}
 revoke {if($GrantId -notmatch '^[A-Za-z0-9-]{1,80}$'){throw 'Supply a valid -GrantId'};$body=@{grant_id=$GrantId}|ConvertTo-Json;Invoke-RestMethod "$base/v1/revoke" -Method Post -Headers $headers -ContentType application/json -Body $body|Select-Object id,revoked}
 audit {Invoke-RestMethod "$base/v1/audit" -Headers $headers|Format-Table id,time,actor,action,player_name,grant_id}
 status {Invoke-RestMethod "$base/v1/status" -Headers $headers|ConvertTo-Json -Depth 4}
}
