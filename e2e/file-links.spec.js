const { test, expect } = require('@playwright/test');
const { execSync, spawn } = require('child_process');
const fs = require('fs');

const C3_BIN = '/home/chris/c3/c3';
const PORT = 9098;
const BASE = `http://localhost:${PORT}`;
const SESSION = 'e2e-filelink';
const TARGET = `${SESSION}:0.0`;

let c3Process;

test.beforeAll(async () => {
  try { execSync(`tmux kill-session -t ${SESSION} 2>/dev/null`); } catch {}
  execSync(`tmux new-session -d -s ${SESSION} -x 120 -y 40`);

  // Create test files
  const pngHeader = Buffer.from([0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A]);
  fs.writeFileSync('/tmp/e2e-test-image.png', Buffer.concat([pngHeader, Buffer.alloc(64, 0xAB)]));
  fs.writeFileSync('/tmp/e2e-test-file.txt', 'Hello from e2e test!\nLine two.\n');

  c3Process = spawn(C3_BIN, ['--listen-addr', `:${PORT}`], {
    stdio: ['ignore', 'pipe', 'pipe'],
  });
  await new Promise(r => setTimeout(r, 2000));
});

test.afterAll(async () => {
  if (c3Process) c3Process.kill('SIGTERM');
  await new Promise(r => setTimeout(r, 500));
  try { execSync(`tmux kill-session -t ${SESSION} 2>/dev/null`); } catch {}
  try { fs.unlinkSync('/tmp/e2e-test-image.png'); } catch {}
  try { fs.unlinkSync('/tmp/e2e-test-file.txt'); } catch {}
});

// Helper: find a file path in terminal output and click it
async function clickFilePath(page, filePath) {
  const found = await page.evaluate((fp) => {
    const rows = document.querySelectorAll('.xterm-rows > div');
    for (let i = 0; i < rows.length; i++) {
      const text = rows[i].textContent || '';
      if (text.includes(fp)) {
        return { index: i, text, pathPos: text.indexOf(fp) };
      }
    }
    return null;
  }, filePath);

  expect(found, `Should find "${filePath}" in terminal output`).not.toBeNull();

  const row = page.locator('.xterm-rows > div').nth(found.index);
  const rowBox = await row.boundingBox();
  const charWidth = rowBox.width / 120;
  const pathMid = found.pathPos + filePath.length / 2;
  const clickX = rowBox.x + pathMid * charWidth;
  const clickY = rowBox.y + rowBox.height / 2;

  await page.mouse.move(clickX, clickY);
  await page.waitForTimeout(300);
  await page.mouse.click(clickX, clickY);
}

test('file path in terminal output is clickable and opens image preview', async ({ page }) => {
  await page.goto(`${BASE}/s/${encodeURIComponent(TARGET)}/`);
  await page.waitForSelector('.xterm-screen', { timeout: 10000 });
  await page.waitForTimeout(3000);

  execSync(`tmux send-keys -t ${TARGET} 'echo /tmp/e2e-test-image.png' Enter`);
  await page.waitForTimeout(3000);

  await clickFilePath(page, '/tmp/e2e-test-image.png');

  const modal = page.locator('.fp-backdrop');
  await expect(modal).toBeVisible({ timeout: 5000 });
  await expect(page.locator('.fp-filename')).toHaveText('e2e-test-image.png');
  await expect(page.locator('.fp-image')).toBeVisible();

  await page.locator('.fp-close').click();
  await expect(modal).not.toBeVisible();
});

test('file path click opens text file preview', async ({ page }) => {
  await page.goto(`${BASE}/s/${encodeURIComponent(TARGET)}/`);
  await page.waitForSelector('.xterm-screen', { timeout: 10000 });
  await page.waitForTimeout(3000);

  execSync(`tmux send-keys -t ${TARGET} 'echo /tmp/e2e-test-file.txt' Enter`);
  await page.waitForTimeout(3000);

  await clickFilePath(page, '/tmp/e2e-test-file.txt');

  const modal = page.locator('.fp-backdrop');
  await expect(modal).toBeVisible({ timeout: 5000 });
  await expect(page.locator('.fp-filename')).toHaveText('e2e-test-file.txt');
  await expect(page.locator('.fp-text')).toBeVisible();
  await expect(page.locator('.fp-text')).toContainText('Hello from e2e test!');

  // Close via Escape
  await page.keyboard.press('Escape');
  await expect(modal).not.toBeVisible();
});

test('preview modal has download button', async ({ page }) => {
  await page.goto(`${BASE}/s/${encodeURIComponent(TARGET)}/`);
  await page.waitForSelector('.xterm-screen', { timeout: 10000 });
  await page.waitForTimeout(3000);

  execSync(`tmux send-keys -t ${TARGET} 'echo /tmp/e2e-test-image.png' Enter`);
  await page.waitForTimeout(3000);

  await clickFilePath(page, '/tmp/e2e-test-image.png');

  const modal = page.locator('.fp-backdrop');
  await expect(modal).toBeVisible({ timeout: 5000 });

  const downloadLink = page.locator('.fp-download');
  await expect(downloadLink).toBeVisible();
  const href = await downloadLink.getAttribute('href');
  expect(href).toContain('/api/files/raw');
  expect(href).toContain('e2e-test-image.png');

  await page.locator('.fp-close').click();
});
