<script lang="ts">
  import Upload from './Upload.svelte';

  let { onAction, uploadUrl = '/api/upload' }: { onAction: (bytes: string) => void; uploadUrl?: string } = $props();

  const actions = [
    { label: 'y', bytes: 'y\n' },
    { label: 'n', bytes: 'n\n' },
    { label: 'Ctrl-C', bytes: '\x03' },
    { label: 'Ctrl-D', bytes: '\x04' },
  ];
</script>

<div class="quick-actions">
  {#each actions as action}
    <button class="action-btn" onclick={() => onAction(action.bytes)}>
      {action.label}
    </button>
  {/each}
  <Upload {uploadUrl} />
</div>

<style>
  .quick-actions {
    display: flex;
    gap: 6px;
    padding: 6px 8px;
    background: var(--bg-secondary, #2d2d2d);
    border-top: 1px solid var(--border, #444);
    overflow-x: auto;
    -webkit-overflow-scrolling: touch;
  }

  .action-btn {
    flex-shrink: 0;
    min-width: 44px;
    min-height: 44px;
    padding: 8px 14px;
    background: var(--bg, #1e1e1e);
    color: var(--fg, #d4d4d4);
    border: 1px solid var(--border, #444);
    border-radius: 20px;
    font-size: 13px;
    font-family: inherit;
    white-space: nowrap;
  }

  .action-btn:active {
    background: var(--accent, #0e639c);
    color: white;
  }
</style>
