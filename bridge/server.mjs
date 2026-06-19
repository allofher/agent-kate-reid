// repl-control bridge server.
//
// Serves the Strudel page (public/ + vendored @strudel/web from node_modules)
// and runs the WebSocket hub that relays control messages between Kate's Go
// harness (clientType "cli") and the browser tab (clientType "browser").
//
// Wire protocol: see ../docs/tools.md and ../internal/strudel/protocol.go.
// Everything binds to localhost only.

import { createServer } from 'node:http';
import { readFile } from 'node:fs/promises';
import { join, normalize } from 'node:path';
import { fileURLToPath } from 'node:url';
import { WebSocketServer } from 'ws';

const PORT = Number(process.env.BRIDGE_PORT ?? 8081);
const HOST = '127.0.0.1';

const root = fileURLToPath(new URL('.', import.meta.url));
const publicDir = join(root, 'public');
const vendorDir = join(root, 'node_modules', '@strudel', 'web', 'dist');

const MIME = {
  '.html': 'text/html; charset=utf-8',
  '.js': 'text/javascript; charset=utf-8',
  '.mjs': 'text/javascript; charset=utf-8',
  '.css': 'text/css; charset=utf-8',
  '.json': 'application/json',
  '.wav': 'audio/wav',
  '.mp3': 'audio/mpeg',
  '.ogg': 'audio/ogg',
};

// --- static file server -----------------------------------------------------

async function serveFile(res, path) {
  try {
    const data = await readFile(path);
    const ext = path.slice(path.lastIndexOf('.'));
    res.writeHead(200, { 'content-type': MIME[ext] ?? 'application/octet-stream' });
    res.end(data);
  } catch {
    res.writeHead(404);
    res.end('not found');
  }
}

const httpServer = createServer((req, res) => {
  const url = new URL(req.url, `http://${req.headers.host}`);
  let path = normalize(url.pathname).replace(/^(\.\.[/\\])+/, '');

  if (path === '/' || path === '') path = '/index.html';

  // /vendor/* maps into the installed @strudel/web bundle so the page works
  // fully offline — no CDN. Everything else comes from public/.
  if (path.startsWith('/vendor/')) {
    serveFile(res, join(vendorDir, path.slice('/vendor/'.length)));
  } else {
    serveFile(res, join(publicDir, path));
  }
});

// --- WebSocket hub -----------------------------------------------------------

// One browser (a newer tab replaces an older one), any number of cli clients.
let browser = null;
const clis = new Set();

const wss = new WebSocketServer({ server: httpServer });

function send(ws, msg) {
  if (ws && ws.readyState === ws.OPEN) ws.send(JSON.stringify(msg));
}

wss.on('connection', (ws) => {
  let role = null; // set by handshake

  ws.on('message', (data) => {
    let msg;
    try {
      msg = JSON.parse(data);
    } catch {
      return;
    }

    switch (msg.type) {
      case 'handshake':
        role = msg.clientType;
        if (role === 'browser') {
          if (browser && browser !== ws) browser.close();
          browser = ws;
          log('browser connected');
        } else {
          clis.add(ws);
          log(`cli connected (${clis.size} total)`);
        }
        break;

      case 'control':
        // cli → browser
        if (!browser || browser.readyState !== browser.OPEN) {
          send(ws, {
            type: 'error',
            error: 'no browser connected — open http://localhost:' + PORT + ' and click the page',
            requestId: msg.requestId,
          });
          return;
        }
        send(browser, msg);
        break;

      case 'state':
      case 'error':
        // browser → all clis; the Go client matches replies by requestId.
        for (const cli of clis) send(cli, msg);
        break;
    }
  });

  ws.on('close', () => {
    if (ws === browser) {
      browser = null;
      log('browser disconnected');
    }
    if (clis.delete(ws)) log(`cli disconnected (${clis.size} left)`);
  });
});

function log(s) {
  console.log(`[bridge] ${s}`);
}

httpServer.listen(PORT, HOST, () => {
  log(`http + ws listening on http://${HOST}:${PORT}`);
  log('open it in a browser and click once to activate audio');
});
