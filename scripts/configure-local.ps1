$ErrorActionPreference='Stop'
$repo=Split-Path $PSScriptRoot -Parent
$local=Join-Path $repo '.local'
if(Test-Path "$local/config.json"){throw 'Private configuration exists; edit it deliberately instead of overwriting it'}
New-Item -ItemType Directory -Force "$local/credentials","$local/state","$local/logs"|Out-Null
function Secret($path){if(-not(Test-Path $path)){$bytes=New-Object byte[] 32;[Security.Cryptography.RandomNumberGenerator]::Create().GetBytes($bytes);[IO.File]::WriteAllText($path,[Convert]::ToBase64String($bytes))};return [IO.File]::ReadAllText($path).Trim()}
function Hash($s){$sha=[Security.Cryptography.SHA256]::Create();return ([BitConverter]::ToString($sha.ComputeHash([Text.Encoding]::UTF8.GetBytes($s)))).Replace('-','').ToLower()}
$game=Secret "$local/credentials/game.token"; $aToken=Secret "$local/credentials/authority-a.token"; $bToken=Secret "$local/credentials/authority-b.token"
$authorities=@();foreach($name in @('authority-a','authority-b')){$base=Join-Path (Split-Path $repo -Parent) "sandbox/nas/docker-fleet/$name";$gen=Get-Content "$base/genesis.signed.json" -Raw|ConvertFrom-Json;Copy-Item -LiteralPath "$base/keys/operator.key" -Destination "$local/credentials/$name-operator.key";$port=if($name -eq 'authority-a'){20443}else{20444};$authorities+=@{name=$name;url="http://127.0.0.1:$port";public_key=$gen.network_authority.public_key;key_id='na-2025-q1';operator_key_file="$local/credentials/$name-operator.key";operator_key_id='operator-local';root_file="$local/credentials/$name-root.json"}}
# Local account bindings are explicit. Their native Luanti authentication
# passwords are independent from the bridge's scoped service credentials.
$players=@();foreach($name in @('alice','bob','north','south')){$authority=if($name -in @('alice','north')){'authority-a'}else{'authority-b'};$gen=$authorities|Where-Object name -eq $authority;$pw=Secret "$local/credentials/$name.password";$player=@{name=$name;authority=$authority;identity_file="$local/credentials/$name-identity.json";public_key=(& "$local/bridge.exe" -keygen "$local/credentials/$name.key")};if($name -eq 'north'){$player.operator_authority='authority-a'};if($name -eq 'south'){$player.operator_authority='authority-b'};$players+=$player}
$config=@{address='127.0.0.1:8789';state_file="$local/state/bridge.bolt";world='genesis-world-lab';area='demo-area';gateway_url='http://127.0.0.1:8080';owner='authority-a';authorities=$authorities;players=$players;principals=@(@{name='luanti';game=$true;token_hash=(Hash $game)},@{name='north';authority='authority-a';token_hash=(Hash $aToken)},@{name='south';authority='authority-b';token_hash=(Hash $bToken)})}
[IO.File]::WriteAllText("$local/config.json",($config|ConvertTo-Json -Depth 8))
'Private lab configuration prepared'