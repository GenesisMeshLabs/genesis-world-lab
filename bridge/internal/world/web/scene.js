import * as T from './vendor/three.module.js';

// Visualizes the authenticated engine snapshot. Decorations are actual server
// nodes; labels and trust rings are overlays, never collision or permission data.
const source = document.getElementById('world');
try {
 const renderer = new T.WebGLRenderer({antialias:true, alpha:false});
 renderer.setPixelRatio(Math.min(devicePixelRatio, 2));
 renderer.shadowMap.enabled = true;
 renderer.shadowMap.type = T.PCFSoftShadowMap;
 renderer.setClearColor('#102b31');
 const el = renderer.domElement;
 el.id='world-3d'; el.tabIndex=0;
 el.setAttribute('aria-label','Live 3D world. WASD to move, click to build, drag to orbit.');
 el.style.cssText='position:absolute;inset:0;width:100%;height:100%;z-index:0';
 source.after(el); source.style.visibility='hidden';
 const scene = new T.Scene(), camera=new T.OrthographicCamera(-40,40,25,-25,.1,250);
 const target=new T.Vector3(15,0,0), ray=new T.Raycaster(), pointer=new T.Vector2();
 let azimuth=.72, scale=1, state=null, lastLayers='', width=0,height=0, following=false;
 scene.add(new T.HemisphereLight('#e4f8ff','#355142',2.5));
 const sun=new T.DirectionalLight('#fff0d0',3.4); sun.position.set(-15,40,20);sun.castShadow=true;
 sun.shadow.mapSize.set(2048,2048); Object.assign(sun.shadow.camera,{left:-45,right:45,top:35,bottom:-35,near:1,far:100});sun.shadow.bias=-.001;
 scene.add(sun); scene.add(sun.target);sun.target.position.set(15,0,0);
 const cube=new T.BoxGeometry(.985,.985,.985), blocks=new T.Group();scene.add(blocks);
 const palette={s:'#8a9e9f',g:'#edc568',l:'#6fc5cb',w:'#b38958',r:'#599d79',d:'#7f6951',u:'#7d9481',b:'#527bc1',e:'#50bb98',q:'#e0e5d9',t:'#805333',v:'#37896d'};
 const materials=Object.fromEntries(Object.entries(palette).map(([key,color])=>[key,new T.MeshStandardMaterial({color,roughness:.8,metalness:key==='g'?.35:.05,transparent:key==='l',opacity:key==='l'?.5:1})]));
 const base=new T.Mesh(new T.BoxGeometry(49,1.6,25),new T.MeshStandardMaterial({color:'#254b48',roughness:1}));base.position.set(16,-1.3,0);base.receiveShadow=true;scene.add(base);
 const water=new T.Mesh(new T.PlaneGeometry(350,350),new T.MeshStandardMaterial({color:'#143840',roughness:.65,metalness:.25}));water.rotation.x=-Math.PI/2;water.position.y=-2.2;water.receiveShadow=true;scene.add(water);
 const select=new T.Mesh(new T.BoxGeometry(1.04,.055,1.04),new T.MeshBasicMaterial({color:'#e1ff9b',transparent:true,opacity:.65}));select.visible=false;scene.add(select);
 const avatars=new Map(), labels=[];
 function label(text,x,y,z,kind='station-label') {const div=document.createElement('div');div.className=kind;div.textContent=text;source.parentElement.append(div);const obj={div,position:new T.Vector3(x,y,z)};labels.push(obj);return obj;}
 label('01 / IDENTITY DOCK',-1,1,-7);label('02 / FEDERATION GATE',18,1,-3);label('03 / GOLD COURT',28,1,0);label('04 / RECOVERY ARCHIVE',-4,1,8);
 const rings=[];
 for(const [x,z,color] of [[-1,-7,'#79b7ff'],[18,0,'#d8ef90'],[28,0,'#ffd77e'],[-4,8,'#d1b9ff']]) {const mesh=new T.Mesh(new T.TorusGeometry(1.6,.055,8,48),new T.MeshBasicMaterial({color,transparent:true,opacity:.7}));mesh.rotation.x=Math.PI/2;mesh.position.set(x,.56,z);scene.add(mesh);rings.push(mesh);}
 function rebuild(layers){for(const mesh of [...blocks.children]){blocks.remove(mesh);mesh.dispose?.();}const buckets={};layers.forEach((layer,y)=>{[...layer].forEach((code,i)=>{if(code==='a')return;(buckets[code]??=[]).push({x:i%49-8,y,z:Math.floor(i/49)-12});});});for(const [code,coords] of Object.entries(buckets)){const mesh=new T.InstancedMesh(cube,materials[code]||materials.u,coords.length),matrix=new T.Matrix4();coords.forEach((p,i)=>{matrix.makeTranslation(p.x,p.y,p.z);mesh.setMatrixAt(i,matrix);});mesh.userData.coords=coords;mesh.castShadow=code!=='l';mesh.receiveShadow=true;mesh.computeBoundingSphere();blocks.add(mesh);}}
 function avatar(p){const group=new T.Group(),color=p.name==='bob'||p.name==='south'?'#db91b8':'#85d9d1';const part=(size,pos,c)=>{const m=new T.Mesh(new T.BoxGeometry(...size),new T.MeshStandardMaterial({color:c}));m.position.set(...pos);m.castShadow=true;group.add(m);};part([.55,.64,.32],[0,1.16,0],color);part([.44,.43,.42],[0,1.71,0],'#f1cda4');part([.47,.12,.44],[0,1.92,0],'#34404c');part([.2,.55,.25],[-.16,.56,0],'#354e68');part([.2,.55,.25],[.16,.56,0],'#354e68');scene.add(group);const tag=label(p.name+(p.name===state.player?' · YOU':''),p.x,2.5,p.z,'avatar-label');return{group,tag};}
 function pick(e){const rect=el.getBoundingClientRect();pointer.set((e.clientX-rect.left)/rect.width*2-1,-(e.clientY-rect.top)/rect.height*2+1);ray.setFromCamera(pointer,camera);const hit=ray.intersectObjects(blocks.children,false)[0];return hit?hit.object.userData.coords[hit.instanceId]:null;}
 let down=null,dragged=false;
 el.addEventListener('pointerdown',e=>{down={x:e.clientX,y:e.clientY,azimuth};dragged=false;el.setPointerCapture(e.pointerId);});
 el.addEventListener('pointermove',e=>{if(down&&Math.abs(e.clientX-down.x)>5){dragged=true;azimuth=down.azimuth-(e.clientX-down.x)*.006;}const tile=pick(e);select.visible=!!tile;if(tile)select.position.set(tile.x,tile.y+.53,tile.z);});
 el.addEventListener('pointerup',()=>down=null);el.addEventListener('pointercancel',()=>down=null);el.addEventListener('pointerleave',()=>select.visible=false);
 el.addEventListener('click',e=>{if(!dragged)source.dispatchEvent(new MouseEvent('click',{clientX:e.clientX,clientY:e.clientY}));el.focus();});
 el.addEventListener('wheel',e=>{e.preventDefault();scale=T.MathUtils.clamp(scale*(e.deltaY>0?.93:1.07),.7,2.8);},{passive:false});
 document.getElementById('follow-player')?.addEventListener('click',e=>{following=!following;e.currentTarget.classList.toggle('selected',following);});
 window.worldView={pick,zoom(value){scale=value;},update(value){state=value;const layers=state?.frame?.layers;if(layers){const key=layers.join('');if(key!==lastLayers){lastLayers=key;rebuild(layers);}}}};
 let last=performance.now();
 function draw(now){requestAnimationFrame(draw);if(document.hidden)return;const dt=Math.min((now-last)/1000,.1);last=now;const rect=el.getBoundingClientRect();if(rect.width!==width||rect.height!==height){width=rect.width;height=rect.height;renderer.setSize(width,height,false);}const aspect=width/Math.max(height,1),span=Math.max(23,35/aspect)/scale;camera.left=-span*aspect;camera.right=span*aspect;camera.top=span;camera.bottom=-span;camera.updateProjectionMatrix();const current=state?.frame?.players?.find(p=>p.name===state.player);target.lerp(new T.Vector3(following&&current?current.x:15,0,following&&current?current.z:0),1-Math.exp(-dt*5));camera.position.set(target.x+Math.sin(azimuth)*55,48,target.z+Math.cos(azimuth)*55);camera.lookAt(target);
 const online=new Set();for(const p of state?.frame?.players||[]){online.add(p.name);if(!avatars.has(p.name)){const a=avatar(p);a.group.position.set(p.x,0,p.z);avatars.set(p.name,a);}const a=avatars.get(p.name);a.group.position.lerp(new T.Vector3(p.x,0,p.z),1-Math.exp(-dt*12));a.tag.position.set(a.group.position.x,2.6,a.group.position.z);}
 for(const [name,a] of avatars){if(!online.has(name)){scene.remove(a.group);a.group.traverse(o=>{o.geometry?.dispose();o.material?.dispose();});a.tag.div.remove();labels.splice(labels.indexOf(a.tag),1);avatars.delete(name);}}
 for(const item of labels){const v=item.position.clone().project(camera);item.div.style.transform=`translate(${(v.x+1)*width/2}px,${(1-v.y)*height/2}px) translate(-50%,-100%)`;item.div.hidden=!state||v.x<-.95||v.x>.95||v.y<-1||v.y>1;}
 renderer.render(scene,camera);
 }
 requestAnimationFrame(draw);
 el.addEventListener('webglcontextlost',e=>{e.preventDefault();window.worldView=null;el.hidden=true;source.style.visibility='visible';for(const l of labels)l.div.hidden=true;});
} catch(error) {source.style.visibility='visible';document.getElementById('world-3d')?.remove();console.warn('Using accessible 2D world renderer:',error.message);}
