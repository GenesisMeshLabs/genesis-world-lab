[CmdletBinding()]
param()
. "$PSScriptRoot/common.ps1"
$server=Get-LabProcess 'luanti' $LuantiExe
if($server){
 [IO.File]::WriteAllText("$LabRoot/world/.private/shutdown.request",'shutdown')
 for($i=0;$i -lt 45;$i++){if(-not(Get-LabProcess 'luanti' $LuantiExe)){break};Start-Sleep -Seconds 1}
 if(Get-LabProcess 'luanti' $LuantiExe){throw 'Luanti did not shut down cleanly; inspect logs before taking a backup'}
}
$bridge=Get-LabProcess 'bridge' "$LabRoot/bridge.exe"
if($bridge){Stop-Process -Id $bridge.ProcessId;Wait-Process -Id $bridge.ProcessId -Timeout 15 -ErrorAction SilentlyContinue}
foreach($name in @('alice','bob','north','south')){$client=Get-LabProcess "client-$name" $LuantiExe;if($client){Stop-Process -Id $client.ProcessId}}
Write-Output 'Lab stopped. Luanti flushed its world; bridge transactions were committed with synchronous durability.'
