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
  }: {
    connectionState?: ConnectionState;
    paneState?: PaneState;
    target?: string;
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
    } catch {}
  }

  function navigateTo(t: string) {
    window.location.href = `/s/${encodeURIComponent(t)}/`;
  }

  function cycleTab(delta: number) {
    if (allTargets.length === 0) return;
    const idx = allTargets.findIndex(t => t.target === target);
    const next = (idx + delta + allTargets.length) % allTargets.length;
    navigateTo(allTargets[next].target);
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.ctrlKey && e.shiftKey) {
      if (e.key === '<' || e.key === ',') {
        e.preventDefault();
        e.stopPropagation();
        cycleTab(-1);
      } else if (e.key === '>' || e.key === '.') {
        e.preventDefault();
        e.stopPropagation();
        cycleTab(1);
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
    {#each allTargets as t}
      <button
        class="tab"
        class:active={t.target === target}
        onclick={() => navigateTo(t.target)}
        title="{t.target} — {t.command}"
      >
        <span class="tab-label">{t.label}</span>
      </button>
    {/each}
  </div>
  <span class="status-indicator">
    <span class="dot" style:background={stateColors[connectionState]}></span>
    {#if !isMobile}
      <span class="status-label">{stateLabels[connectionState]}</span>
    {/if}
  </span>
  {#if !isMobile}
    <span class="hint">Ctrl+Shift+&lt; / &gt;</span>
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
