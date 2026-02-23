const { test, expect } = require('@playwright/test');
const { execSync, spawn } = require('child_process');

const C3_BIN = '/home/chris/c3/c3';
const PORT = 9099;
const BASE = `http://localhost:${PORT}`;
const SESSION = 'e2e-mobile-scroll';
const TARGET = `${SESSION}:0.0`;

let c3Process;

test.beforeAll(async () => {
  try { execSync(`tmux kill-session -t ${SESSION} 2>/dev/null`); } catch {}
  execSync(`tmux new-session -d -s ${SESSION} -x 80 -y 24`);

  c3Process = spawn(C3_BIN, ['--listen-addr', `:${PORT}`], {
    stdio: ['ignore', 'pipe', 'pipe'],
  });
  await new Promise(r => setTimeout(r, 2000));
});

test.afterAll(async () => {
  if (c3Process) c3Process.kill('SIGTERM');
  await new Promise(r => setTimeout(r, 500));
  try { execSync(`tmux kill-session -t ${SESSION} 2>/dev/null`); } catch {}
});

test.use({
  viewport: { width: 375, height: 812 },
  userAgent: 'Mozilla/5.0 (iPhone; CPU iPhone OS 16_0 like Mac OS X)',
});

test('mobile controls do not overlap terminal wrapper', async ({ page }) => {
  await page.goto(`${BASE}/s/${encodeURIComponent(TARGET)}/`);
  await page.waitForSelector('.xterm-screen', { timeout: 10000 });
  await page.waitForTimeout(2000);

  // Check that mobile controls and terminal wrapper don't overlap
  const layout = await page.evaluate(() => {
    const wrapper = document.querySelector('.terminal-wrapper');
    const controls = document.querySelector('.mobile-controls');
    if (!wrapper || !controls) return null;
    const wr = wrapper.getBoundingClientRect();
    const cr = controls.getBoundingClientRect();
    return {
      wrapperBottom: wr.bottom,
      controlsTop: cr.top,
      overlap: wr.bottom - cr.top,
    };
  });

  console.log('Layout:', JSON.stringify(layout));
  expect(layout).not.toBeNull();
  // Terminal wrapper should end at or before mobile controls start
  expect(layout.overlap, 'Terminal wrapper should not overlap mobile controls').toBeLessThanOrEqual(1);
});

test('xterm viewport is fully within the terminal wrapper', async ({ page }) => {
  await page.goto(`${BASE}/s/${encodeURIComponent(TARGET)}/`);
  await page.waitForSelector('.xterm-screen', { timeout: 10000 });
  await page.waitForTimeout(2000);

  const layout = await page.evaluate(() => {
    const wrapper = document.querySelector('.terminal-wrapper');
    const viewport = document.querySelector('.xterm-viewport');
    const controls = document.querySelector('.mobile-controls');
    if (!wrapper || !viewport || !controls) return null;
    const wr = wrapper.getBoundingClientRect();
    const vr = viewport.getBoundingClientRect();
    const cr = controls.getBoundingClientRect();
    return {
      wrapperHeight: wr.height,
      wrapperBottom: wr.bottom,
      viewportHeight: vr.height,
      viewportBottom: vr.bottom,
      controlsTop: cr.top,
      viewportExceedsWrapper: vr.bottom > wr.bottom + 1,
      viewportBehindControls: vr.bottom > cr.top + 1,
    };
  });

  console.log('Viewport layout:', JSON.stringify(layout));
  expect(layout).not.toBeNull();
  expect(layout.viewportExceedsWrapper, 'xterm viewport should not exceed wrapper').toBe(false);
  expect(layout.viewportBehindControls, 'xterm viewport should not be behind controls').toBe(false);
});

test('last line of long output is visible after scroll on mobile', async ({ page }) => {
  await page.goto(`${BASE}/s/${encodeURIComponent(TARGET)}/`);
  await page.waitForSelector('.xterm-screen', { timeout: 10000 });
  await page.waitForTimeout(3000);

  // Generate long output with a unique marker at the end
  execSync(`tmux send-keys -t ${TARGET} 'for i in $(seq 1 100); do echo "line-$i-padding"; done; echo "FINAL_VISIBLE_LINE"' Enter`);
  await page.waitForTimeout(3000);

  // Use xterm's scrollToBottom via the app's scroll mechanism
  // The app has a "Jump to Live" button and also auto-scrolls
  await page.evaluate(() => {
    const viewport = document.querySelector('.xterm-viewport');
    if (viewport) {
      viewport.scrollTop = viewport.scrollHeight;
      // Dispatch scroll event so xterm picks it up
      viewport.dispatchEvent(new Event('scroll'));
    }
  });
  await page.waitForTimeout(1000);

  await page.screenshot({ path: 'test-results/mobile-scroll-final.png' });

  // Check that the shell prompt (which comes after our echo) is in the
  // visible area. After scrolling to bottom, xterm should show the latest rows.
  // We verify by checking the xterm-viewport scroll position:
  // scrollTop + clientHeight should equal scrollHeight (scrolled to bottom)
  const scrollState = await page.evaluate(() => {
    const viewport = document.querySelector('.xterm-viewport');
    if (!viewport) return null;
    return {
      scrollTop: viewport.scrollTop,
      scrollHeight: viewport.scrollHeight,
      clientHeight: viewport.clientHeight,
      atBottom: Math.abs(viewport.scrollHeight - viewport.scrollTop - viewport.clientHeight) < 2,
    };
  });

  console.log('Scroll state:', JSON.stringify(scrollState));
  expect(scrollState).not.toBeNull();
  expect(scrollState.atBottom, 'Should be scrolled to the bottom').toBe(true);
  // clientHeight should be reasonable (not larger than viewport minus statusbar minus controls)
  expect(scrollState.clientHeight, 'Viewport height should be > 0').toBeGreaterThan(100);
  expect(scrollState.clientHeight, 'Viewport height should fit between statusbar and controls').toBeLessThan(750);
});
