-- Server-side enforcement. The only credential here is a scoped game token
-- supplied in the private runtime configuration; authority keys stay in Go.
local http = core.request_http_api()
local settings = core.settings
local url = settings:get('genesismesh.bridge_url') or 'http://127.0.0.1:8789'
local token = settings:get('genesismesh.game_token') or ''
local area = 'demo-area'
local low = {x=20,y=4,z=-8}
local high = {x=36,y=20,z=8}
local lobby = {x=0,y=7,z=0}
local state = {}
local clock = function() return core.get_us_time()/1000000 end
local accounts = {}
local f=io.open(core.get_worldpath()..'/.private/accounts.json','r')
if f then accounts=core.parse_json(f:read('*a')) or {};f:close() end
if not http or token=='' or next(accounts)==nil then
 error('[genesismesh] HTTP permission, game credential and invited accounts are required')
end
local function request(method,path,body,cb)
 local started=clock()
 http.fetch({url=url..path,method=method,timeout=3,data=body and core.write_json(body) or nil,
  extra_headers={'Content-Type: application/json','Authorization: Bearer '..token}},function(r)
  local d=r.succeeded and r.code>=200 and r.code<300 and core.parse_json(r.data) or nil
  cb(d,d and nil or 'Trust bridge unavailable or request denied',started)
 end)
end
local function inside(p)
 return p.x>=low.x and p.x<=high.x and p.y>=low.y and p.y<=high.y and p.z>=low.z and p.z<=high.z
end
local function clear(name)
 local s=state[name];if s then s.caps={};s.until_time=0;s.epoch=s.epoch+1 end
end
local function held(name,cap,scope)
 local s=state[name];if not s or clock()>=s.until_time then return false end
 for _,g in ipairs(s.caps) do if g.capability==cap and (g.area=='' or g.area==scope) then return true end end
 return false
end
local function audit_boundary(name,cap,scope)
 local s=state[name];if not s then return end
 s.audit=s.audit or {};local key=cap..':'..scope
 if clock()-(s.audit[key] or -10)<2 then return end
 s.audit[key]=clock()
 request('POST','/v1/check',{player_name=name,capability=cap,area=scope},function() end)
end
local function message(name,text)
 local s=state[name];if not s or clock()-(s.notice or 0)<2 then return end
 s.notice=clock();core.chat_send_player(name,core.colorize('#f2bb62','[GenesisMesh] '..text))
end
local refresh
refresh=function(name)
 local s=state[name];if not s or s.pending then return end
 s.pending=true;local epoch=s.epoch
 local path='/v1/identities/'..core.urlencode(name)..'/capabilities'
 request('GET',path,nil,function(d,err,started)
  if state[name]~=s then return end
  s.pending=false
  if epoch~=s.epoch then return end
  if not d then clear(name);s.error=err;return end
  local lease=math.min(1.5,math.max(0,tonumber(d.lease_ms) or 0)/1000)
  s.until_time=started+lease -- network delay cannot extend the lease
  s.caps=d.capabilities or {};s.identity=d.identity;s.error=nil
 end)
end
local auth=core.get_auth_handler()
core.after(0,function()
 for name,account in pairs(accounts) do
  if not auth.get_auth(name) then
   auth.create_auth(name,core.get_password_hash(name,account.password))
   auth.set_privileges(name,{interact=true,shout=true})
  end
 end
end)
core.register_on_prejoinplayer(function(name)
 if not accounts[name] then return 'This GenesisMesh lab is invite-only. Ask the operator for a test account.' end
end)
core.register_on_joinplayer(function(player)
 local name=player:get_player_name()
 local s={caps={},until_time=0,epoch=0,pending=false};state[name]=s
 player:set_pos(lobby)
 s.hud=player:hud_add({type='text',position={x=0.02,y=0.1},alignment={x=1,y=1},scale={x=100,y=100},number=0xa8eaff,text='GenesisMesh | Linking signed identity...'})
 request('POST','/v1/identities/link',{player_name=name},function(d,err)
  if state[name]~=s then return end
  if not d then core.kick_player(name,'GenesisMesh identity unavailable. Retry when the bridge is ready.');return end
  s.identity=d.identity;refresh(name)
  core.chat_send_player(name,'[GenesisMesh] Welcome. /gm_status shows your rights; /gm_demo explains the protected court east of spawn.')
 end)
end)
core.register_on_leaveplayer(function(p) state[p:get_player_name()]=nil end)
local old_protected=core.is_protected
core.is_protected=function(pos,name)
 if old_protected(pos,name) then return true end -- retain other mods' protection
 if not held(name,'world.build','') or not held(name,'world.destroy','') then return true end
 if inside(pos) and not held(name,'region.demo.build',area) then
  audit_boundary(name,'region.demo.build',area);message(name,'Building here needs a current region.demo.build grant.');return true
 end
 return false
end
core.register_on_chat_message(function(name)
 if not held(name,'chat.send','') then message(name,'Chat waits for a current identity lease.');return true end
 return false
end)
local elapsed=0
core.register_globalstep(function(dt)
 elapsed=elapsed+dt;if elapsed<0.1 then return end
 elapsed=0
 for _,p in ipairs(core.get_connected_players()) do
  local name=p:get_player_name();local s=state[name]
  if s then
   if clock()>=(s.next_refresh or 0) then s.next_refresh=clock()+0.5;refresh(name) end
   local enter=held(name,'region.demo.enter',area)
   local build=held(name,'region.demo.build',area)
   if inside(p:get_pos()) and not enter then audit_boundary(name,'region.demo.enter',area);p:set_pos(lobby);message(name,'Entry denied: no current region.demo.enter grant.') end
   if not held(name,'world.read','') then p:set_pos(lobby) end
   if s.hud then p:hud_change(s.hud,'text','GENESISMESH WORLD LAB\n'..name..' | '..(s.identity and s.identity.authority or 'linking')..'\nCourt entry: '..(enter and 'ALLOWED' or 'DENIED')..'  Build: '..(build and 'ALLOWED' or 'DENIED')..'\n'..(clock()<s.until_time and 'Signed identity / fresh revocation feed' or 'Trust unavailable - protected actions closed')) end
  end
 end
end)
core.register_chatcommand('gm_status',{description='Show your signed identity and current capability lease.',func=function(name)
 local s=state[name];if not s or not s.identity then return false,'Identity not linked yet' end
 local caps={};for _,g in ipairs(s.caps) do caps[#caps+1]=g.capability end
 return true,s.identity.genesismesh_id..'\nLease valid: '..tostring(clock()<s.until_time)..'\n'..table.concat(caps,', ')
end})
core.register_chatcommand('gm_demo',{description='Explain the delegation and revocation demonstration.',func=function()
 return true,'Walk east to the glass court (x20..36). Entry/build start denied. North or south delegates region.demo.enter and region.demo.build for 60 seconds. Try again, then revoke or wait for expiry. Rights and denial are visible in the HUD. /gm_lobby returns to spawn.'
end})
core.register_chatcommand('gm_lobby',{description='Return to the demonstration lobby.',func=function(name)
 local p=core.get_player_by_name(name);if p then p:set_pos(lobby) end;return true,'Returned to lobby'
end})
core.register_chatcommand('gm_delegate',{params='<player> <capability> <seconds>',description='Delegate as your configured authority operator.',func=function(name,param)
 if not accounts[name] or not accounts[name].operator then return false,'Not an authority operator' end
 local target,cap,ttl=param:match('^(%S+)%s+(%S+)%s+(%d+)$');if not target then return false,'Usage: /gm_delegate alice region.demo.enter 60' end
 clear(target)
 request('POST','/v1/delegate',{caller_name=name,player_name=target,capability=cap,area=area,ttl_seconds=tonumber(ttl)},function(d,err)
  core.chat_send_player(name,d and ('[GenesisMesh] Grant '..d.id..' issued for '..target) or ('[GenesisMesh] '..err));refresh(target)
 end)
 return true,'Requesting signed authority attestation...'
end})
core.register_chatcommand('gm_revoke',{params='<grant_id>',description='Revoke a grant issued by your authority.',func=function(name,param)
 if not accounts[name] or not accounts[name].operator then return false,'Not an authority operator' end
 local id=param:match('^(%S+)$');if not id then return false,'Usage: /gm_revoke <grant_id>' end
 for n in pairs(state) do clear(n) end
 request('POST','/v1/revoke',{caller_name=name,grant_id=id},function(d,err)
  core.chat_send_player(name,d and '[GenesisMesh] Revocation persisted and published' or ('[GenesisMesh] '..err))
  for n in pairs(state) do clear(n);refresh(n) end
 end)
 return true,'Revoking; local permission caches invalidated.'
end})
-- Visible deterministic court and lobby, created once. Preserve the world on restart.
local storage=core.get_mod_storage()
core.after(0,function()
 if storage:get_int('court_v1')==1 then return end
 core.emerge_area({x=-8,y=3,z=-12},{x=40,y=22,z=12},function(_,_,remaining)
  if remaining~=0 then return end
  for x=-8,40 do for z=-12,12 do
   core.set_node({x=x,y=5,z=z},{name='mcl_core:stonebrick'})
   for y=6,12 do core.set_node({x=x,y=y,z=z},{name='air'}) end
   if x>=20 and x<=36 and z>=-8 and z<=8 then core.set_node({x=x,y=5,z=z},{name='mcl_core:goldblock'}) end
  end end
  for x=20,36 do for _,z in ipairs({-8,8}) do for y=6,8 do core.set_node({x=x,y=y,z=z},{name='mcl_core:glass'}) end end end
  for z=-8,8 do for y=6,8 do core.set_node({x=36,y=y,z=z},{name='mcl_core:glass'}) end end
  storage:set_int('court_v1',1);core.log('action','[genesismesh] Protected court and lobby constructed')
 end)
end)
-- Narrow hooks for automated engine tests; no player bypass or public grant API.
genesismesh={held=held,in_demo_area=inside,refresh=refresh,lobby=lobby}
if settings:get_bool('genesismesh.browser_enabled',false) then
 dofile(core.get_modpath('genesismesh')..'/browser.lua')({request=request,held=held,inside=inside,lobby=lobby,audit=audit_boundary})
end
core.log('action','[genesismesh] Signed-membership enforcement loaded')
-- Local operator shutdown request: let Luanti flush its world normally.
local control_elapsed=0
core.register_globalstep(function(dt)
 control_elapsed=control_elapsed+dt;if control_elapsed<1 then return end;control_elapsed=0
 local path=core.get_worldpath()..'/.private/shutdown.request'
 local request=io.open(path,'r');if request then request:close();os.remove(path);core.request_shutdown('Lab stopped by local operator',false,0) end
end)
