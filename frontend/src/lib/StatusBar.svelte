<script lang="ts">
  import { onMount } from 'svelte';
  import type { ConnectionState, PaneState } from './websocket';

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
  let allTargets = $state<{target: string; label: string; command: string}[]>([]);

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
              label: `${sess.name}:${win.name}`,
              command: pane.currentCommand,
            });
          }
        }
      }
      allTargets = targets;
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
    {#each allTargets as t}
      <a
        class="tab"
        class:active={t.target === target && pageMode === 'session'}
        href="/s/{encodeURIComponent(t.target)}/"
        title="{t.target} — {t.command}"
      >
        <span class="tab-label">{t.label}</span>
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
  .files-tab {
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
