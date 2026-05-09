package inspector

// inspectorHTML is the dev UI served by the inspector. All dynamic
// values coming from the MCP server (tool names, descriptions,
// argument names, resource URIs, etc.) are rendered via textContent
// or via DOM APIs that set properties, never via innerHTML string
// concatenation. A malicious YAML tool definition, OpenAPI
// operationId, or gRPC service name cannot inject HTML or JavaScript
// through this UI.
const inspectorHTML = `<!DOCTYPE html>
<html lang="en"><head><meta charset="utf-8"><title>GoMCP Inspector</title>
<style>
*{box-sizing:border-box;margin:0;padding:0}
body{font-family:system-ui,-apple-system,sans-serif;background:#0d1117;color:#c9d1d9;display:flex;height:100vh}
nav{width:240px;background:#161b22;border-right:1px solid #30363d;padding:16px;overflow-y:auto}
nav h1{font-size:16px;margin-bottom:16px}
nav h2{font-size:13px;color:#8b949e;text-transform:uppercase;margin:16px 0 8px;letter-spacing:.5px}
nav h2:first-of-type{margin-top:0}
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
.type{color:#484f58}
</style></head><body>
<nav id="sidebar"></nav>
<main id="main"></main>
<script>
'use strict';
const $ = s => document.querySelector(s);

// All helpers below assign text and attribute *values* via properties,
// never by concatenating into innerHTML. Tool names, descriptions,
// URIs, etc. are therefore treated as data, not markup.
function el(tag, opts) {
  const n = document.createElement(tag);
  if (!opts) return n;
  if (opts.text != null) n.textContent = opts.text;
  if (opts.cls) n.className = opts.cls;
  if (opts.attrs) for (const [k, v] of Object.entries(opts.attrs)) n.setAttribute(k, v);
  if (opts.onclick) n.addEventListener('click', opts.onclick);
  if (opts.children) for (const c of opts.children) if (c) n.appendChild(c);
  return n;
}

let tools = [], resources = [], templates = [], prompts = [];

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
  const nav = $('#sidebar');
  nav.replaceChildren();
  nav.appendChild(el('h1', { text: '🔍 GoMCP Inspector' }));

  const section = (title, items, onClick) => {
    nav.appendChild(el('h2', { text: title + ' (' + items.length + ')' }));
    items.forEach((it, i) => {
      nav.appendChild(el('button', { text: it.name || it.uri || '', onclick: () => onClick(i) }));
    });
  };
  section('Tools', tools, showTool);
  section('Resources', resources, showResource);
  section('Prompts', prompts, showPrompt);
}

function showTool(i) {
  const t = tools[i];
  const main = $('#main');
  main.replaceChildren();

  const header = el('h1', { text: t.name || '' });
  if (t.annotations) {
    for (const [k, v] of Object.entries(t.annotations)) {
      header.appendChild(el('span', { cls: 'tag', text: k + ': ' + v }));
    }
  }
  main.appendChild(header);

  const descCard = el('div', { cls: 'card', children: [
    el('h3', { text: t.description || '' }),
    el('pre', { text: JSON.stringify(t.inputSchema, null, 2) }),
  ]});
  main.appendChild(descCard);

  const props = t.inputSchema?.properties || {};
  const req = t.inputSchema?.required || [];
  const paramsDiv = el('div', { cls: 'params' });
  for (const [k, v] of Object.entries(props)) {
    const required = req.includes(k);
    const label = el('label', {});
    label.appendChild(document.createTextNode(k + (required ? ' *' : ' ')));
    label.appendChild(el('span', { cls: 'type', text: '(' + (v.type || '') + ')' }));
    paramsDiv.appendChild(label);
    paramsDiv.appendChild(el('input', { attrs: { name: k, placeholder: v.description || '' } }));
  }
  const btn = el('button', { cls: 'btn', text: 'Execute', onclick: () => callTool(t.name) });
  const callCard = el('div', { cls: 'card', children: [
    el('h3', { text: 'Call Tool' }),
    paramsDiv,
    btn,
  ]});
  main.appendChild(callCard);
  main.appendChild(el('div', { attrs: { id: 'result' } }));
}

async function callTool(name) {
  const inputs = $('#main').querySelectorAll('input[name]');
  const args = {};
  inputs.forEach(el => { if (el.value) args[el.name] = el.value; });
  const resp = await (await fetch('/api/call', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ method: 'tools/call', params: { name: name, arguments: args } }),
  })).json();
  showResult(resp);
}

function showResource(i) {
  const r = resources[i];
  const main = $('#main');
  main.replaceChildren();
  main.appendChild(el('h1', { text: r.name || '' }));
  main.appendChild(el('div', { cls: 'card', children: [
    el('pre', { text: 'URI: ' + (r.uri || '') }),
  ]}));
  main.appendChild(el('button', { cls: 'btn', text: 'Read', onclick: () => readResource(r.uri) }));
  main.appendChild(el('div', { attrs: { id: 'result' } }));
}

async function readResource(uri) {
  const resp = await (await fetch('/api/call', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ method: 'resources/read', params: { uri: uri } }),
  })).json();
  showResult(resp);
}

function showPrompt(i) {
  const p = prompts[i];
  const main = $('#main');
  main.replaceChildren();
  main.appendChild(el('h1', { text: p.name || '' }));
  main.appendChild(el('div', { cls: 'card', children: [
    el('h3', { text: p.description || '' }),
  ]}));

  const paramsDiv = el('div', { cls: 'params' });
  (p.arguments || []).forEach(a => {
    paramsDiv.appendChild(el('label', { text: a.name + (a.required ? ' *' : '') }));
    paramsDiv.appendChild(el('input', { attrs: { name: a.name, placeholder: a.description || '' } }));
  });
  const btn = el('button', { cls: 'btn', text: 'Get', onclick: () => getPrompt(p.name) });
  main.appendChild(el('div', { cls: 'card', children: [
    el('h3', { text: 'Get Prompt' }),
    paramsDiv,
    btn,
  ]}));
  main.appendChild(el('div', { attrs: { id: 'result' } }));
}

async function getPrompt(name) {
  const inputs = $('#main').querySelectorAll('input[name]');
  const args = {};
  inputs.forEach(el => { if (el.value) args[el.name] = el.value; });
  const resp = await (await fetch('/api/call', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ method: 'prompts/get', params: { name: name, arguments: args } }),
  })).json();
  showResult(resp);
}

function showResult(resp) {
  const box = $('#result');
  if (!box) return;
  box.replaceChildren(el('div', { cls: 'card', children: [
    el('h3', { text: 'Result' }),
    el('pre', { text: JSON.stringify(resp, null, 2) }),
  ]}));
}

load();
</script></body></html>`
