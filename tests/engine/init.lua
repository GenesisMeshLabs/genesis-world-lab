-- Explicitly installed only in the local acceptance world, not a shipped mod.
local elapsed=0
core.register_globalstep(function(dt)
 elapsed=elapsed+dt;if elapsed<0.1 then return end;elapsed=0
 local root=core.get_worldpath()..'/.private/'
 local file=io.open(root..'probe.json','r');if not file then return end
 local q=core.parse_json(file:read('*a'));file:close();os.remove(root..'probe.json')
 if not q then return end
 local player=core.get_player_by_name(q.player or 'alice')
 if q.action=='move' and player then player:set_pos(q.position) end
 core.after(0.4,function()
  local p=core.get_player_by_name(q.player or 'alice')
  local report={id=q.id,connected={},player=q.player}
  for _,v in ipairs(core.get_connected_players()) do table.insert(report.connected,v:get_player_name()) end
  if p then
   report.position=p:get_pos()
   report.entry=genesismesh.held(q.player,'region.demo.enter','demo-area')
   report.build=genesismesh.held(q.player,'region.demo.build','demo-area')
   report.protected=core.is_protected({x=24,y=6,z=0},q.player)
   if q.action=='place' then
    local target={x=24,y=6,z=0}
    local before=core.get_node(target).name
    core.item_place_node(ItemStack('mcl_core:stone'),p,{type='node',under={x=24,y=5,z=0},above=target})
    report.before=before;report.after=core.get_node(target).name
   end
   if q.action=='dig' then
    local target={x=24,y=6,z=0};report.before=core.get_node(target).name
    core.node_dig(target,core.get_node(target),p);report.after=core.get_node(target).name
   end
  end
  local out=io.open(root..'probe-result.json','w');out:write(core.write_json(report));out:close()
 end)
end)