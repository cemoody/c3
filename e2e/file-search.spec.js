const { test, expect } = require('@playwright/test');
const { execSync, spawn } = require('child_process');
const fs = require('fs');

const C3_BIN = '/home/chris/c3/c3';
const PORT = 9099;
const BASE = `http://localhost:${PORT}`;
const SESSION = 'e2e-search';

let c3Process;

test.beforeAll(async () => {
  // Create tmux session for c3
  try { execSync(`tmux kill-session -t ${SESSION} 2>/dev/null`); } catch {}
  execSync(`tmux new-session -d -s ${SESSION} -x 120 -y 40`);

  // Create test files in /tmp/ (c3 indexer scans /tmp/)
  const pngHeader = Buffer.from([0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A]);
  for (let i = 0; i < 3; i++) {
    fs.writeFileSync(`/tmp/e2e-srch-img-${i}.png`, Buffer.concat([pngHeader, Buffer.alloc(64, 0xAB)]));
  }
  // HTML file mixed in — must NOT break preview navigation
  fs.writeFileSync('/tmp/e2e-srch-img-page.html', '<html><body>test</body></html>');

  // Start c3
  c3Process = spawn(C3_BIN, ['--listen-addr', `:${PORT}`], {
    stdio: ['ignore', 'pipe', 'pipe'],
  });

  // Wait for c3 to start and indexer to do initial scan
  for (let attempt = 0; attempt < 10; attempt++) {
    await new Promise(r => setTimeout(r, 2000));
    try {
      const res = await fetch(`${BASE}/api/search?q=e2e-srch-img`);
      const data = await res.json();
      if (data.results && data.results.length >= 3) break;
    } catch {}
  }
});

test.afterAll(async () => {
  if (c3Process) c3Process.kill('SIGTERM');
  await new Promise(r => setTimeout(r, 500));
  try { execSync(`tmux kill-session -t ${SESSION} 2>/dev/null`); } catch {}
  for (let i = 0; i < 3; i++) {
    try { fs.unlinkSync(`/tmp/e2e-srch-img-${i}.png`); } catch {}
  }
  try { fs.unlinkSync('/tmp/e2e-srch-img-page.html'); } catch {}
});

test('clicking image search result in file browser opens preview', async ({ page }) => {
  await page.goto(`${BASE}/files/`);
  await page.waitForSelector('.file-browser', { timeout: 10000 });

  // Activate search and type query
  const searchInput = page.locator('.search-input');
  await searchInput.click();
  await searchInput.fill('e2e-srch-img');

  // Wait for search results to appear
  const firstResult = page.locator('.search-result').first();
  await expect(firstResult).toBeVisible({ timeout: 10000 });

  // Verify it's a PNG file
  const filename = firstResult.locator('.result-filename');
  await expect(filename).toContainText('.png');

  // Click the first search result
  await firstResult.click();

  // Preview overlay should appear with an image
  const previewOverlay = page.locator('.preview-overlay');
  await expect(previewOverlay).toBeVisible({ timeout: 5000 });
  const previewImg = previewOverlay.locator('img');
  await expect(previewImg).toBeVisible({ timeout: 5000 });

  // Close preview by clicking close button
  await page.locator('.close-btn').click();
  await expect(previewOverlay).not.toBeVisible({ timeout: 3000 });

  // Click second search result — should still work
  const secondResult = page.locator('.search-result').nth(1);
  await secondResult.click();
  await expect(previewOverlay).toBeVisible({ timeout: 5000 });
  await expect(previewOverlay.locator('img')).toBeVisible({ timeout: 5000 });
});

test('arrow-key navigation skips HTML files and does not open new tabs', async ({ page }) => {
  await page.goto(`${BASE}/files/`);
  await page.waitForSelector('.file-browser', { timeout: 10000 });

  // Search for files that include both PNG and HTML results
  const searchInput = page.locator('.search-input');
  await searchInput.click();
  await searchInput.fill('e2e-srch-img');

  // Wait for results
  const firstResult = page.locator('.search-result').first();
  await expect(firstResult).toBeVisible({ timeout: 10000 });

  // Open the first result (should be a PNG) via Enter
  await searchInput.press('Enter');
  const previewOverlay = page.locator('.preview-overlay');
  await expect(previewOverlay).toBeVisible({ timeout: 5000 });

  // Count pages before arrow navigation (to detect unwanted window.open)
  const pagesBefore = page.context().pages().length;

  // Navigate forward through results with arrow key
  await previewOverlay.press('ArrowRight');
  await page.waitForTimeout(500);

  // Preview should still be visible (not broken by HTML file in results)
  await expect(previewOverlay).toBeVisible();

  // No new tabs should have opened from HTML files
  const pagesAfter = page.context().pages().length;
  expect(pagesAfter).toBe(pagesBefore);

  // Close and verify search results are still clickable
  await page.locator('.close-btn').click();
  await expect(previewOverlay).not.toBeVisible({ timeout: 3000 });

  // Click a result — should still work after arrow navigation
  const lastPngResult = page.locator('.search-result').nth(2);
  await lastPngResult.click();
  await expect(previewOverlay).toBeVisible({ timeout: 5000 });
});
