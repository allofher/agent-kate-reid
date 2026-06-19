// Smoke test for the relay hub: fakes both a browser and a cli client and
// checks the full round trip cli → server → browser → server → cli.
// Run with the server already up, or standalone: `npm test` (starts its own).
//
// Exercises: handshake routing, control relay, requestId echo, the
// no-browser error path, and toggle/stop semantics.

import { spawn } from 'node:child_process';
import WebSocket from 'ws';

const PORT = 8099; // dedicated test port so a live session on 8081 is untouched
const URL = `ws://127.0.0.1:${PORT}`;

const server = spawn('node', ['server.mjs'], {
  env: { ...process.env, BRIDGE_PORT: PORT },
  stdio: 'ignore',
});

const fail = (msg) => {
  console.error(`FAIL: ${msg}`);
  server.kill();
  process.exit(1);
};

const connect = (clientType) =>
  new Promise((resolve, reject) => {
    const ws = new WebSocket(URL);
    ws.on('open', () => {
      ws.send(JSON.stringify({ type: 'handshake', clientType }));
      resolve(ws);
    });
    ws.on('error', reject);
  });

const nextMsg = (ws) =>
  new Promise((resolve) => ws.once('message', (d) => resolve(JSON.parse(d))));

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

// wait for the server to come up
for (let i = 0; ; i++) {
  try {
    await connect('probe').then((ws) => ws.close());
    break;
  } catch {
    if (i > 20) fail('server did not start');
    await sleep(100);
  }
}

// 1. cli with no browser connected → error reply with requestId echoed
{
  const cli = await connect('cli');
  cli.send(JSON.stringify({ type: 'control', action: 'getState', requestId: 'req_0' }));
  const msg = await nextMsg(cli);
  if (msg.type !== 'error' || msg.requestId !== 'req_0') {
    fail(`expected error reply without browser, got ${JSON.stringify(msg)}`);
  }
  cli.close();
}

// 2. full round trip: fake browser answers evaluate with a state message
{
  const browser = await connect('browser');
  const cli = await connect('cli');

  // fake browser: echo every control as a state reply, like public/index.html
  let lastCode = '';
  browser.on('message', (d) => {
    const msg = JSON.parse(d);
    if (msg.type !== 'control') return;
    if (msg.action === 'evaluate') lastCode = msg.payload.code;
    browser.send(JSON.stringify({
      type: 'state',
      payload: { code: lastCode, playing: msg.action !== 'stop', error: null, timestamp: Date.now() },
      requestId: msg.requestId,
    }));
  });

  cli.send(JSON.stringify({
    type: 'control',
    action: 'evaluate',
    payload: { code: '$: s("bd sd") // drums' },
    requestId: 'req_1',
  }));
  const st = await nextMsg(cli);
  if (st.type !== 'state' || st.requestId !== 'req_1' || st.payload.code !== '$: s("bd sd") // drums' || st.payload.playing !== true) {
    fail(`bad evaluate round trip: ${JSON.stringify(st)}`);
  }

  cli.send(JSON.stringify({ type: 'control', action: 'stop', requestId: 'req_2' }));
  const st2 = await nextMsg(cli);
  if (st2.requestId !== 'req_2' || st2.payload.playing !== false) {
    fail(`bad stop round trip: ${JSON.stringify(st2)}`);
  }

  // 3. browser disconnect → cli gets error again
  browser.close();
  await sleep(100);
  cli.send(JSON.stringify({ type: 'control', action: 'getState', requestId: 'req_3' }));
  const st3 = await nextMsg(cli);
  if (st3.type !== 'error' || st3.requestId !== 'req_3') {
    fail(`expected error after browser disconnect, got ${JSON.stringify(st3)}`);
  }
  cli.close();
}

console.log('PASS: all relay round trips OK');
server.kill();
process.exit(0);
