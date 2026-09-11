'use strict';
const $=id=>document.getElementById(id);
const canvas=$('world'),ctx=canvas.getContext('2d');
let token=sessionStorage.getItem('world-session')||'',state=null,tool='walk',zoom=1,hover=null,geometry=null,hitTiles=[],busy=false,heldKey=null,lastMove=0,lastResult='',lastNotice=0,polling=false;
const fragment=new URLSearchParams(location.hash.slice(1));
if(fragment.has('session')){token=fragment.get('session');sessionStorage.setItem('world-session',token);history.replaceState(null,'',location.pathname);}
const colors={s:['#a6b3a4','#7d9080','#657c6f'],g:['#d8bc70','#b39b56','#978448'],l:['#acd3c0','#81b5a5','#669e90'],w:['#c29e6a','#997549','#7d5e3c'],r:['#839e6d','#688154','#506e49'],d:['#a39370','#897656','#706044'],u:['#8ea480','#728567','#5e7559']};
function notice(text,error=false){$('notice').textContent=text;$('notice').classList.toggle('error',error);lastNotice=Date.now();}
async function api(path,body){
 const response=await fetch('/play/api/'+path,{method:body===undefined?'GET':'POST',headers:{'Content-Type':'application/json',...(token?{Authorization:'Bearer '+token}:{})},body:body===undefined?undefined:JSON.stringify(body),cache:'no-store',signal:AbortSignal.timeout(5000)});
 const data=await response.json();
 if(!response.ok){if(response.status===401&&path!=='login'){token='';sessionStorage.removeItem('world-session');signedOut();}throw new Error(data.error||'The world could not complete that request');}return data;
}
function signedOut(){state=null;heldKey=null;$('login-panel').hidden=false;$('logout').hidden=true;$('operator-panel').hidden=true;$('identity-name').textContent='Not signed in';$('identity-status').textContent='Signed membership required';$('identity-dot').classList.add('muted');$('connection').textContent='Sign in to play';$('connection').classList.remove('live');}
$('login-form').addEventListener('submit',async e=>{e.preventDefault();const button=e.submitter;button.disabled=true;$('login-error').textContent='';try{const d=await api('login',{player:$('player-name').value,password:$('password').value});token=d.token;sessionStorage.setItem('world-session',token);$('password').value='';await refresh();canvas.focus();}catch(e){$('login-error').textContent=e.message;}finally{button.disabled=false;}});
$('logout').addEventListener('click',async()=>{try{await api('logout',{});}catch{}token='';sessionStorage.removeItem('world-session');signedOut();});
function badge(id,allowed){$(id).textContent=allowed?'ALLOWED':'LOCKED';$(id).classList.toggle('allow',allowed);}
async function refresh(){
 if(!token||polling)return;polling=true;
 try{const d=await api('state');state=d;$('login-panel').hidden=true;$('logout').hidden=false;
  const live=d.ready&&d.engine_online,connected=d.frame.players?.some(p=>p.name===d.player);
  $('connection').textContent=live?'● Live world':'Controls paused';$('connection').classList.toggle('live',live);
  $('identity-name').textContent=d.player;$('avatar').textContent=d.player[0].toUpperCase();$('identity-authority').textContent=d.identity.authority.replace('authority-','Authority ').toUpperCase();
  $('identity-status').textContent=live?'Signed identity · fresh trust':'Waiting for world / fresh trust';$('identity-dot').classList.toggle('muted',!live);
  const caps=d.capabilities||[];badge('baseline',caps.includes('world.read'));badge('entry',caps.includes('region.demo.enter'));badge('build',caps.includes('region.demo.build'));$('rights-count').textContent=caps.length+' ACTIVE';
  $('operator-panel').hidden=!d.operator;
  const players=d.frame.players||[];$('player-count').textContent=players.length+' ONLINE';$('players').replaceChildren(...players.map(p=>{const el=document.createElement('span');el.className='player-chip';const dot=document.createElement('i');dot.className='dot';el.append(dot,document.createTextNode(p.name+(p.name===d.player?' · you':'')));return el;}));
  const current=players.find(p=>p.name===d.player);$('coordinates').textContent=current?`X ${current.x.toFixed(1)} / Z ${current.z.toFixed(1)}`:'Player relay offline';
  const results=(d.frame.results||[]).filter(r=>r.player===d.player);
  const latest=results.at(-1);if(latest&&latest.id!==lastResult){lastResult=latest.id;if(latest.message!=='Moved')notice(latest.message,!latest.ok);}
  const activity=[...results.slice(-6).map(r=>({text:r.message,sub:r.ok?'World confirmed':'Action denied'})),...(d.events||[]).slice(-4).reverse().map(e=>({text:e.action.replaceAll('.',' · '),sub:e.player_name||e.actor}))];
  if(activity.length){$('activity').replaceChildren(...activity.map(a=>{const li=document.createElement('li');li.textContent=a.text;const sub=document.createElement('span');sub.textContent=a.sub;li.append(sub);return li;}));}
  if(d.operator){$('grant-list').replaceChildren(...d.grants.filter(g=>g.authority===d.identity.authority).map(g=>{const b=document.createElement('button');b.textContent=`Revoke ${g.player_name} · ${g.capability.endsWith('enter')?'entry':'build'}`;b.addEventListener('click',async()=>{try{await api('revoke',{grant_id:g.id});notice('Revocation saved. The world will remove access.');await refresh();}catch(e){notice(e.message,true);}});return b;}));}
  if(!live){heldKey=null;notice('Controls paused: waiting for the world and fresh trust.',true);}else if(!connected){heldKey=null;notice('Player relay offline. Run scripts/play-web.ps1 for this account.',true);}else if(Date.now()-lastNotice>6000){notice('W A S D to explore · select a material to build');}
 }catch(e){heldKey=null;$('connection').textContent='Disconnected';$('connection').classList.remove('live');notice(e.message,true);}finally{polling=false;}
}
async function command(action,x=0,z=0,material=''){
 if(!token){notice('Sign in to play.',true);return;}if(busy)return;
 busy=true;try{await api('command',{action,x,z,material});}catch(e){notice(e.message,true);heldKey=null;}finally{busy=false;}
}
function selectTool(next){tool=next;document.querySelectorAll('[data-tool]').forEach(b=>b.classList.toggle('selected',b.dataset.tool===next));notice(next==='walk'?'W A S D to move. Click toward a nearby tile to take a step.':next==='dig'?'Click a nearby block to remove it.':'Click an empty nearby tile to place '+next+'.');canvas.focus();}
document.querySelectorAll('[data-tool]').forEach(b=>b.addEventListener('click',()=>selectTool(b.dataset.tool)));
$('lobby').addEventListener('click',()=>command('lobby'));
$('zoom-in').addEventListener('click',()=>{zoom=Math.min(2,zoom+.15);});$('zoom-out').addEventListener('click',()=>{zoom=Math.max(.7,zoom-.15);});
document.querySelectorAll('[data-move]').forEach(b=>b.addEventListener('click',()=>{const [x,z]=b.dataset.move.split(',').map(Number);command('move',x,z);}));
const directions={w:[0,-1],ArrowUp:[0,-1],s:[0,1],ArrowDown:[0,1],a:[-1,0],ArrowLeft:[-1,0],d:[1,0],ArrowRight:[1,0]};
document.addEventListener('keydown',e=>{if(/INPUT|SELECT|TEXTAREA/.test(e.target.tagName)||$('help').open||!$('login-panel').hidden)return;const k=e.key.length===1?e.key.toLowerCase():e.key;if(directions[k]){e.preventDefault();heldKey=k;if(!e.repeat){lastMove=Date.now();command('move',...directions[k]);}}if('12345'.includes(k)&&k.length===1){e.preventDefault();selectTool(['walk','stone','wood','glass','dig'][Number(k)-1]);}});
document.addEventListener('keyup',e=>{if(e.key.toLowerCase()===heldKey?.toLowerCase())heldKey=null;});window.addEventListener('blur',()=>heldKey=null);document.addEventListener('visibilitychange',()=>{if(document.hidden)heldKey=null;});
function tileAt(e){if(!geometry)return null;const b=canvas.getBoundingClientRect(),px=e.clientX-b.left,py=e.clientY-b.top;for(let n=hitTiles.length-1;n>=0;n--){const tile=hitTiles[n],points=tile.points;let inside=false;for(let i=0,j=points.length-1;i<points.length;j=i++){const [xi,yi]=points[i],[xj,yj]=points[j];if(((yi>py)!==(yj>py))&&(px<(xj-xi)*(py-yi)/(yj-yi)+xi))inside=!inside;}if(inside)return {x:tile.x,z:tile.z};}return null;}
canvas.addEventListener('pointermove',e=>{hover=tileAt(e);});canvas.addEventListener('pointerleave',()=>hover=null);
canvas.addEventListener('click',e=>{canvas.focus();const target=tileAt(e),p=state?.frame.players?.find(p=>p.name===state.player);if(!target||!p)return;if(tool==='walk'){const dx=target.x-p.x,dz=target.z-p.z;if(Math.abs(dx)+Math.abs(dz)<.3)return;command('move',Math.abs(dx)>Math.abs(dz)?Math.sign(dx):0,Math.abs(dx)>Math.abs(dz)?0:Math.sign(dz));}else{command(tool==='dig'?'dig':'place',target.x,target.z,tool==='dig'?'':tool);}});
$('grant-button').addEventListener('click',async()=>{const b=$('grant-button');b.disabled=true;try{for(const capability of ['region.demo.enter','region.demo.build'])await api('delegate',{player_name:$('grant-player').value,capability,area:'demo-area',ttl_seconds:60});notice('Signed court grants issued for 60 seconds.');await refresh();}catch(e){notice(e.message,true);}finally{b.disabled=false;}});
$('help-button').addEventListener('click',()=>{$('help').showModal();heldKey=null;});for(const id of ['close-help','help-done'])$(id).addEventListener('click',()=>{$('help').close();canvas.focus();});
function polygon(points,fill,stroke){ctx.beginPath();for(let i=0;i<points.length;i++){if(i===0)ctx.moveTo(...points[i]);else ctx.lineTo(...points[i]);}ctx.closePath();ctx.fillStyle=fill;ctx.fill();if(stroke){ctx.strokeStyle=stroke;ctx.lineWidth=.6;ctx.stroke();}}
function render(){
 const box=canvas.getBoundingClientRect(),dpr=Math.min(devicePixelRatio||1,2);if(canvas.width!==Math.round(box.width*dpr)||canvas.height!==Math.round(box.height*dpr)){canvas.width=Math.round(box.width*dpr);canvas.height=Math.round(box.height*dpr);}ctx.setTransform(dpr,0,0,dpr,0,0);const w=box.width,h=box.height;
 const sky=ctx.createLinearGradient(0,0,0,h);sky.addColorStop(0,'#b6cdc8');sky.addColorStop(1,'#8fae99');ctx.fillStyle=sky;ctx.fillRect(0,0,w,h);
 const t=Math.min(w/79,(h-75)/40)*zoom,ox=w/2-16*t,oy=h*.43-16*t/2;geometry={t,ox,oy};
 hitTiles=[];
 const project=(x,z,y=0)=>[ox+(x-z)*t,oy+(x+z)*t/2-y*t*.9];
 const diamond=(x,z,y=0)=>[project(x-.5,z-.5,y),project(x+.5,z-.5,y),project(x+.5,z+.5,y),project(x-.5,z+.5,y)];
 const frame=state?.frame,layers=frame?.layers;
 // An empty scene has no invented world data. The actual server supplies every tile.
 if(!layers?.length){ctx.fillStyle='#466759';ctx.font='12px system-ui';ctx.textAlign='center';ctx.fillText(token?'Connecting to the live world…':'Your shared world is waiting.',w/2,h*.22);requestAnimationFrame(render);return;}
 const actors=frame.players||[];
 for(let sum=-20;sum<=52;sum++)for(let x=-8;x<=40;x++){const z=sum-x;if(z < -12||z>12)continue;const idx=(z+12)*49+x+8;const floor=layers[0][idx];if(floor!=='a'){const c=colors[floor]||colors.u,points=diamond(x,z);polygon(points,c[0],'#576d4833');hitTiles.push({x,z,points});}
  for(let y=1;y<=3;y++){const node=layers[y][idx];if(node==='a')continue;const c=colors[node]||colors.u,top=diamond(x,z,y),bottom=diamond(x,z,y-1),right=[top[1],top[2],bottom[2],bottom[1]],left=[top[2],top[3],bottom[3],bottom[2]];ctx.globalAlpha=node==='l'?.48:1;polygon(right,c[1]);polygon(left,c[2]);polygon(top,c[0],'#ffffff25');hitTiles.push({x,z,points:right},{x,z,points:left},{x,z,points:top});ctx.globalAlpha=1;}
  if(hover&&hover.x===x&&hover.z===z){polygon(diamond(x,z,.04),tool==='dig'?'#e8987466':'#edffd777','#eff9d1');}
  for(const p of actors){if(Math.round(p.x)!==x||Math.round(p.z)!==z)continue;const [px,py]=project(p.x,p.z,.2);const mine=p.name===state.player,color=p.name==='bob'||p.name==='south'?'#cd92a4':'#78c9be';ctx.fillStyle='#203e3544';ctx.beginPath();ctx.ellipse(px,py+3,t*.48,t*.23,0,0,Math.PI*2);ctx.fill();ctx.fillStyle='#344f49';ctx.fillRect(px-t*.28,py-t*.8,t*.23,t*.75);ctx.fillRect(px+t*.07,py-t*.8,t*.23,t*.75);ctx.fillStyle=color;ctx.fillRect(px-t*.42,py-t*1.6,t*.84,t*.93);ctx.fillStyle='#e4c5a0';ctx.fillRect(px-t*.32,py-t*2.22,t*.64,t*.64);ctx.fillStyle='#364039';ctx.fillRect(px-t*.35,py-t*2.28,t*.7,t*.2);ctx.font=(mine?'600 ':'')+'10px system-ui';ctx.textAlign='center';ctx.fillStyle='#203d35';ctx.fillText(p.name+(mine?' · you':''),px,py-t*2.65);}
 }
 const [cx,cy]=project(28,0,.1);ctx.textAlign='center';ctx.font='600 9px system-ui';ctx.fillStyle='#766035';ctx.fillText('GOLD COURT',cx,cy);
 if(heldKey&&Date.now()-lastMove>210&&!busy){lastMove=Date.now();const [x,z]=directions[heldKey];command('move',x,z);}
 requestAnimationFrame(render);
}
if(!token)signedOut();refresh();setInterval(refresh,350);requestAnimationFrame(render);
