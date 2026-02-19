<script lang="ts">
  import type { ConnectionState, PaneState } from './websocket';

  let {
    connectionState = 'disconnected',
    paneState = 'unknown',
    target = '',
    onBack,
  }: {
    connectionState?: ConnectionState;
    paneState?: PaneState;
    target?: string;
    onBack?: () => void;
  } = $props();

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
</script>

<div class="status-bar">
  {#if onBack}
    <button class="back-btn" onclick={onBack}>&larr;</button>
  {/if}
  <span class="dot" style:background={stateColors[connectionState]}></span>
  <span class="label">{stateLabels[connectionState]}</span>
  {#if target}
    <span class="target">{target}</span>
  {/if}
  {#if paneState === 'missing'}
    <span class="pane-missing">Pane not found</span>
  {/if}
</div>

<style>
  .status-bar {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 4px 8px;
    background: var(--bg-secondary);
    border-bottom: 1px solid var(--border);
    font-size: 12px;
    color: var(--fg-dim);
    flex-shrink: 0;
  }
  .dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    flex-shrink: 0;
  }
  .back-btn {
    background: none;
    border: none;
    color: var(--fg-dim);
    font-size: 14px;
    padding: 2px 6px;
    cursor: pointer;
  }
  .back-btn:hover {
    color: var(--fg);
  }
  .target {
    color: var(--fg-dim);
    margin-left: auto;
    font-family: monospace;
  }
  .pane-missing {
    color: var(--error);
    margin-left: 8px;
  }
</style>
