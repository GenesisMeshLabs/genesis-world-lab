[CmdletBinding()]
param()
. "$PSScriptRoot/common.ps1"
if(-not(Test-Path "$LabRoot/tunnel.json")){throw 'Configure the private named tunnel first'}
$executable=@('C:\Program Files (x86)\cloudflared\cloudflared.exe','C:\Program Files\cloudflared\cloudflared.exe')|Where-Object {Test-Path -LiteralPath $_}|Select-Object -First 1
if(-not $executable){throw 'Install cloudflared before starting the tunnel'}
if(-not(Get-LabProcess 'tunnel' $executable)){
 $p=Start-Process -FilePath $executable -ArgumentList "tunnel --config `"$LabRoot/tunnel.json`" --no-autoupdate run" -WindowStyle Hidden -PassThru -RedirectStandardOutput "$LabRoot/logs/tunnel.out.log" -RedirectStandardError "$LabRoot/logs/tunnel.err.log"
 $p.Id|Set-Content "$LabRoot/tunnel.pid"
}
Write-Output 'Named Cloudflare tunnel started; verify the public endpoint before sharing.'
