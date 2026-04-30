package inspector

const inspectorHTML = `<!DOCTYPE html>
<html lang="en"><head><meta charset="utf-8"><title>GoMCP Inspector</title>
<style>
*{box-sizing:border-box;margin:0;padding:0}
body{font-family:system-ui,-apple-system,sans-serif;background:#0d1117;color:#c9d1d9;display:flex;height:100vh}
nav{width:240px;background:#161b22;border-right:1px solid #30363d;padding:16px;overflow-y:auto}
nav h2{font-size:13px;color:#8b949e;text-transform:uppercase;margin:16px 0 8px;letter-spacing:.5px}
nav h2:first-child{margin-top:0}
nav button{display:block;width:100%;text-align:left;background:none;border:none;color:#c9d1d9;padding:6px 8px;border-radius:6px;cursor:pointer;font-size:13px;margin:2px 0}
nav button:hover{background:#21262d}
nav button.active{background:#1f6feb;color:#fff}
main{flex:1;display:flex;flex-direction:column;padding:24px;overflow-y:auto}
h1{font-size:20px;margin-bottom:16px;color:#f0f6fc}
.card{background:#161b22;border:1px solid #30363d;border-radius:8px;padding:16px;margin-bottom:16px}
.card h3{font-size:14px;color:#58a6ff;margin-bottom:8px}
pre{background:#0d1117;border:1px solid #30363d;border-radius:6px;padding:12px;font-size:12px;overflow-x:auto;white-space:pre-wrap;color:#8b949e}
.params{display:flex;flex-direction:column;gap:8px;margin:12px 0}
.params label{font-size:12px;color:#8b949e}
.params input,.params textarea{background:#0d1117;border:1px solid #30363d;border-radius:6px;padding:8px;color:#c9d1d9;font-size:13px;font-family:monospace}
.btn{background:#238636;color:#fff;border:none;padding:8px 16px;border-radius:6px;cursor:pointer;font-size:13px;margin-top:8px}
.btn:hover{background:#2ea043}
#result{margin-top:16px}
.tag{display:inline-block;background:#1f6feb22;color:#58a6ff;padding:2px 8px;border-radius:4px;font-size:11px;margin-left:8px}
</style></head><body>
<nav id="sidebar"><h1 style="font-size:16px;margin-bottom:16px">🔍 GoMCP Inspector</h1></nav>
<main id="main"><h1>Select a tool, resource, or prompt</h1></main>
<script>
const $ = s => document.querySelector(s);
let tools=[], resources=[], templates=[], prompts=[];

async function load() {
  const toolsResp = await (await fetch('/api/tools')).json();
  tools = toolsResp.tools || toolsResp || [];
  const res = await (await fetch('/api/resources')).json();
  resources = res.resources?.resources || res.resources || [];
  templates = res.templates?.templates || res.templates || [];
  const promptsResp = await (await fetch('/api/prompts')).json();
  prompts = promptsResp.prompts || promptsResp || [];
  renderNav();
}

function renderNav() {
  let h = '<h1 style="font-size:16px;margin-bottom:16px">🔍 GoMCP Inspector</h1>';
  h += '<h2>Tools (' + tools.length + ')</h2>';
  tools.forEach((t,i) => h += '<button onclick="showTool('+i+')">'+t.name+'</button>');
  h += '<h2>Resources (' + resources.length + ')</h2>';
  resources.forEach((r,i) => h += '<button onclick="showResource('+i+')">'+r.name+'</button>');
  h += '<h2>Prompts (' + prompts.length + ')</h2>';
  prompts.forEach((p,i) => h += '<button onclick="showPrompt('+i+')">'+p.name+'</button>');
  $('#sidebar').innerHTML = h;
}

function showTool(i) {
  const t = tools[i];
  const props = t.inputSchema?.properties || {};
  const req = t.inputSchema?.required || [];
  let fields = '';
  for (const [k,v] of Object.entries(props)) {
    const r = req.includes(k) ? ' *' : '';
    fields += '<label>'+k+r+' <span style="color:#484f58">('+v.type+')</span></label>';
    fields += '<input name="'+k+'" placeholder="'+(v.description||'')+'">';
  }
  let ann = '';
  if (t.annotations) for (const [k,v] of Object.entries(t.annotations)) ann += '<span class="tag">'+k+': '+v+'</span>';
  $('#main').innerHTML = '<h1>'+t.name+ann+'</h1><div class="card"><h3>'+t.description+'</h3>'
    +'<pre>'+JSON.stringify(t.inputSchema,null,2)+'</pre></div>'
    +'<div class="card"><h3>Call Tool</h3><div class="params">'+fields+'</div>'
    +'<button class="btn" onclick="callTool(\''+t.name+'\')">Execute</button></div>'
    +'<div id="result"></div>';
}

async function callTool(name) {
  const inputs = $('#main').querySelectorAll('input[name]');
  const args = {};
  inputs.forEach(el => { if(el.value) args[el.name] = el.value; });
  const resp = await (await fetch('/api/call', {method:'POST',headers:{'Content-Type':'application/json'},
    body: JSON.stringify({method:'tools/call',params:{name,arguments:args}})})).json();
  $('#result').innerHTML = '<div class="card"><h3>Result</h3><pre>'+JSON.stringify(resp,null,2)+'</pre></div>';
}

function showResource(i) {
  const r = resources[i];
  $('#main').innerHTML = '<h1>'+r.name+'</h1><div class="card"><pre>URI: '+r.uri+'</pre></div>'
    +'<button class="btn" onclick="readResource(\''+r.uri+'\')">Read</button><div id="result"></div>';
}

async function readResource(uri) {
  const resp = await (await fetch('/api/call', {method:'POST',headers:{'Content-Type':'application/json'},
    body: JSON.stringify({method:'resources/read',params:{uri}})})).json();
  $('#result').innerHTML = '<div class="card"><h3>Result</h3><pre>'+JSON.stringify(resp,null,2)+'</pre></div>';
}

function showPrompt(i) {
  const p = prompts[i];
  let fields = '';
  (p.arguments||[]).forEach(a => {
    fields += '<label>'+a.name+(a.required?' *':'')+'</label><input name="'+a.name+'" placeholder="'+(a.description||'')+'">';
  });
  $('#main').innerHTML = '<h1>'+p.name+'</h1><div class="card"><h3>'+p.description+'</h3></div>'
    +'<div class="card"><h3>Get Prompt</h3><div class="params">'+fields+'</div>'
    +'<button class="btn" onclick="getPrompt(\''+p.name+'\')">Get</button></div><div id="result"></div>';
}

async function getPrompt(name) {
  const inputs = $('#main').querySelectorAll('input[name]');
  const args = {};
  inputs.forEach(el => { if(el.value) args[el.name] = el.value; });
  const resp = await (await fetch('/api/call', {method:'POST',headers:{'Content-Type':'application/json'},
    body: JSON.stringify({method:'prompts/get',params:{name,arguments:args}})})).json();
  $('#result').innerHTML = '<div class="card"><h3>Result</h3><pre>'+JSON.stringify(resp,null,2)+'</pre></div>';
}

load();
</script></body></html>`
