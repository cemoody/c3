<script lang="ts">
  import { onMount, onDestroy } from 'svelte';

  type FileEntry = { name: string; isDir: boolean; size: number };

  let currentPath = $state(getInitialPath());
  let files = $state<FileEntry[]>([]);
  let loading = $state(false);
  let previewUrl = $state<string | null>(null);
  let previewName = $state('');

  // Search state
  let searchQuery = $state('');
  let searchResults = $state<string[]>([]);
  let searchActive = $state(false);
  let selectedIndex = $state(0);
  let indexedCount = $state(0);
  let searchInputEl: HTMLInputElement;
  let debounceTimer: ReturnType<typeof setTimeout> | null = null;

  function getInitialPath(): string {
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

  function openFilePath(filePath: string) {
    const name = filePath.split('/').pop() || filePath;
    if (isImage(name)) {
      previewUrl = `/api/files/raw?path=${encodeURIComponent(filePath)}`;
      previewName = name;
    } else if (name.endsWith('.html')) {
      window.open(`/api/files/raw?path=${encodeURIComponent(filePath)}`, '_blank');
    } else if (name.endsWith('.pdf')) {
      window.open(`/api/files/raw?path=${encodeURIComponent(filePath)}`, '_blank');
    } else {
      // Navigate to the file's directory
      const dir = filePath.replace(/\/[^/]+$/, '');
      loadDir(dir);
    }
  }

  function openFile(name: string) {
    openFilePath(currentPath + '/' + name);
  }

  function closePreview() {
    previewUrl = null;
    previewName = '';
  }

  function fileIcon(name: string, isDir: boolean): string {
    if (isDir) return '\uD83D\uDCC1';
    if (isImage(name)) return '\uD83D\uDDBC';
    if (name.endsWith('.py')) return '\uD83D\uDC0D';
    if (name.endsWith('.html')) return '\uD83C\uDF10';
    if (name.endsWith('.csv') || name.endsWith('.parquet')) return '\uD83D\uDCCA';
    return '\uD83D\uDCC4';
  }

  // Search
  async function doSearch(q: string) {
    if (!q.trim()) {
      searchResults = [];
      return;
    }
    try {
      const res = await fetch(`/api/search?q=${encodeURIComponent(q)}`);
      if (!res.ok) return;
      const data = await res.json();
      searchResults = data.results || [];
      indexedCount = data.indexed || 0;
      selectedIndex = 0;
    } catch {}
  }

  function onSearchInput() {
    if (debounceTimer) clearTimeout(debounceTimer);
    debounceTimer = setTimeout(() => doSearch(searchQuery), 100);
  }

  function onSearchKeydown(e: KeyboardEvent) {
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      selectedIndex = Math.min(selectedIndex + 1, searchResults.length - 1);
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      selectedIndex = Math.max(selectedIndex - 1, 0);
    } else if (e.key === 'Enter' && searchResults.length > 0) {
      e.preventDefault();
      const homeDir = currentPath.split('/').slice(0, 3).join('/') || '/home';
      openFilePath(homeDir + '/' + searchResults[selectedIndex]);
      searchQuery = '';
      searchResults = [];
      searchActive = false;
    } else if (e.key === 'Escape') {
      searchQuery = '';
      searchResults = [];
      searchActive = false;
      searchInputEl?.blur();
    }
  }

  function activateSearch() {
    searchActive = true;
    setTimeout(() => searchInputEl?.focus(), 0);
  }

  function handleGlobalKeydown(e: KeyboardEvent) {
    // Cmd+P or Ctrl+P to open search
    if ((e.metaKey || e.ctrlKey) && e.key === 'p') {
      e.preventDefault();
      activateSearch();
    }
  }

  onMount(() => {
    loadDir(currentPath);
    document.addEventListener('keydown', handleGlobalKeydown);
  });

  onDestroy(() => {
    document.removeEventListener('keydown', handleGlobalKeydown);
  });
</script>

<div class="file-browser">
  <div class="toolbar">
    <button class="nav-btn" onclick={goUp}>&larr; Up</button>
    <div class="search-box" class:active={searchActive}>
      <input
        bind:this={searchInputEl}
        bind:value={searchQuery}
        class="search-input"
        type="text"
        placeholder="Search files... (Cmd+P)"
        oninput={onSearchInput}
        onkeydown={onSearchKeydown}
        onfocus={() => searchActive = true}
      />
      {#if indexedCount > 0 && searchActive}
        <span class="index-count">{indexedCount.toLocaleString()} files</span>
      {/if}
    </div>
    <span class="path">{currentPath}</span>
  </div>

  {#if searchActive && searchQuery.trim()}
    <div class="search-results">
      {#each searchResults as result, i}
        <button
          class="search-result"
          class:selected={i === selectedIndex}
          onclick={() => { openFilePath(currentPath.split('/').slice(0, 3).join('/') + '/' + result); searchQuery = ''; searchResults = []; searchActive = false; }}
          onmouseenter={() => selectedIndex = i}
        >
          <span class="icon">{fileIcon(result.split('/').pop() || '', false)}</span>
          <span class="result-path">
            <span class="result-filename">{result.split('/').pop()}</span>
            <span class="result-dir">{result.split('/').slice(0, -1).join('/')}</span>
          </span>
        </button>
      {/each}
      {#if searchResults.length === 0}
        <div class="empty">No matches</div>
      {/if}
    </div>
  {:else if loading}
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
          <span class="icon">{fileIcon(entry.name, entry.isDir)}</span>
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
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <div class="preview-overlay" onclick={closePreview} onkeydown={(e) => e.key === 'Escape' && closePreview()}>
      <!-- svelte-ignore a11y_no_static_element_interactions -->
      <div class="preview-content" onclick={(e) => e.stopPropagation()} onkeydown={() => {}}>
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
  .search-box {
    flex: 1;
    display: flex;
    align-items: center;
    position: relative;
    min-width: 0;
  }
  .search-input {
    width: 100%;
    padding: 5px 10px;
    font-size: 13px;
    font-family: inherit;
    background: var(--bg);
    color: var(--fg);
    border: 1px solid var(--border);
    border-radius: 4px;
    outline: none;
  }
  .search-input:focus {
    border-color: var(--accent);
  }
  .search-input::placeholder {
    color: var(--fg-dim);
  }
  .index-count {
    position: absolute;
    right: 8px;
    font-size: 10px;
    color: var(--fg-dim);
    pointer-events: none;
  }
  .path {
    font-size: 12px;
    color: var(--fg-dim);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    flex-shrink: 0;
    max-width: 30%;
  }
  .search-results {
    flex: 1;
    overflow-y: auto;
    padding: 4px 0;
  }
  .search-result {
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
  .search-result:hover,
  .search-result.selected {
    background: var(--bg-secondary);
  }
  .search-result.selected {
    border-left: 2px solid var(--accent);
  }
  .result-path {
    display: flex;
    flex-direction: column;
    min-width: 0;
    flex: 1;
  }
  .result-filename {
    font-weight: 600;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .result-dir {
    font-size: 11px;
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
