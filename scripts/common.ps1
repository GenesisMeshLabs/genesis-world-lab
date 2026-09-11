$ErrorActionPreference='Stop'
$RepoRoot=Split-Path $PSScriptRoot -Parent
$LabRoot=Join-Path $RepoRoot '.local'
$LuantiExe=Join-Path $LabRoot 'runtime/luanti-5.17.0-win64/bin/luanti.exe'
function Protect-LabDirectory([string]$Path){
 $directory=Get-Item -LiteralPath $Path
 $acl=$directory.GetAccessControl([Security.AccessControl.AccessControlSections]::Access)
 $acl.SetAccessRuleProtection($true,$false)
 $owner=[Security.Principal.WindowsIdentity]::GetCurrent().User
 foreach($existing in @($acl.Access)){$acl.RemoveAccessRuleSpecific($existing)}
 foreach($sid in @($owner.Value,'S-1-5-18','S-1-5-32-544')){
  $rule=New-Object Security.AccessControl.FileSystemAccessRule([Security.Principal.SecurityIdentifier]$sid,'FullControl','ContainerInherit, ObjectInherit','None','Allow')
  $acl.AddAccessRule($rule)
 }
 # Apply only the DACL; setting owner/audit sections can require elevation.
 $directory.SetAccessControl($acl)
}
function Get-LabProcess([string]$Name,[string]$Executable){
 $file=Join-Path $LabRoot "$Name.pid"
 if(-not(Test-Path -LiteralPath $file)){return $null}
 $processIdValue=[int](Get-Content -LiteralPath $file)
 $process=Get-CimInstance Win32_Process -Filter "ProcessId=$processIdValue"
 if($process -and $process.ExecutablePath -eq [IO.Path]::GetFullPath($Executable)){return $process}
 return $null
}
function Wait-LabReady([string]$URL){
 for($i=0;$i -lt 30;$i++){
  try{$health=Invoke-RestMethod "$URL/readyz" -TimeoutSec 3;if($health.ready -and $health.mode -eq 'genesismesh'){return}}catch{}
  Start-Sleep -Seconds 1
 }
 throw "Bridge did not become ready at $URL; inspect .local/logs"
}
