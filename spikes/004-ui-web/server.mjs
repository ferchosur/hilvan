#!/usr/bin/env node
// Spike 004 — read-only local web UI for Hilvan (node:http + SSE + vanilla JS, zero deps).
// Usage (after `npm run build`, from a Hilvan project): node spikes/004-ui-web/server.mjs
// Security: SR-032 loopback + random port, SR-033 session token, SR-034 Host check,
// SR-035 strict CSP + textContent only, SR-036 no arbitrary file access.
import { randomBytes, timingSafeEqual } from 'node:crypto';
import { createServer } from 'node:http';
import { ProjectStore } from '../../dist/store/project.js';

const store = await ProjectStore.find(process.cwd());
const token = randomBytes(32).toString('base64url');

const PAGE = `<!doctype html><html lang="es"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1"><title>Hilvan</title>
<link rel="stylesheet" href="/app.css"></head><body>
<header><h1>Hilvan</h1><span id="status">conectando…</span></header>
<table><thead><tr><th>Tarea</th><th>Estado</th><th>Tipo</th><th>Clasif.</th><th>Perfil</th><th>Última ejecución</th></tr></thead>
<tbody id="tasks"></tbody></table><script src="/app.js"></script></body></html>`;

const CSS = `body{font:14px system-ui,sans-serif;margin:0;background:#0f1115;color:#e6e6e6}
header{display:flex;gap:1rem;align-items:baseline;padding:.75rem 1rem;border-bottom:1px solid #2a2f3a}
h1{font-size:1.1rem;margin:0}#status{color:#8b93a7}table{border-collapse:collapse;width:100%}
th,td{padding:.45rem 1rem;text-align:left;border-bottom:1px solid #1d2230}th{color:#8b93a7;font-weight:500}
.st-running{color:#6cb6ff}.st-in_review{color:#7ee787}.st-blocked,.st-failed{color:#ff7b72}.st-uncertain{color:#e3b341}`;

// Agent-derived strings are only ever assigned with textContent (SR-035).
const JS = `const tbody=document.getElementById('tasks'),status=document.getElementById('status');
function cell(t,cls){const td=document.createElement('td');td.textContent=t??'—';if(cls)td.className=cls;return td}
function render(tasks){tbody.replaceChildren(...tasks.map(t=>{const tr=document.createElement('tr');const r=t.runs.at(-1);
const last=r?[r.state,r.usage?.costUsd!==undefined?'~$'+r.usage.costUsd.toFixed(4):null,
(r.limits||[]).map(w=>w.name+' '+Math.round(w.utilization*100)+'%').join(', ')].filter(Boolean).join(' · '):'';
tr.append(cell(t.title),cell(t.state,'st-'+t.state),cell(t.taskType),cell(t.classification),cell(t.profileId),cell(last));return tr}))}
const es=new EventSource('/events');es.addEventListener('tasks',e=>{render(JSON.parse(e.data));status.textContent='en vivo · '+new Date().toLocaleTimeString()});
es.onerror=()=>{status.textContent='desconectado'}`;

const SECURITY_HEADERS = {
  'Content-Security-Policy': "default-src 'none'; script-src 'self'; style-src 'self'; connect-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'",
  'X-Content-Type-Options': 'nosniff',
  'Referrer-Policy': 'no-referrer',
  'Cache-Control': 'no-store',
};

function validToken(value) {
  if (!value) return false;
  const a = Buffer.from(value);
  const b = Buffer.from(token);
  return a.length === b.length && timingSafeEqual(a, b);
}
function cookieToken(req) {
  return /(?:^|;\s*)hilvan_token=([^;]+)/.exec(req.headers.cookie ?? '')?.[1];
}

let port = 0;
const server = createServer(async (req, res) => {
  const url = new URL(req.url ?? '/', 'http://127.0.0.1');
  // SR-034: reject DNS-rebinding hosts.
  const host = req.headers.host ?? '';
  if (host !== `127.0.0.1:${port}` && host !== `localhost:${port}`) {
    res.writeHead(421).end('misdirected request');
    return;
  }
  // SR-033: first visit carries ?t=<token>; it is swapped for an HttpOnly cookie and dropped from the URL.
  if (url.pathname === '/' && validToken(url.searchParams.get('t') ?? undefined)) {
    res.writeHead(303, {
      ...SECURITY_HEADERS,
      'Set-Cookie': `hilvan_token=${token}; HttpOnly; SameSite=Strict; Path=/`,
      Location: '/',
    });
    res.end();
    return;
  }
  if (!validToken(cookieToken(req))) {
    res.writeHead(401, SECURITY_HEADERS).end('unauthorised: open the URL printed by hilvan');
    return;
  }
  if (req.method !== 'GET') {
    res.writeHead(405, SECURITY_HEADERS).end();
    return;
  }
  const send = (type, body) => res.writeHead(200, { ...SECURITY_HEADERS, 'Content-Type': type }).end(body);
  if (url.pathname === '/') return send('text/html; charset=utf-8', PAGE);
  if (url.pathname === '/app.css') return send('text/css', CSS);
  if (url.pathname === '/app.js') return send('text/javascript', JS);
  if (url.pathname === '/events') {
    res.writeHead(200, { ...SECURITY_HEADERS, 'Content-Type': 'text/event-stream', Connection: 'keep-alive' });
    let last = '';
    const push = async () => {
      const data = JSON.stringify(await store.listTasks());
      if (data !== last) {
        last = data;
        res.write(`event: tasks\ndata: ${data}\n\n`);
      }
    };
    await push();
    const timer = setInterval(() => push().catch(() => {}), 1000);
    req.on('close', () => clearInterval(timer));
    return;
  }
  res.writeHead(404, SECURITY_HEADERS).end();
});

// SR-032: loopback only, random port.
server.listen(0, '127.0.0.1', () => {
  port = server.address().port;
  console.log(`Hilvan UI (solo lectura): http://127.0.0.1:${port}/?t=${token}`);
});
