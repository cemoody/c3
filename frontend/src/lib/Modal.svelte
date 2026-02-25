<script lang="ts">
  import { onMount } from 'svelte';
  import type { Snippet } from 'svelte';

  let {
    title,
    onClose,
    position = 'right',
    panelWidth,
    headerExtra,
    children,
  }: {
    title: string;
    onClose: () => void;
    position?: 'right' | 'center';
    panelWidth?: string;
    headerExtra?: Snippet;
    children: Snippet;
  } = $props();

  function handleBackdrop(e: MouseEvent) {
    if ((e.target as HTMLElement).classList.contains('modal-backdrop')) {
      onClose();
    }
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') {
      e.stopPropagation();
      onClose();
    }
  }

  onMount(() => {
    document.addEventListener('keydown', handleKeydown, true);
    return () => document.removeEventListener('keydown', handleKeydown, true);
  });
</script>

<!-- svelte-ignore a11y_click_events_have_key_events -->
<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="modal-backdrop" class:center={position === 'center'} onclick={handleBackdrop}>
  <div
    class="modal-panel"
    class:center={position === 'center'}
    style:width={position === 'right' && panelWidth ? panelWidth : undefined}
  >
    <div class="modal-header">
      <span class="modal-title">{title}</span>
      {#if headerExtra}
        {@render headerExtra()}
      {/if}
      <button class="modal-close" onclick={onClose}>&times;</button>
    </div>
    {@render children()}
  </div>
</div>

<style>
  .modal-backdrop {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.4);
    z-index: 1000;
    display: flex;
    justify-content: flex-end;
  }

  .modal-backdrop.center {
    justify-content: center;
    align-items: center;
    background: rgba(0, 0, 0, 0.6);
    z-index: 1100;
    padding: 24px;
  }

  .modal-panel {
    width: 320px;
    max-width: 100%;
    height: 100%;
    background: var(--bg-secondary);
    border-left: 1px solid var(--border);
    display: flex;
    flex-direction: column;
    animation: modal-slide-in 0.15s ease-out;
  }

  @keyframes modal-slide-in {
    from { transform: translateX(100%); }
    to { transform: translateX(0); }
  }

  .modal-panel.center {
    width: 800px;
    max-width: 90vw;
    max-height: 90vh;
    height: auto;
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: 12px;
    overflow: hidden;
    animation: modal-pop 0.12s ease-out;
  }

  @keyframes modal-pop {
    from { transform: scale(0.95); opacity: 0; }
    to { transform: scale(1); opacity: 1; }
  }

  @media (max-width: 480px) {
    .modal-panel {
      width: 100%;
    }
    .modal-backdrop.center {
      padding: 8px;
    }
    .modal-panel.center {
      max-width: 100%;
      max-height: 100%;
      width: 100%;
      border-radius: 8px;
    }
  }

  .modal-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
    padding: 12px 16px;
    border-bottom: 1px solid var(--border);
    flex-shrink: 0;
  }

  .modal-title {
    font-size: 14px;
    font-weight: 600;
    color: var(--fg);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .modal-close {
    background: none;
    border: none;
    color: var(--fg-dim);
    font-size: 20px;
    cursor: pointer;
    padding: 4px 8px;
    line-height: 1;
    border-radius: 4px;
    flex-shrink: 0;
  }
  .modal-close:hover {
    background: var(--bg);
    color: var(--fg);
  }
  .modal-panel.center .modal-close:hover {
    background: var(--bg-secondary);
  }
</style>
