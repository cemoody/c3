<script lang="ts">
  import Upload from './Upload.svelte';

  let { onAction, uploadUrl = '/api/upload' }: { onAction: (bytes: string) => void; uploadUrl?: string } = $props();

  let arrowPadOpen = $state(false);

  const actions = [
    {
      label: 'Return',
      bytes: '\r',
      // Return/enter arrow icon
      icon: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M9 17H4v-5"/><path d="m4 17 7-7 4 4 5-5"/></svg>`,
      iconAlt: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><polyline points="9 10 4 15 9 20"/><path d="M20 4v7a4 4 0 0 1-4 4H4"/></svg>`,
    },
    {
      label: 'Ctrl-C',
      bytes: '\x03',
      // X / cancel icon
      icon: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><path d="m15 9-6 6"/><path d="m9 9 6 6"/></svg>`,
    },
    {
      label: 'Ctrl-D',
      bytes: '\x04',
      // EOF / eject icon
      icon: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M9 3H5a2 2 0 0 0-2 2v4"/><path d="M9 21H5a2 2 0 0 1-2-2v-4"/><path d="M15 3h4a2 2 0 0 1 2 2v4"/><path d="M15 21h4a2 2 0 0 0 2-2v-4"/><line x1="4" y1="12" x2="20" y2="12"/></svg>`,
    },
  ];

  function sendArrow(dir: string) {
    const arrows: Record<string, string> = {
      up: '\x1b[A',
      down: '\x1b[B',
      right: '\x1b[C',
      left: '\x1b[D',
    };
    onAction(arrows[dir]);
  }
</script>

<div class="quick-actions">
  {#each actions as action}
    <button class="action-icon" onclick={() => onAction(action.bytes)} title={action.label}>
      {@html action.iconAlt || action.icon}
    </button>
  {/each}
  <Upload {uploadUrl} />
  <button class="action-icon" onclick={() => arrowPadOpen = true} title="Arrow keys">
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M12 3v18"/><path d="M3 12h18"/><polyline points="8 8 12 4 16 8"/><polyline points="8 16 12 20 16 16"/><polyline points="4 8 4 8"/><polyline points="16 8 20 12 16 16"/><polyline points="8 8 4 12 8 16"/></svg>
  </button>
</div>

{#if arrowPadOpen}
  <!-- svelte-ignore a11y_click_events_have_key_events -->
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div class="arrow-backdrop" onclick={() => arrowPadOpen = false}>
    <div class="arrow-pad" onclick={(e) => e.stopPropagation()}>
      <div class="arrow-row">
        <div class="arrow-spacer"></div>
        <button class="arrow-btn" onclick={() => sendArrow('up')} title="Up">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><polyline points="18 15 12 9 6 15"/></svg>
        </button>
        <div class="arrow-spacer"></div>
      </div>
      <div class="arrow-row">
        <button class="arrow-btn" onclick={() => sendArrow('left')} title="Left">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><polyline points="15 18 9 12 15 6"/></svg>
        </button>
        <button class="arrow-btn arrow-center" onclick={() => arrowPadOpen = false} title="Close">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="3"/></svg>
        </button>
        <button class="arrow-btn" onclick={() => sendArrow('right')} title="Right">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><polyline points="9 18 15 12 9 6"/></svg>
        </button>
      </div>
      <div class="arrow-row">
        <div class="arrow-spacer"></div>
        <button class="arrow-btn" onclick={() => sendArrow('down')} title="Down">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><polyline points="6 9 12 15 18 9"/></svg>
        </button>
        <div class="arrow-spacer"></div>
      </div>
    </div>
  </div>
{/if}

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

  .action-icon {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 44px;
    height: 44px;
    background: var(--bg, #1e1e1e);
    color: var(--fg, #d4d4d4);
    border: 1px solid var(--border, #444);
    border-radius: 10px;
    cursor: pointer;
    flex-shrink: 0;
    padding: 0;
  }

  .action-icon:active {
    background: var(--accent, #0e639c);
    color: white;
  }

  .action-icon :global(svg) {
    width: 20px;
    height: 20px;
  }

  /* Arrow pad modal */
  .arrow-backdrop {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.4);
    z-index: 1000;
    display: flex;
    align-items: flex-end;
    justify-content: center;
    padding-bottom: 80px;
  }

  .arrow-pad {
    display: flex;
    flex-direction: column;
    gap: 4px;
    padding: 12px;
    background: var(--bg-secondary, #2d2d2d);
    border: 1px solid var(--border, #444);
    border-radius: 16px;
    animation: arrow-pop 0.12s ease-out;
  }

  @keyframes arrow-pop {
    from { transform: scale(0.9); opacity: 0; }
    to { transform: scale(1); opacity: 1; }
  }

  .arrow-row {
    display: flex;
    gap: 4px;
    justify-content: center;
  }

  .arrow-btn {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 56px;
    height: 56px;
    background: var(--bg, #1e1e1e);
    color: var(--fg, #d4d4d4);
    border: 1px solid var(--border, #444);
    border-radius: 12px;
    cursor: pointer;
    padding: 0;
  }

  .arrow-btn:active {
    background: var(--accent, #0e639c);
    color: white;
  }

  .arrow-btn :global(svg) {
    width: 24px;
    height: 24px;
  }

  .arrow-center {
    opacity: 0.4;
  }

  .arrow-spacer {
    width: 56px;
    height: 56px;
  }
</style>
