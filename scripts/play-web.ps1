[CmdletBinding()]
param([ValidateSet('alice','bob','north','south')][string]$Player='alice',[switch]$NoOpen)
. "$PSScriptRoot/common.ps1"
& "$PSScriptRoot/start-lab.ps1" -Player $Player
$config=Get-Content "$LabRoot/config.json" -Raw|ConvertFrom-Json
$password=[IO.File]::ReadAllText("$LabRoot/credentials/$Player.password").Trim()
$body=@{player=$Player;password=$password}|ConvertTo-Json
$session=Invoke-RestMethod "http://$($config.address)/play/api/login" -Method Post -ContentType application/json -Body $body
# Fragment credentials never reach HTTP access logs. The page removes this
# fragment immediately and keeps the expiring control session in its tab only.
$url="http://$($config.address)/play/#session=$($session.token)"
[IO.File]::WriteAllText("$LabRoot/browser-url.txt",$url)
if(-not $NoOpen){Start-Process -FilePath $url -WindowStyle Hidden}
Write-Output "Browser game ready at http://$($config.address)/play/ as $Player"
