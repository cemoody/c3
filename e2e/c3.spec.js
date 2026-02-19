const { execSync, spawn } = require('child_process');
const crypto = require('crypto');
const fs = require('fs');
const http = require('http');
const path = require('path');
const WebSocket = require('ws');
const assert = require('assert');

const C3_BIN = '/home/chris/c3/c3';
const PORT = 9097;
const BASE = `http://localhost:${PORT}`;

let c3Process;

function sleep(ms) {
  return new Promise(r => setTimeout(r, ms));
}

function httpGet(path) {
  return new Promise((resolve, reject) => {
    http.get(`${BASE}${path}`, (res) => {
      let data = '';
      res.on('data', chunk => data += chunk);
      res.on('end', () => resolve({ status: res.statusCode, headers: res.headers, body: data }));
    }).on('error', reject);
  });
}

function setupTmux(name) {
  try { execSync(`tmux kill-session -t ${name} 2>/dev/null`); } catch {}
  execSync(`tmux new-session -d -s ${name} -x 80 -y 24`);
}

function teardownTmux(name) {
  try { execSync(`tmux kill-session -t ${name} 2>/dev/null`); } catch {}
}

function tmuxSend(target, text) {
  execSync(`tmux send-keys -t ${target} "${text}" Enter`);
}

function tmuxCapture(target) {
  return execSync(`tmux capture-pane -t ${target} -p`).toString();
}

// Connect WS and start buffering messages immediately (before hello response arrives).
function connectWS(target) {
  return new Promise((resolve, reject) => {
    const ws = new WebSocket(`ws://localhost:${PORT}/s/${target}/ws`);
    ws._msgBuffer = [];
    ws._msgListeners = [];

    // Buffer all messages from the start
    ws.on('message', (data) => {
      const msg = JSON.parse(data.toString());
      ws._msgBuffer.push(msg);
      // Notify any pending readWSMessages
      for (const cb of ws._msgListeners) cb();
    });

    ws.on('open', () => {
      ws.send(JSON.stringify({ type: 'hello', replayMode: 'full' }));
      resolve(ws);
    });
    ws.on('error', reject);
  });
}

function readWSMessages(ws, timeout = 5000) {
  return new Promise((resolve) => {
    const timer = setTimeout(() => {
      const idx = ws._msgListeners.indexOf(check);
      if (idx >= 0) ws._msgListeners.splice(idx, 1);
      resolve([...ws._msgBuffer]);
    }, timeout);

    function check() {
      // Just let the timeout collect all messages
    }
    ws._msgListeners.push(check);
  });
}

let passed = 0;
let failed = 0;
const failures = [];

async function test(name, fn) {
  process.stdout.write(`  ${name} ... `);
  try {
    await fn();
    console.log('PASS');
    passed++;
  } catch (e) {
    console.log('FAIL');
    console.log(`    ${e.message}`);
    failed++;
    failures.push({ name, error: e.message });
  }
}

async function main() {
  console.log('Setting up...');
  setupTmux('e2e-test-1');
  setupTmux('e2e-test-2');

  c3Process = spawn(C3_BIN, ['--listen-addr', `:${PORT}`], {
    stdio: ['ignore', 'pipe', 'pipe'],
  });

  await sleep(2000);
  console.log('Server started.\n');
  console.log('Running e2e tests:\n');

  // ---- Tests ----

  await test('GET / returns HTML with session picker', async () => {
    const res = await httpGet('/');
    assert.strictEqual(res.status, 200);
    assert.ok(res.body.includes('<!DOCTYPE html>'), 'should be HTML');
    assert.ok(res.headers['content-type'].includes('text/html'), 'content-type should be text/html');
  });

  await test('GET /api/sessions returns session list JSON', async () => {
    const res = await httpGet('/api/sessions');
    assert.strictEqual(res.status, 200);
    const data = JSON.parse(res.body);
    assert.ok(Array.isArray(data.sessions), 'sessions should be an array');
    const names = data.sessions.map(s => s.name);
    assert.ok(names.includes('e2e-test-1'), 'should include e2e-test-1');
    assert.ok(names.includes('e2e-test-2'), 'should include e2e-test-2');
  });

  await test('GET /api/sessions contains pane targets', async () => {
    const res = await httpGet('/api/sessions');
    const data = JSON.parse(res.body);
    const sess = data.sessions.find(s => s.name === 'e2e-test-1');
    assert.ok(sess, 'e2e-test-1 session should exist');
    assert.ok(sess.windows.length > 0, 'should have windows');
    const pane = sess.windows[0].panes[0];
    assert.ok(pane.target, 'pane should have target');
    assert.strictEqual(pane.target, 'e2e-test-1:0.0');
    assert.ok(pane.currentCommand, 'pane should have currentCommand');
  });

  await test('GET /s/e2e-test-1:0.0/ returns SPA HTML (not 404)', async () => {
    const res = await httpGet('/s/e2e-test-1:0.0/');
    assert.strictEqual(res.status, 200);
    assert.ok(res.body.includes('<!DOCTYPE html>'), 'should serve index.html');
  });

  await test('GET /s/e2e-test-2:0.0/ also returns SPA HTML', async () => {
    const res = await httpGet('/s/e2e-test-2:0.0/');
    assert.strictEqual(res.status, 200);
    assert.ok(res.body.includes('<!DOCTYPE html>'));
  });

  await test('GET /s/nonexistent:0.0/ still returns SPA HTML (client handles error)', async () => {
    const res = await httpGet('/s/nonexistent:0.0/');
    assert.strictEqual(res.status, 200);
    assert.ok(res.body.includes('<!DOCTYPE html>'));
  });

  await test('WebSocket /s/e2e-test-1:0.0/ws connects and receives status', async () => {
    const ws = await connectWS('e2e-test-1:0.0');
    const messages = await readWSMessages(ws, 5000);
    ws.close();

    const types = messages.map(m => m.type);
    assert.ok(types.includes('status'), `should receive status message, got: ${types.join(', ')}`);

    const status = messages.find(m => m.type === 'status');
    assert.ok(status.epoch >= 0, 'epoch should be >= 0');
    assert.ok(status.paneState, 'should have paneState');
  });

  await test('status message includes pane dimensions', async () => {
    const ws = await connectWS('e2e-test-1:0.0');
    const messages = await readWSMessages(ws, 5000);
    ws.close();

    // Find the status message with dimensions (may be the second one)
    const statusWithDims = messages.find(m => m.type === 'status' && m.cols > 0);
    assert.ok(statusWithDims, 'should receive status message with cols/rows');
    assert.ok(statusWithDims.cols > 0, `cols should be > 0, got ${statusWithDims.cols}`);
    assert.ok(statusWithDims.rows > 0, `rows should be > 0, got ${statusWithDims.rows}`);
    console.log(`    pane dimensions: ${statusWithDims.cols}x${statusWithDims.rows}`);
  });

  await test('resize messages are ignored (no pane resize)', async () => {
    // Get current pane dimensions
    const before = execSync('tmux display-message -p -t e2e-test-1:0.0 "#{pane_width}x#{pane_height}"').toString().trim();

    const ws = await connectWS('e2e-test-1:0.0');
    await sleep(1000);

    // Send resize — should be ignored by the server
    ws.send(JSON.stringify({ type: 'resize', cols: 42, rows: 13 }));
    await sleep(1000);

    // Pane dimensions should NOT have changed
    const after = execSync('tmux display-message -p -t e2e-test-1:0.0 "#{pane_width}x#{pane_height}"').toString().trim();
    assert.strictEqual(before, after, `pane should not resize: was ${before}, now ${after}`);
    ws.close();
  });

  await test('WebSocket receives replay of existing output', async () => {
    // Seed output
    tmuxSend('e2e-test-1:0.0', 'echo e2e-replay-marker');
    await sleep(2000);

    const ws = await connectWS('e2e-test-1:0.0');
    const messages = await readWSMessages(ws, 3000);
    ws.close();

    const outputs = messages.filter(m => m.type === 'output');
    assert.ok(outputs.length > 0, 'should receive output messages');

    const allData = outputs.map(m => Buffer.from(m.data, 'base64').toString()).join('');
    assert.ok(allData.includes('e2e-replay-marker'), 'replay should contain seeded output');
  });

  await test('WebSocket input is delivered to tmux pane', async () => {
    const ws = await connectWS('e2e-test-1:0.0');
    await sleep(1000);

    // Send input
    const input = 'echo ws-e2e-input-test\n';
    const b64 = Buffer.from(input).toString('base64');
    ws.send(JSON.stringify({ type: 'input', data: b64 }));

    await sleep(2000);
    ws.close();

    // Check tmux pane
    const paneContent = tmuxCapture('e2e-test-1:0.0');
    assert.ok(paneContent.includes('ws-e2e-input-test'), 'input should appear in tmux pane');
  });

  await test('two sessions have independent WebSocket streams', async () => {
    // Pre-connect to create sessions and let pipe-pane start
    const preWs1 = await connectWS('e2e-test-1:0.0');
    const preWs2 = await connectWS('e2e-test-2:0.0');
    await sleep(3000); // let pipe-pane attach
    preWs1.close();
    preWs2.close();
    await sleep(500);

    // Now seed unique output in each session
    tmuxSend('e2e-test-1:0.0', 'echo session-1-unique');
    tmuxSend('e2e-test-2:0.0', 'echo session-2-unique');
    await sleep(3000);

    const ws1 = await connectWS('e2e-test-1:0.0');
    const ws2 = await connectWS('e2e-test-2:0.0');

    const msgs1 = await readWSMessages(ws1, 5000);
    const msgs2 = await readWSMessages(ws2, 5000);
    ws1.close();
    ws2.close();

    const data1 = msgs1.filter(m => m.type === 'output').map(m => Buffer.from(m.data, 'base64').toString()).join('');
    const data2 = msgs2.filter(m => m.type === 'output').map(m => Buffer.from(m.data, 'base64').toString()).join('');
    console.log(`    session-1: ${data1.length} bytes, session-2: ${data2.length} bytes`);

    assert.ok(data1.includes('session-1-unique'), `ws1 should have session-1 data (got ${data1.length} bytes)`);
    assert.ok(data2.includes('session-2-unique'), `ws2 should have session-2 data (got ${data2.length} bytes)`);
    assert.ok(!data1.includes('session-2-unique'), 'ws1 should NOT have session-2 data');
    assert.ok(!data2.includes('session-1-unique'), 'ws2 should NOT have session-1 data');
  });

  await test('WebSocket resize message does not crash server', async () => {
    const ws = await connectWS('e2e-test-1:0.0');
    await sleep(500);

    ws.send(JSON.stringify({ type: 'resize', cols: 120, rows: 40 }));
    ws.send(JSON.stringify({ type: 'resize', cols: 80, rows: 24 }));
    await sleep(500);

    // Server should still be responsive
    const res = await httpGet('/api/sessions');
    assert.strictEqual(res.status, 200);
    ws.close();
  });

  await test('static assets are served (CSS/JS)', async () => {
    // Get the HTML and extract the CSS/JS paths
    const html = (await httpGet('/')).body;
    const cssMatch = html.match(/href="(\/assets\/[^"]+\.css)"/);
    const jsMatch = html.match(/src="(\/assets\/[^"]+\.js)"/);

    if (cssMatch) {
      const cssRes = await httpGet(cssMatch[1]);
      assert.strictEqual(cssRes.status, 200);
      assert.ok(cssRes.headers['content-type'].includes('css'), 'CSS content-type');
    }
    if (jsMatch) {
      const jsRes = await httpGet(jsMatch[1]);
      assert.strictEqual(jsRes.status, 200);
      assert.ok(jsRes.headers['content-type'].includes('javascript'), 'JS content-type');
    }
  });

  // ---- Image upload tests (simulates clipboard paste flow) ----

  await test('image paste/upload saves file content-addressed and returns path', async () => {
    // Simulate what the browser paste handler does: POST multipart to /s/{target}/upload
    // Create a fake PNG (just bytes with PNG magic header)
    const pngHeader = Buffer.from([0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A]);
    const fakeImageData = Buffer.concat([pngHeader, crypto.randomBytes(256)]);
    const expectedHash = crypto.createHash('sha256').update(fakeImageData).digest('hex');

    // Build multipart form body
    const boundary = '----e2eTestBoundary' + Date.now();
    const body = Buffer.concat([
      Buffer.from(`--${boundary}\r\n`),
      Buffer.from(`Content-Disposition: form-data; name="image"; filename="paste.png"\r\n`),
      Buffer.from(`Content-Type: image/png\r\n\r\n`),
      fakeImageData,
      Buffer.from(`\r\n--${boundary}--\r\n`),
    ]);

    const result = await new Promise((resolve, reject) => {
      const req = http.request(`${BASE}/s/e2e-test-1:0.0/upload`, {
        method: 'POST',
        headers: {
          'Content-Type': `multipart/form-data; boundary=${boundary}`,
          'Content-Length': body.length,
        },
      }, (res) => {
        let data = '';
        res.on('data', chunk => data += chunk);
        res.on('end', () => resolve({ status: res.statusCode, body: data }));
      });
      req.on('error', reject);
      req.write(body);
      req.end();
    });

    assert.strictEqual(result.status, 200, `upload should return 200, got ${result.status}`);

    const json = JSON.parse(result.body);
    assert.ok(json.path, 'response should include file path');
    assert.strictEqual(json.hash, expectedHash, 'hash should match SHA-256 of uploaded data');
    console.log(`    saved to: ${json.path}`);

    // Verify file exists on disk
    assert.ok(fs.existsSync(json.path), `file should exist at ${json.path}`);

    // Verify file contents match
    const ondisk = fs.readFileSync(json.path);
    assert.ok(ondisk.equals(fakeImageData), 'file on disk should match uploaded data');
  });

  await test('duplicate image upload deduplicates (same hash, same file)', async () => {
    const imageData = Buffer.from('dedupe-test-image-data-12345');
    const expectedHash = crypto.createHash('sha256').update(imageData).digest('hex');

    async function uploadImage() {
      const boundary = '----e2eDedupe' + Date.now();
      const body = Buffer.concat([
        Buffer.from(`--${boundary}\r\n`),
        Buffer.from(`Content-Disposition: form-data; name="image"; filename="test.png"\r\n`),
        Buffer.from(`Content-Type: image/png\r\n\r\n`),
        imageData,
        Buffer.from(`\r\n--${boundary}--\r\n`),
      ]);
      return new Promise((resolve, reject) => {
        const req = http.request(`${BASE}/s/e2e-test-1:0.0/upload`, {
          method: 'POST',
          headers: {
            'Content-Type': `multipart/form-data; boundary=${boundary}`,
            'Content-Length': body.length,
          },
        }, (res) => {
          let data = '';
          res.on('data', chunk => data += chunk);
          res.on('end', () => resolve(JSON.parse(data)));
        });
        req.on('error', reject);
        req.write(body);
        req.end();
      });
    }

    const result1 = await uploadImage();
    const result2 = await uploadImage();

    assert.strictEqual(result1.hash, expectedHash);
    assert.strictEqual(result2.hash, expectedHash);
    assert.strictEqual(result1.path, result2.path, 'same image should resolve to same path');

    // Verify only one file on disk
    const dir = path.dirname(result1.path);
    const matching = fs.readdirSync(dir).filter(f => f.startsWith(expectedHash));
    assert.strictEqual(matching.length, 1, `should have exactly 1 file for hash, got ${matching.length}`);
  });

  await test('image upload rejects non-image file types', async () => {
    const boundary = '----e2eReject' + Date.now();
    const body = Buffer.concat([
      Buffer.from(`--${boundary}\r\n`),
      Buffer.from(`Content-Disposition: form-data; name="image"; filename="evil.txt"\r\n`),
      Buffer.from(`Content-Type: text/plain\r\n\r\n`),
      Buffer.from('not an image'),
      Buffer.from(`\r\n--${boundary}--\r\n`),
    ]);

    const result = await new Promise((resolve, reject) => {
      const req = http.request(`${BASE}/s/e2e-test-1:0.0/upload`, {
        method: 'POST',
        headers: {
          'Content-Type': `multipart/form-data; boundary=${boundary}`,
          'Content-Length': body.length,
        },
      }, (res) => {
        let data = '';
        res.on('data', chunk => data += chunk);
        res.on('end', () => resolve({ status: res.statusCode, body: data }));
      });
      req.on('error', reject);
      req.write(body);
      req.end();
    });

    assert.strictEqual(result.status, 400, `should reject .txt upload with 400, got ${result.status}`);
  });

  await test('image upload injects prompt into tmux pane', async () => {
    // First, connect WS so pipe-pane is running and we capture output
    const ws = await connectWS('e2e-test-1:0.0');
    await sleep(3000); // let pipe-pane start

    const imageData = Buffer.from('prompt-injection-test-' + Date.now());
    const boundary = '----e2ePrompt' + Date.now();
    const body = Buffer.concat([
      Buffer.from(`--${boundary}\r\n`),
      Buffer.from(`Content-Disposition: form-data; name="image"; filename="screenshot.png"\r\n`),
      Buffer.from(`Content-Type: image/png\r\n\r\n`),
      imageData,
      Buffer.from(`\r\n--${boundary}--\r\n`),
    ]);

    await new Promise((resolve, reject) => {
      const req = http.request(`${BASE}/s/e2e-test-1:0.0/upload`, {
        method: 'POST',
        headers: {
          'Content-Type': `multipart/form-data; boundary=${boundary}`,
          'Content-Length': body.length,
        },
      }, (res) => {
        let data = '';
        res.on('data', chunk => data += chunk);
        res.on('end', () => resolve(JSON.parse(data)));
      });
      req.on('error', reject);
      req.write(body);
      req.end();
    });

    await sleep(2000);

    // Check that "Analyze this image:" was injected into the pane
    const paneContent = tmuxCapture('e2e-test-1:0.0');
    assert.ok(
      paneContent.includes('Analyze this image:'),
      'pane should contain the injected image prompt'
    );

    ws.close();
  });

  // ---- Summary ----

  console.log(`\n${passed + failed} tests: ${passed} passed, ${failed} failed`);
  if (failures.length > 0) {
    console.log('\nFailures:');
    failures.forEach(f => console.log(`  - ${f.name}: ${f.error}`));
  }

  // Cleanup
  c3Process.kill('SIGTERM');
  await sleep(500);
  teardownTmux('e2e-test-1');
  teardownTmux('e2e-test-2');

  process.exit(failed > 0 ? 1 : 0);
}

main().catch(e => {
  console.error('Fatal error:', e);
  if (c3Process) c3Process.kill('SIGTERM');
  process.exit(1);
});
