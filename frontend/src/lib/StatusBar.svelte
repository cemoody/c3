<script lang="ts">
  import { onMount } from 'svelte';
  import type { ConnectionState, PaneState } from './websocket';
  import TabManager from './TabManager.svelte';

  type Pane = { index: string; currentCommand: string; target: string };
  type Window = { index: string; name: string; panes: Pane[] };
  type Session = { name: string; windows: Window[] };

  let {
    connectionState = 'disconnected',
    paneState = 'unknown',
    target = '',
    pageMode = 'session',
  }: {
    connectionState?: ConnectionState;
    paneState?: PaneState;
    target?: string;
    pageMode?: string;
  } = $props();

  let sessions = $state<Session[]>([]);
  let allTargets = $state<{target: string; label: string; windowName: string; command: string}[]>([]);

  const isMobile = /iPhone|iPad|iPod|Android/i.test(navigator.userAgent);

  const stateColors: Record<ConnectionState, string> = {
    live: 'var(--success)',
    replaying: 'var(--warning)',
    connecting: 'var(--warning)',
    disconnected: 'var(--error)',
    error: 'var(--error)',
  };

  const stateLabels: Record<ConnectionState, string> = {
    live: 'Live',
    replaying: 'Replaying...',
    connecting: 'Connecting...',
    disconnected: 'Disconnected',
    error: 'Error',
  };

  let editingTarget = $state<string | null>(null);
  let editValue = $state('');

  let renameCommitted = false;

  // Tab Manager modal state
  let tabManagerOpen = $state(false);

  // Custom tab ordering from localStorage
  const TAB_ORDER_KEY = 'c3-tab-order';

  function loadTabOrder(): string[] {
    try {
      const saved = localStorage.getItem(TAB_ORDER_KEY);
      return saved ? JSON.parse(saved) : [];
    } catch { return []; }
  }

  function saveTabOrder(order: string[]) {
    localStorage.setItem(TAB_ORDER_KEY, JSON.stringify(order));
  }

  function applyTabOrder(targets: typeof allTargets): typeof allTargets {
    const order = loadTabOrder();
    if (order.length === 0) return targets;

    const orderMap = new Map(order.map((t, i) => [t, i]));
    return [...targets].sort((a, b) => {
      const ai = orderMap.get(a.target) ?? Infinity;
      const bi = orderMap.get(b.target) ?? Infinity;
      if (ai === Infinity && bi === Infinity) return 0;
      return ai - bi;
    });
  }

  async function startRename(t: {target: string; label: string; windowName: string}, e: MouseEvent) {
    // Only allow renaming the active tab
    if (t.target !== target || pageMode !== 'session') return;
    e.preventDefault();
    e.stopPropagation();
    editingTarget = t.target;
    editValue = t.windowName;
    renameCommitted = false;
    // Focus the input after Svelte updates the DOM
    await new Promise(r => setTimeout(r, 50));
    const input = document.querySelector('.tab-rename-input') as HTMLInputElement;
    if (input) {
      input.focus();
      input.select();
    }
  }

  async function commitRename() {
    // Guard against double-commit from blur + Enter
    if (renameCommitted || !editingTarget) return;
    renameCommitted = true;

    const name = editValue.trim();
    const tgt = editingTarget;
    editingTarget = null;

    if (!name) return;

    try {
      await fetch('/api/rename', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ target: tgt, name }),
      });
      await fetchSessions();
    } catch {}
  }

  function handleRenameKeydown(e: KeyboardEvent) {
    if (e.key === 'Enter') {
      e.preventDefault();
      commitRename();
    } else if (e.key === 'Escape') {
      editingTarget = null;
    }
  }

  async function doRename(tgt: string, name: string) {
    try {
      await fetch('/api/rename', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ target: tgt, name }),
      });
      await fetchSessions();
    } catch {}
  }

  async function doNewSession(name: string) {
    const res = await fetch('/api/new-session', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name }),
    });
    if (!res.ok) throw new Error('Failed to create session');
    await fetchSessions();
    // Navigate to the new session's first pane
    const newTarget = allTargets.find(t => t.target.startsWith(name + ':'));
    if (newTarget) {
      window.location.href = `/s/${encodeURIComponent(newTarget.target)}/`;
    }
  }

  async function doKill(tgt: string) {
    try {
      const res = await fetch('/api/kill-window', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ target: tgt }),
      });
      if (!res.ok) return;
      // Brief delay for tmux to fully clean up before refreshing
      await new Promise(r => setTimeout(r, 300));
      await fetchSessions();
    } catch {}
  }

  function handleReorder(ordered: string[]) {
    saveTabOrder(ordered);
    allTargets = applyTabOrder(allTargets);
  }

  async function fetchSessions() {
    try {
      const res = await fetch('/api/sessions');
      if (!res.ok) return;
      const data = await res.json();
      sessions = data.sessions || [];
      // Flatten into a list of targets
      const targets: typeof allTargets = [];
      for (const sess of sessions) {
        for (const win of sess.windows) {
          for (const pane of win.panes) {
            targets.push({
              target: pane.target,
              label: win.name,
              windowName: win.name,
              command: pane.currentCommand,
            });
          }
        }
      }
      allTargets = applyTabOrder(targets);
      updatePrefetch();
    } catch {}
  }

  function navigateTo(t: string) {
    window.location.href = `/s/${encodeURIComponent(t)}/`;
  }

  // All navigable pages: Files + session targets
  function allPages(): string[] {
    return ['/files/', ...allTargets.map(t => `/s/${encodeURIComponent(t.target)}/`)];
  }

  function currentPageIndex(): number {
    if (pageMode === 'files') return 0;
    const idx = allTargets.findIndex(t => t.target === target);
    return idx >= 0 ? idx + 1 : 0;
  }

  // Prefetch adjacent pages for instant tab switching
  function updatePrefetch() {
    // Remove old prefetch links
    document.querySelectorAll('link[data-c3-prefetch]').forEach(el => el.remove());

    const pages = allPages();
    if (pages.length <= 1) return;
    const idx = currentPageIndex();
    const prev = (idx - 1 + pages.length) % pages.length;
    const next = (idx + 1) % pages.length;

    for (const url of [pages[prev], pages[next]]) {
      const link = document.createElement('link');
      link.rel = 'prefetch';
      link.href = url;
      link.setAttribute('data-c3-prefetch', '');
      document.head.appendChild(link);
    }
  }

  function cyclePage(delta: number) {
    const pages = allPages();
    if (pages.length === 0) return;
    const idx = currentPageIndex();
    const next = (idx + delta + pages.length) % pages.length;
    window.location.href = pages[next];
  }

  function handleKeydown(e: KeyboardEvent) {
    // Cmd+[ / Cmd+] on Mac (metaKey), Ctrl+[ / Ctrl+] elsewhere
    if (e.metaKey || e.ctrlKey) {
      if (e.key === '[') {
        e.preventDefault();
        e.stopPropagation();
        cyclePage(-1);
      } else if (e.key === ']') {
        e.preventDefault();
        e.stopPropagation();
        cyclePage(1);
      }
    }
  }

  // When pane goes missing, refresh sessions and navigate to next tab
  let lastPaneState = $state(paneState);
  $effect(() => {
    if (paneState === 'missing' && lastPaneState !== 'missing' && pageMode === 'session') {
      // Pane was just destroyed — refresh and navigate away
      setTimeout(async () => {
        await fetchSessions();
        // If this target no longer exists, go to next tab or home
        const stillExists = allTargets.some(t => t.target === target);
        if (!stillExists) {
          const pages = allPages();
          if (pages.length > 1) {
            // Navigate to the next available page (skip the dead one)
            window.location.href = pages[0];
          } else {
            window.location.href = '/';
          }
        }
      }, 1000); // Brief delay for tmux to fully clean up
    }
    lastPaneState = paneState;
  });

  onMount(() => {
    fetchSessions();
    const interval = setInterval(fetchSessions, 10000);
    document.addEventListener('keydown', handleKeydown);
    return () => {
      clearInterval(interval);
      document.removeEventListener('keydown', handleKeydown);
    };
  });
</script>

<div class="tab-bar">
  <div class="tabs">
    <a
      class="tab files-tab"
      class:active={pageMode === 'files'}
      href="/files/"
    >
      <span class="tab-label">Files</span>
    </a>
    <button
      class="tab tabs-btn"
      onclick={() => tabManagerOpen = !tabManagerOpen}
    >
      <span class="tab-label">Tabs</span>
    </button>
    {#each allTargets as t}
      <a
        class="tab"
        class:active={t.target === target && pageMode === 'session'}
        href="/s/{encodeURIComponent(t.target)}/"
        title="{t.target} — {t.command}"
      >
        {#if editingTarget === t.target}
          <!-- svelte-ignore a11y_autofocus -->
          <input
            class="tab-rename-input"
            type="text"
            bind:value={editValue}
            onkeydown={handleRenameKeydown}
            onblur={commitRename}
            autofocus
            onclick={(e) => { e.preventDefault(); e.stopPropagation(); }}
          />
        {:else}
          <span
            class="tab-label"
            role="textbox"
            tabindex="0"
            onclick={(e) => { if (t.target === target && pageMode === 'session') { e.preventDefault(); e.stopPropagation(); startRename(t, e); } }}
          >{t.label}</span>
        {/if}
      </a>
    {/each}
  </div>
  <span class="status-indicator">
    <span class="dot" style:background={stateColors[connectionState]}></span>
    {#if !isMobile}
      <span class="status-label">{stateLabels[connectionState]}</span>
    {/if}
  </span>
  {#if !isMobile}
    <span class="hint">&#8984;[ / &#8984;]</span>
  {/if}
</div>

{#if tabManagerOpen}
  <TabManager
    targets={allTargets}
    activeTarget={target}
    onClose={() => tabManagerOpen = false}
    onNavigate={navigateTo}
    onReorder={handleReorder}
    onRename={doRename}
    onNewSession={doNewSession}
    onKill={doKill}
  />
{/if}

<style>
  .tab-bar {
    display: flex;
    align-items: center;
    gap: 4px;
    padding: 3px 6px;
    background: var(--bg-secondary);
    border-bottom: 1px solid var(--border);
    flex-shrink: 0;
    overflow-x: auto;
    scrollbar-width: none;
  }
  .tab-bar::-webkit-scrollbar {
    display: none;
  }
  .tabs {
    display: flex;
    gap: 2px;
    flex: 1;
    min-width: 0;
  }
  .tab {
    display: flex;
    align-items: center;
    gap: 4px;
    padding: 3px 10px;
    background: transparent;
    border: 1px solid transparent;
    border-radius: 4px;
    color: var(--fg-dim);
    font-size: 11px;
    font-family: inherit;
    white-space: nowrap;
    flex-shrink: 0;
    text-decoration: none;
    cursor: pointer;
  }
  .tab:hover {
    background: var(--bg);
    border-color: var(--border);
  }
  .tab.active {
    background: var(--bg);
    border-color: var(--accent);
    color: var(--fg);
    font-weight: 600;
  }
  .tab-label {
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .tab-rename-input {
    width: 80px;
    padding: 1px 4px;
    font-size: 11px;
    font-family: inherit;
    background: var(--bg);
    color: var(--fg);
    border: 1px solid var(--accent);
    border-radius: 2px;
    outline: none;
  }
  .files-tab {
    border-right: 1px solid var(--border);
    margin-right: 4px;
    padding-right: 12px;
  }
  .tabs-btn {
    border-right: 1px solid var(--border);
    margin-right: 4px;
    padding-right: 12px;
  }
  .status-indicator {
    display: flex;
    align-items: center;
    gap: 5px;
    flex-shrink: 0;
    padding: 0 4px;
  }
  .status-label {
    font-size: 11px;
    color: var(--fg-dim);
  }
  .dot {
    width: 7px;
    height: 7px;
    border-radius: 50%;
    flex-shrink: 0;
  }
  .hint {
    color: var(--fg-dim);
    font-size: 10px;
    white-space: nowrap;
    flex-shrink: 0;
    opacity: 0.5;
  }
</style>
