-- Upgrade only the original bare floor / empty decorative spots. Preserve edits.
local storage=core.get_mod_storage()
core.after(1,function()
 if storage:get_int('landscape_v2')==1 then return end
 core.emerge_area({x=-8,y=4,z=-12},{x=40,y=12,z=12},function(_,_,remaining)
  if remaining~=0 then return end
  for x=-8,40 do for z=-12,12 do
   local p={x=x,y=5,z=z};local node=core.get_node(p).name
   if node=='mcl_core:stonebrick' then
    local material='mcl_core:dirt_with_grass'
    if math.abs(z)<=1 or (x>=-1 and x<=1) or (x>=16 and x<=18) then material='mcl_core:stonebrick' end
    if (x+1)^2+(z+7)^2<13 then material='mcl_core:lapisblock' end
    if (x+4)^2+(z-8)^2<10 then material='mcl_nether:quartz_block' end
    core.set_node(p,{name=material})
   end
  end end
  for _,p in ipairs({{x=-5,z=-9},{x=5,z=-9},{x=9,z=8},{x=14,z=-8},{x=38,z=9},{x=38,z=-9}}) do
   if core.get_node({x=p.x,y=6,z=p.z}).name=='air' and core.get_node({x=p.x,y=7,z=p.z}).name=='air' and core.get_node({x=p.x,y=8,z=p.z}).name=='air' then
    core.set_node({x=p.x,y=6,z=p.z},{name='mcl_trees:tree_oak'})
    for dx=-1,1 do for dz=-1,1 do local q={x=p.x+dx,y=7,z=p.z+dz};if core.get_node(q).name=='air' then core.set_node(q,{name='mcl_trees:leaves_oak'}) end end end
    core.set_node({x=p.x,y=8,z=p.z},{name='mcl_trees:leaves_oak'})
   end
  end
  storage:set_int('landscape_v2',1)
 end)
end)
