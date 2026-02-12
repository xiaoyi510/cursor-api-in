let config={port:3029,api_key:'',admin_password:'',providers:[]};

async function apiFetch(url,options={}){
  const r=await fetch(url,{...options,credentials:'same-origin'});
  if(r.status===401){showLogin();throw new Error('unauthorized')}
  return r;
}

async function checkAuth(){
  try{
    const r=await fetch('/admin/api/auth-check',{credentials:'same-origin'});
    const d=await r.json();
    if(d.need_login){showLogin();return}
    hideLogin();loadConfig();
  }catch(e){showLogin()}
}

function showLogin(){document.getElementById('login-overlay').style.display='flex'}
function hideLogin(){document.getElementById('login-overlay').style.display='none'}

async function login(){
  const pwd=document.getElementById('login-password').value;
  const errEl=document.getElementById('login-error');
  errEl.style.display='none';
  try{
    const r=await fetch('/admin/api/login',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({password:pwd}),credentials:'same-origin'});
    const d=await r.json();
    if(d.ok){hideLogin();loadConfig()}
    else{errEl.textContent=d.error||'登录失败';errEl.style.display='block'}
  }catch(e){errEl.textContent='请求失败';errEl.style.display='block'}
}

async function loadConfig(){
  try{const r=await apiFetch('/admin/api/config');config=await r.json();if(!config.providers)config.providers=[];}catch(e){if(e.message!=='unauthorized')toast('加载失败','err')}
  render();
}

async function putConfig(){
  try{const r=await apiFetch('/admin/api/config',{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify(config)});config=await r.json();toast('已保存');}catch(e){if(e.message!=='unauthorized')toast('保存失败','err')}
  render();
}

function countModels(){
  const s=new Set();
  config.providers.forEach(p=>(p.models||[]).forEach(m=>s.add(m.from)));
  return s.size;
}

function render(){
  document.getElementById('s-providers').textContent=config.providers.length;
  document.getElementById('s-models').textContent=countModels();
  document.getElementById('s-port').textContent=config.port;
  document.getElementById('s-auth').textContent=config.api_key?'已启用':'未启用';

  let oh='';
  config.providers.forEach(p=>{
    const models=(p.models||[]).map(m=>`<span class="chip chip-model" style="${m.enabled?'':'opacity:.4;text-decoration:line-through'}">${esc(m.from)} → ${esc(m.to)}</span>`).join(' ');
    oh+=`<tr><td style="font-family:var(--mono)">${esc(p.id)}</td><td>${esc(p.name)}</td><td><span class="chip chip-${p.type}">${p.type}</span></td><td>${models||'<span style="color:var(--text2)">无</span>'}</td><td><span class="chip chip-weight">${p.weight}</span></td></tr>`;
  });
  document.getElementById('overview-table').innerHTML=oh||'<tr><td colspan="5" class="empty">暂无服务商</td></tr>';

  let ph='';
  config.providers.forEach((p,i)=>{
    const mc=(p.models||[]).length;
    ph+=`<tr>
      <td style="font-family:var(--mono)">${esc(p.id)}</td><td>${esc(p.name)}</td>
      <td><span class="chip chip-${p.type}">${p.type}</span></td>
      <td style="font-family:var(--mono);font-size:12px;max-width:200px;overflow:hidden;text-overflow:ellipsis">${esc(p.base_url)}</td>
      <td>${mc} 个</td>
      <td><span class="chip chip-weight">${p.weight}</span></td>
      <td><div class="actions"><button class="btn btn-sm" onclick="editProvider(${i})">编辑</button><button class="btn btn-sm" onclick="openTestModel(${i})">测试模型</button><button class="btn btn-sm btn-red" onclick="deleteProvider(${i})">删除</button></div></td>
    </tr>`;
  });
  document.getElementById('providers-table').innerHTML=ph||'<tr><td colspan="7" class="empty">暂无服务商，点击上方按钮添加</td></tr>';

  document.getElementById('set-port').value=config.port;
  document.getElementById('set-key').value=config.api_key||'';
  document.getElementById('set-admin-pwd').value=config.admin_password||'';
}

function switchTab(name){
  document.querySelectorAll('.tab').forEach(t=>t.classList.toggle('active',t.textContent.includes({overview:'概览',providers:'服务商',settings:'设置'}[name])));
  document.querySelectorAll('.panel').forEach(p=>p.classList.remove('active'));
  document.getElementById('panel-'+name).classList.add('active');
}

function closeModal(id){document.getElementById(id).classList.remove('show')}

function addModelRow(from,to,enabled){
  if(enabled===undefined)enabled=true;
  const el=document.getElementById('p-models');
  const div=document.createElement('div');
  div.className='model-item'+(enabled?'':' disabled');
  div.innerHTML=`<label class="toggle"><input type="checkbox" ${enabled?'checked':''} onchange="this.closest('.model-item').classList.toggle('disabled',!this.checked)"><span></span></label><input placeholder="请求模型名 (如 gpt-4o)" value="${esc(from||'')}"><span class="arrow">→</span><input placeholder="实际模型名" value="${esc(to||'')}"><button class="btn-x" onclick="this.parentElement.remove()">×</button>`;
  el.appendChild(div);
}

function getModelsFromForm(){
  const items=document.querySelectorAll('#p-models .model-item');
  const models=[];
  items.forEach(item=>{
    const inputs=item.querySelectorAll('input[type=text],input[placeholder]');
    const enabled=item.querySelector('.toggle input').checked;
    const from=inputs[0].value.trim(),to=inputs[1].value.trim();
    if(from&&to)models.push({from,to,enabled});
  });
  return models;
}

function openProviderModal(idx){
  const isEdit=idx!==undefined;
  document.getElementById('provider-modal-title').textContent=isEdit?'编辑服务商':'添加服务商';
  document.getElementById('p-edit-idx').value=isEdit?idx:'';
  document.getElementById('p-test-result').innerHTML='';
  document.getElementById('p-models').innerHTML='';
  if(isEdit){
    const p=config.providers[idx];
    document.getElementById('p-id').value=p.id;
    document.getElementById('p-name').value=p.name;
    document.getElementById('p-type').value=p.type;
    document.getElementById('p-url').value=p.base_url;
    document.getElementById('p-key').value=p.api_key;
    document.getElementById('p-weight').value=p.weight;
    document.getElementById('p-timeout').value=p.timeout;
    (p.models||[]).forEach(m=>addModelRow(m.from,m.to,m.enabled));
  }else{
    ['p-id','p-name','p-url','p-key'].forEach(id=>document.getElementById(id).value='');
    document.getElementById('p-type').value='anthropic';
    document.getElementById('p-weight').value=1;
    document.getElementById('p-timeout').value=300;
  }
  document.getElementById('provider-modal').classList.add('show');
}
function editProvider(i){openProviderModal(i)}
function deleteProvider(i){if(confirm('确定删除此服务商？')){config.providers.splice(i,1);putConfig()}}

function getProviderFromForm(){
  return {
    id:document.getElementById('p-id').value.trim(),
    name:document.getElementById('p-name').value.trim(),
    type:document.getElementById('p-type').value,
    base_url:document.getElementById('p-url').value.trim(),
    api_key:document.getElementById('p-key').value.trim(),
    weight:parseInt(document.getElementById('p-weight').value)||0,
    timeout:parseInt(document.getElementById('p-timeout').value)||300,
    models:getModelsFromForm()
  };
}

function saveProvider(){
  const p=getProviderFromForm();
  if(!p.id||!p.base_url){toast('ID 和 URL 必填','err');return}
  const idx=document.getElementById('p-edit-idx').value;
  if(idx!==''){config.providers[parseInt(idx)]=p}else{config.providers.push(p)}
  closeModal('provider-modal');
  putConfig();
}

async function testProvider(){
  const p=getProviderFromForm();
  const el=document.getElementById('p-test-result');
  el.innerHTML='<div style="color:var(--text2);font-size:12px">测试中...</div>';
  try{
    const r=await apiFetch('/admin/api/providers/test',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(p)});
    const d=await r.json();
    if(d.ok){el.innerHTML=`<div class="test-result test-ok">✓ 连接成功 (HTTP ${d.status})</div>`}
    else{el.innerHTML=`<div class="test-result test-fail">✗ 连接失败 (HTTP ${d.status})\n${d.error||d.body||''}</div>`}
  }catch(e){if(e.message!=='unauthorized')el.innerHTML=`<div class="test-result test-fail">✗ 请求失败: ${e.message}</div>`}
}

async function fetchModels(){
  const p=getProviderFromForm();
  const el=document.getElementById('p-model-picker');
  el.innerHTML='<div style="color:var(--text2);font-size:12px;margin-top:8px">获取中...</div>';
  try{
    const r=await apiFetch('/admin/api/providers/models',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(p)});
    const d=await r.json();
    if(d.error){el.innerHTML=`<div class="test-result test-fail">✗ ${d.error}</div>`;return}
    if(!d.models||!d.models.length){el.innerHTML='<div style="color:var(--text2);font-size:12px;margin-top:8px">未找到模型</div>';return}
    const existing=new Set(getModelsFromForm().map(m=>m.to));
    let html='<div class="model-picker">';
    d.models.forEach(m=>{
      const checked=existing.has(m)?'checked':'';
      html+=`<label><input type="checkbox" value="${esc(m)}" ${checked}>${esc(m)}</label>`;
    });
    html+=`<div class="picker-actions"><button class="btn btn-sm btn-accent" onclick="addPickedModels()">添加选中</button><button class="btn btn-sm" onclick="document.getElementById('p-model-picker').innerHTML=''">关闭</button></div></div>`;
    el.innerHTML=html;
  }catch(e){if(e.message!=='unauthorized')el.innerHTML=`<div class="test-result test-fail">✗ ${e.message}</div>`}
}

function addPickedModels(){
  const existing=new Set(getModelsFromForm().map(m=>m.to));
  document.querySelectorAll('#p-model-picker input:checked').forEach(cb=>{
    if(!existing.has(cb.value)){addModelRow(cb.value,cb.value)}
  });
  document.getElementById('p-model-picker').innerHTML='';
  toast('已添加');
}

// --- Test Model ---

function openTestModel(i){
  const p=config.providers[i];
  const models=[...new Set((p.models||[]).map(m=>m.to))];
  if(!models.length){toast('该服务商未配置模型','err');return}
  document.getElementById('tm-provider-idx').value=i;
  const sel=document.getElementById('tm-model-select');
  sel.innerHTML=models.map(m=>`<option value="${esc(m)}">${esc(m)}</option>`).join('');
  document.getElementById('tm-result').innerHTML='';
  document.getElementById('test-model-modal').classList.add('show');
}

async function runTestModel(){
  const i=parseInt(document.getElementById('tm-provider-idx').value);
  const p=config.providers[i];
  const model=document.getElementById('tm-model-select').value;
  const el=document.getElementById('tm-result');
  const btn=document.getElementById('tm-run-btn');
  btn.disabled=true;btn.textContent='测试中...';
  el.innerHTML='<div style="color:var(--text2);font-size:12px;margin-top:8px">正在发送测试请求...</div>';
  try{
    const r=await apiFetch('/admin/api/providers/test-model',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({provider:p,model})});
    const d=await r.json();
    if(d.ok){
      el.innerHTML=`<div class="test-result test-ok">✓ 模型可用 (HTTP ${d.status}, ${d.latency_ms}ms)${d.reply?'\n回复: '+d.reply:''}</div>`;
    }else{
      el.innerHTML=`<div class="test-result test-fail">✗ 模型不可用 (HTTP ${d.status||'N/A'}, ${d.latency_ms}ms)\n${d.error||''}</div>`;
    }
  }catch(e){if(e.message!=='unauthorized')el.innerHTML=`<div class="test-result test-fail">✗ 请求失败: ${e.message}</div>`}
  btn.disabled=false;btn.textContent='开始测试';
}

function saveSettings(){
  config.port=parseInt(document.getElementById('set-port').value)||3029;
  config.api_key=document.getElementById('set-key').value.trim();
  config.admin_password=document.getElementById('set-admin-pwd').value.trim();
  putConfig();
}

function esc(s){const d=document.createElement('div');d.textContent=s;return d.innerHTML}
function toast(msg,type='ok'){
  const t=document.createElement('div');t.className=`toast toast-${type}`;t.textContent=msg;
  document.body.appendChild(t);setTimeout(()=>t.remove(),2500);
}

checkAuth();
