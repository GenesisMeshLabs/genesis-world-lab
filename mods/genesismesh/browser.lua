-- Narrow browser controller for authenticated, already connected players.
-- Uses normal Luanti placement/digging and the existing protection hooks.
return function(api)
 local elapsed,pending=0,false
 local results={}
 local materials={stone='mcl_core:stone',wood='mcl_trees:wood_oak',glass='mcl_core:glass'}
 local palette={air='a',['mcl_core:stonebrick']='s',['mcl_core:stone']='s',['mcl_core:goldblock']='g',['mcl_core:glass']='l',['mcl_core:wood']='w',['mcl_core:dirt_with_grass']='r',['mcl_core:dirt']='d'}
 palette['mcl_trees:wood_oak']='w'
 local function blocked(pos)
  local node=core.get_node(pos)
  local def=core.registered_nodes[node.name]
  return not def or def.walkable
 end
 local function execute(c)
  local player=core.get_player_by_name(c.player)
  if not player then return false,'Player relay is offline' end
  if not api.held(c.player,'world.read','') then return false,'Trust lease unavailable' end
  local p=player:get_pos()
  if c.action=='lobby' then player:set_pos(api.lobby);return true,'Returned to lobby' end
  if c.action=='move' then
   if c.x*c.x+c.z*c.z~=1 then return false,'Invalid movement' end
   local target={x=p.x+c.x*0.8,y=p.y,z=p.z+c.z*0.8}
   if target.x < -7 or target.x>39 or target.z < -11 or target.z>11 then return false,'Edge of the browser play area' end
   if api.inside(target) and not api.held(c.player,'region.demo.enter','demo-area') then
    api.audit(c.player,'region.demo.enter','demo-area');return false,'Entry denied. Ask an operator for a court grant.'
   end
   if blocked({x=target.x,y=6,z=target.z}) or blocked({x=target.x,y=7,z=target.z}) then return false,'A block is in the way' end
   player:set_pos(target);return true,'Moved'
  end
  if c.action~='place' and c.action~='dig' then return false,'Unsupported action' end
  if math.abs(c.x-p.x)>4 or math.abs(c.z-p.z)>4 then return false,'Move closer (four-block reach)' end
  local pos={x=c.x,y=6,z=c.z}
  if core.is_protected(pos,c.player) or (api.inside(pos) and not api.held(c.player,'region.demo.enter','demo-area')) then
   return false,'Build denied. A current court building grant is required.'
  end
  local before=core.get_node(pos).name
  if c.action=='place' then
   if not materials[c.material] then return false,'Unknown material' end
   if before~='air' then return false,'Choose an empty tile' end
   for _,other in ipairs(core.get_connected_players()) do
    local op=other:get_pos();if math.abs(op.x-c.x)<0.7 and math.abs(op.z-c.z)<0.7 then return false,'A player occupies that tile' end
   end
   core.item_place_node(ItemStack(materials[c.material]),player,{type='node',under={x=c.x,y=5,z=c.z},above=pos})
  else
   if before=='air' then return false,'No block to remove' end
   -- The browser edits the first layer only; foundations remain intact.
   core.node_dig(pos,core.get_node(pos),player)
  end
  local changed=before~=core.get_node(pos).name
  return changed,changed and (c.action=='place' and 'Block placed' or 'Block removed') or 'The world denied that edit'
 end
 core.register_globalstep(function(dt)
  elapsed=elapsed+dt;if elapsed<0.25 or pending then return end;elapsed=0;pending=true
  local frame={layers={},players={},results=results};results={}
  for y=5,8 do
   local layer={}
   for z=-12,12 do for x=-8,40 do layer[#layer+1]=palette[core.get_node({x=x,y=y,z=z}).name] or 'u' end end
   frame.layers[#frame.layers+1]=table.concat(layer)
  end
  for _,p in ipairs(core.get_connected_players()) do local pos=p:get_pos();frame.players[#frame.players+1]={name=p:get_player_name(),x=pos.x,y=pos.y,z=pos.z} end
  api.request('POST','/v1/game/frame',frame,function(d,err,started)
   pending=false
   if not d then return end
   -- A late response cannot move a player after a long browser/bridge stall.
   for _,c in ipairs(d.commands or {}) do
    local ok,message=false,'Command expired'
    if core.get_us_time()/1000000-started<0.75 then ok,message=execute(c) end
    results[#results+1]={id=c.id,player=c.player,ok=ok,message=message}
   end
  end)
 end)
end
