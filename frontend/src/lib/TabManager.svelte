<script lang="ts">
  type TabItem = { target: string; label: string; windowName: string; command: string };

  let {
    targets,
    activeTarget = '',
    onClose,
    onNavigate,
    onReorder,
    onRename,
    onNewSession,
  }: {
    targets: TabItem[];
    activeTarget?: string;
    onClose: () => void;
    onNavigate: (target: string) => void;
    onReorder: (ordered: string[]) => void;
    onRename: (target: string, name: string) => void;
    onNewSession: (name: string) => Promise<void>;
  } = $props();

  let editingTarget = $state<string | null>(null);
  let editValue = $state('');
  let renameCommitted = false;

  let creatingSession = $state(false);
  let newSessionName = $state('');
  let newSessionLoading = $state(false);

  // Drag state
  let dragIdx = $state<number | null>(null);
  let dragOverIdx = $state<number | null>(null);

  function startRename(t: TabItem, e: MouseEvent) {
    e.preventDefault();
    e.stopPropagation();
    editingTarget = t.target;
    editValue = t.windowName;
    renameCommitted = false;
    setTimeout(() => {
      const input = document.querySelector('.tm-rename-input') as HTMLInputElement;
      if (input) { input.focus(); input.select(); }
    }, 50);
  }

  function commitRename() {
    if (renameCommitted || !editingTarget) return;
    renameCommitted = true;
    const name = editValue.trim();
    const tgt = editingTarget;
    editingTarget = null;
    if (name) onRename(tgt, name);
  }

  function handleRenameKeydown(e: KeyboardEvent) {
    if (e.key === 'Enter') { e.preventDefault(); commitRename(); }
    else if (e.key === 'Escape') { editingTarget = null; }
  }

  async function handleNewSession() {
    const name = newSessionName.trim();
    if (!name || newSessionLoading) return;
    newSessionLoading = true;
    try {
      await onNewSession(name);
      newSessionName = '';
      creatingSession = false;
    } finally {
      newSessionLoading = false;
    }
  }

  function handleNewSessionKeydown(e: KeyboardEvent) {
    if (e.key === 'Enter') { e.preventDefault(); handleNewSession(); }
    else if (e.key === 'Escape') { creatingSession = false; newSessionName = ''; }
  }

  // Drag-and-drop handlers
  function onDragStart(e: DragEvent, idx: number) {
    dragIdx = idx;
    if (e.dataTransfer) {
      e.dataTransfer.effectAllowed = 'move';
      e.dataTransfer.setData('text/plain', String(idx));
    }
  }

  function onDragOver(e: DragEvent, idx: number) {
    e.preventDefault();
    if (e.dataTransfer) e.dataTransfer.dropEffect = 'move';
    dragOverIdx = idx;
  }

  function onDrop(e: DragEvent, idx: number) {
    e.preventDefault();
    if (dragIdx !== null && dragIdx !== idx) {
      const ordered = targets.map(t => t.target);
      const [moved] = ordered.splice(dragIdx, 1);
      ordered.splice(idx, 0, moved);
      onReorder(ordered);
    }
    dragIdx = null;
    dragOverIdx = null;
  }

  function onDragEnd() {
    dragIdx = null;
    dragOverIdx = null;
  }

  // Touch drag-and-drop
  let touchStartIdx = $state<number | null>(null);
  let touchCurrentIdx = $state<number | null>(null);

  function onTouchStart(idx: number) {
    touchStartIdx = idx;
    touchCurrentIdx = idx;
  }

  function onTouchMove(e: TouchEvent) {
    if (touchStartIdx === null) return;
    const touch = e.touches[0];
    const el = document.elementFromPoint(touch.clientX, touch.clientY);
    if (el) {
      const row = el.closest('[data-tab-idx]') as HTMLElement;
      if (row) {
        touchCurrentIdx = parseInt(row.dataset.tabIdx!, 10);
      }
    }
  }

  function onTouchEnd() {
    if (touchStartIdx !== null && touchCurrentIdx !== null && touchStartIdx !== touchCurrentIdx) {
      const ordered = targets.map(t => t.target);
      const [moved] = ordered.splice(touchStartIdx, 1);
      ordered.splice(touchCurrentIdx, 0, moved);
      onReorder(ordered);
    }
    touchStartIdx = null;
    touchCurrentIdx = null;
  }

  function handleBackdropClick(e: MouseEvent) {
    if ((e.target as HTMLElement).classList.contains('tm-backdrop')) {
      onClose();
    }
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') onClose();
  }
</script>

<svelte:window onkeydown={handleKeydown} />

<!-- svelte-ignore a11y_click_events_have_key_events -->
<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="tm-backdrop" onclick={handleBackdropClick}>
  <div class="tm-panel">
    <div class="tm-header">
      <span class="tm-title">Tabs</span>
      <button class="tm-close" onclick={onClose}>&times;</button>
    </div>

    <div class="tm-list" ontouchmove={onTouchMove} ontouchend={onTouchEnd}>
      {#each targets as t, i}
        <div
          class="tm-row"
          class:active={t.target === activeTarget}
          class:drag-over={dragOverIdx === i && dragIdx !== i}
          class:touch-over={touchCurrentIdx === i && touchStartIdx !== null && touchStartIdx !== i}
          data-tab-idx={i}
          draggable="true"
          ondragstart={(e) => onDragStart(e, i)}
          ondragover={(e) => onDragOver(e, i)}
          ondrop={(e) => onDrop(e, i)}
          ondragend={onDragEnd}
          ontouchstart={() => onTouchStart(i)}
        >
          <span class="tm-drag-handle" title="Drag to reorder">&#9776;</span>

          {#if editingTarget === t.target}
            <input
              class="tm-rename-input"
              type="text"
              bind:value={editValue}
              onkeydown={handleRenameKeydown}
              onblur={commitRename}
              onclick={(e) => { e.preventDefault(); e.stopPropagation(); }}
            />
          {:else}
            <button
              class="tm-name"
              onclick={() => { onNavigate(t.target); onClose(); }}
              title="{t.target} — {t.command}"
            >
              {t.windowName}
            </button>
          {/if}

          <button class="tm-edit-btn" onclick={(e) => startRename(t, e)} title="Rename">
            &#9998;
          </button>
        </div>
      {/each}
    </div>

    <div class="tm-footer">
      {#if creatingSession}
        <div class="tm-new-row">
          <input
            class="tm-new-input"
            type="text"
            placeholder="Session name..."
            bind:value={newSessionName}
            onkeydown={handleNewSessionKeydown}
          />
          <button class="tm-new-confirm" onclick={handleNewSession} disabled={newSessionLoading || !newSessionName.trim()}>
            {newSessionLoading ? '...' : 'Create'}
          </button>
          <button class="tm-new-cancel" onclick={() => { creatingSession = false; newSessionName = ''; }}>
            &times;
          </button>
        </div>
      {:else}
        <button class="tm-new-btn" onclick={() => { creatingSession = true; setTimeout(() => { const input = document.querySelector('.tm-new-input') as HTMLInputElement; if (input) input.focus(); }, 50); }}>
          + New Session
        </button>
      {/if}
    </div>
  </div>
</div>

<style>
  .tm-backdrop {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.4);
    z-index: 1000;
    display: flex;
    justify-content: flex-end;
  }

  .tm-panel {
    width: 320px;
    max-width: 100%;
    height: 100%;
    background: var(--bg-secondary);
    border-left: 1px solid var(--border);
    display: flex;
    flex-direction: column;
    animation: tm-slide-in 0.15s ease-out;
  }

  @keyframes tm-slide-in {
    from { transform: translateX(100%); }
    to { transform: translateX(0); }
  }

  @media (max-width: 480px) {
    .tm-panel {
      width: 100%;
    }
  }

  .tm-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 12px 16px;
    border-bottom: 1px solid var(--border);
  }

  .tm-title {
    font-size: 14px;
    font-weight: 600;
    color: var(--fg);
  }

  .tm-close {
    background: none;
    border: none;
    color: var(--fg-dim);
    font-size: 20px;
    cursor: pointer;
    padding: 4px 8px;
    line-height: 1;
    border-radius: 4px;
  }
  .tm-close:hover {
    background: var(--bg);
    color: var(--fg);
  }

  .tm-list {
    flex: 1;
    overflow-y: auto;
    padding: 8px 0;
  }

  .tm-row {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 6px 16px;
    min-height: 44px;
    transition: background 0.1s;
  }
  .tm-row:hover {
    background: var(--bg);
  }
  .tm-row.active {
    background: var(--bg);
    border-left: 3px solid var(--accent);
    padding-left: 13px;
  }
  .tm-row.drag-over,
  .tm-row.touch-over {
    border-top: 2px solid var(--accent);
  }

  .tm-drag-handle {
    cursor: grab;
    color: var(--fg-dim);
    font-size: 14px;
    flex-shrink: 0;
    user-select: none;
    min-width: 20px;
    text-align: center;
  }
  .tm-drag-handle:active {
    cursor: grabbing;
  }

  .tm-name {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    background: none;
    border: none;
    color: var(--fg);
    font-size: 13px;
    font-family: inherit;
    text-align: left;
    cursor: pointer;
    padding: 4px 0;
  }
  .tm-name:hover {
    color: var(--accent);
  }

  .tm-edit-btn {
    background: none;
    border: none;
    color: var(--fg-dim);
    font-size: 14px;
    cursor: pointer;
    padding: 4px 6px;
    border-radius: 4px;
    flex-shrink: 0;
    opacity: 0;
    transition: opacity 0.1s;
  }
  .tm-row:hover .tm-edit-btn {
    opacity: 1;
  }
  .tm-edit-btn:hover {
    background: var(--bg-secondary);
    color: var(--fg);
  }

  .tm-rename-input {
    flex: 1;
    min-width: 0;
    padding: 4px 8px;
    font-size: 13px;
    font-family: inherit;
    background: var(--bg);
    color: var(--fg);
    border: 1px solid var(--accent);
    border-radius: 4px;
    outline: none;
  }

  .tm-footer {
    border-top: 1px solid var(--border);
    padding: 12px 16px;
  }

  .tm-new-btn {
    width: 100%;
    padding: 10px;
    background: var(--bg);
    color: var(--fg);
    border: 1px dashed var(--border);
    border-radius: 6px;
    font-size: 13px;
    font-family: inherit;
    cursor: pointer;
    min-height: 44px;
  }
  .tm-new-btn:hover {
    border-color: var(--accent);
    color: var(--accent);
  }

  .tm-new-row {
    display: flex;
    gap: 8px;
    align-items: center;
  }

  .tm-new-input {
    flex: 1;
    min-width: 0;
    padding: 8px 10px;
    font-size: 13px;
    font-family: inherit;
    background: var(--bg);
    color: var(--fg);
    border: 1px solid var(--border);
    border-radius: 4px;
    outline: none;
    min-height: 44px;
  }
  .tm-new-input:focus {
    border-color: var(--accent);
  }

  .tm-new-confirm {
    padding: 8px 14px;
    background: var(--accent);
    color: white;
    border: none;
    border-radius: 4px;
    font-size: 13px;
    font-family: inherit;
    cursor: pointer;
    min-height: 44px;
    flex-shrink: 0;
  }
  .tm-new-confirm:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .tm-new-cancel {
    background: none;
    border: none;
    color: var(--fg-dim);
    font-size: 18px;
    cursor: pointer;
    padding: 4px 8px;
    flex-shrink: 0;
  }
</style>
