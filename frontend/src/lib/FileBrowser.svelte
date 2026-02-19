<script lang="ts">
  import { onMount } from 'svelte';

  type FileEntry = { name: string; isDir: boolean; size: number };

  let currentPath = $state(getInitialPath());
  let files = $state<FileEntry[]>([]);
  let loading = $state(false);
  let previewUrl = $state<string | null>(null);
  let previewName = $state('');

  function getInitialPath(): string {
    // Try to get from URL hash, otherwise use home
    const hash = location.hash.replace('#', '');
    return hash || '';
  }

  async function loadDir(path: string) {
    loading = true;
    try {
      const params = path ? `?path=${encodeURIComponent(path)}` : '';
      const res = await fetch(`/api/files${params}`);
      if (!res.ok) throw new Error(await res.text());
      const data = await res.json();
      currentPath = data.path;
      files = data.files || [];
      location.hash = currentPath;
    } catch (e) {
      console.error('Failed to load directory:', e);
    } finally {
      loading = false;
    }
  }

  function navigate(name: string) {
    loadDir(currentPath + '/' + name);
  }

  function goUp() {
    const parent = currentPath.replace(/\/[^/]+$/, '') || '/';
    loadDir(parent);
  }

  function formatSize(bytes: number): string {
    if (bytes < 1024) return bytes + ' B';
    if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB';
    if (bytes < 1024 * 1024 * 1024) return (bytes / (1024 * 1024)).toFixed(1) + ' MB';
    return (bytes / (1024 * 1024 * 1024)).toFixed(1) + ' GB';
  }

  function isImage(name: string): boolean {
    return /\.(png|jpg|jpeg|gif|webp|svg|bmp)$/i.test(name);
  }

  function isPlot(name: string): boolean {
    return /\.(png|jpg|jpeg|gif|webp|svg|pdf|html)$/i.test(name);
  }

  function openFile(name: string) {
    const filePath = currentPath + '/' + name;
    if (isImage(name)) {
      previewUrl = `/api/files/raw?path=${encodeURIComponent(filePath)}`;
      previewName = name;
    } else if (name.endsWith('.html')) {
      // Open HTML files (like plotly plots) in a new tab
      window.open(`/api/files/raw?path=${encodeURIComponent(filePath)}`, '_blank');
    } else if (name.endsWith('.pdf')) {
      window.open(`/api/files/raw?path=${encodeURIComponent(filePath)}`, '_blank');
    } else {
      // Download other files
      const a = document.createElement('a');
      a.href = `/api/files/raw?path=${encodeURIComponent(filePath)}`;
      a.download = name;
      a.click();
    }
  }

  function closePreview() {
    previewUrl = null;
    previewName = '';
  }

  function fileIcon(entry: FileEntry): string {
    if (entry.isDir) return '\uD83D\uDCC1';
    if (isImage(entry.name)) return '\uD83D\uDDBC';
    if (entry.name.endsWith('.py')) return '\uD83D\uDC0D';
    if (entry.name.endsWith('.html')) return '\uD83C\uDF10';
    if (entry.name.endsWith('.csv') || entry.name.endsWith('.parquet')) return '\uD83D\uDCCA';
    return '\uD83D\uDCC4';
  }

  onMount(() => {
    loadDir(currentPath);
  });
</script>

<div class="file-browser">
  <div class="toolbar">
    <button class="nav-btn" onclick={goUp}>&larr; Up</button>
    <span class="path">{currentPath}</span>
  </div>

  {#if loading}
    <div class="loading">Loading...</div>
  {:else}
    <div class="file-list">
      {#each files as entry}
        <button
          class="file-entry"
          class:dir={entry.isDir}
          class:plot={!entry.isDir && isPlot(entry.name)}
          onclick={() => entry.isDir ? navigate(entry.name) : openFile(entry.name)}
        >
          <span class="icon">{fileIcon(entry)}</span>
          <span class="name">{entry.name}</span>
          {#if !entry.isDir}
            <span class="size">{formatSize(entry.size)}</span>
          {/if}
        </button>
      {/each}
      {#if files.length === 0}
        <div class="empty">Empty directory</div>
      {/if}
    </div>
  {/if}

  {#if previewUrl}
    <div class="preview-overlay" onclick={closePreview}>
      <div class="preview-content" onclick={(e) => e.stopPropagation()}>
        <div class="preview-header">
          <span>{previewName}</span>
          <button class="close-btn" onclick={closePreview}>&times;</button>
        </div>
        <img src={previewUrl} alt={previewName} />
      </div>
    </div>
  {/if}
</div>

<style>
  .file-browser {
    height: 100%;
    display: flex;
    flex-direction: column;
    background: var(--bg);
    color: var(--fg);
    overflow: hidden;
  }
  .toolbar {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 6px 10px;
    background: var(--bg-secondary);
    border-bottom: 1px solid var(--border);
    flex-shrink: 0;
  }
  .nav-btn {
    padding: 4px 10px;
    background: var(--bg);
    color: var(--fg);
    border: 1px solid var(--border);
    border-radius: 4px;
    font-size: 12px;
    font-family: inherit;
  }
  .nav-btn:hover {
    border-color: var(--accent);
  }
  .path {
    font-size: 12px;
    color: var(--fg-dim);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .file-list {
    flex: 1;
    overflow-y: auto;
    padding: 4px 0;
  }
  .file-entry {
    display: flex;
    align-items: center;
    gap: 8px;
    width: 100%;
    padding: 6px 12px;
    background: transparent;
    border: none;
    color: var(--fg);
    font-size: 13px;
    font-family: inherit;
    text-align: left;
  }
  .file-entry:hover {
    background: var(--bg-secondary);
  }
  .file-entry.dir .name {
    font-weight: 600;
  }
  .file-entry.plot {
    color: var(--accent);
  }
  .icon {
    flex-shrink: 0;
    width: 20px;
    text-align: center;
  }
  .name {
    flex: 1;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .size {
    color: var(--fg-dim);
    font-size: 11px;
    flex-shrink: 0;
  }
  .loading, .empty {
    padding: 20px;
    text-align: center;
    color: var(--fg-dim);
    font-size: 13px;
  }
  .preview-overlay {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.8);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 200;
    padding: 20px;
  }
  .preview-content {
    max-width: 90vw;
    max-height: 90vh;
    background: var(--bg);
    border-radius: 8px;
    overflow: hidden;
    display: flex;
    flex-direction: column;
  }
  .preview-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 8px 12px;
    background: var(--bg-secondary);
    font-size: 13px;
  }
  .close-btn {
    background: none;
    border: none;
    color: var(--fg);
    font-size: 20px;
    padding: 0 4px;
  }
  .preview-content img {
    max-width: 90vw;
    max-height: calc(90vh - 40px);
    object-fit: contain;
  }
</style>
