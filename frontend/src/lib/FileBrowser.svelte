<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { marked } from 'marked';
  import { isImage, isMarkdown, isTextFile, isPreviewable, isPlot } from './fileTypes';

  type FileEntry = { name: string; isDir: boolean; size: number };

  let currentPath = $state(getInitialPath());
  let files = $state<FileEntry[]>([]);
  let loading = $state(false);
  let previewUrl = $state<string | null>(null);
  let previewName = $state('');
  let previewFullPath = $state('');
  let previewOverlayEl: HTMLDivElement;

  // Markdown editor state
  let mdContent = $state('');
  let mdRendered = $state('');
  let mdEditing = $state(false);
  let mdDirty = $state(false);
  let mdSaving = $state(false);
  let mdPreviewPath = $state('');
  let textPreviewContent = $state<string | null>(null);

  // Auto-focus the preview overlay so it receives keyboard events
  $effect(() => {
    if ((previewUrl || mdPreviewPath) && previewOverlayEl) {
      previewOverlayEl.focus();
    }
  });

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


  async function openFilePath(filePath: string) {
    const name = filePath.split('/').pop() || filePath;
    if (isImage(name)) {
      previewUrl = `/api/files/raw?path=${encodeURIComponent(filePath)}`;
      mdPreviewPath = '';
      textPreviewContent = null;
      previewName = name;
      previewFullPath = filePath;
    } else if (isMarkdown(name)) {
      previewUrl = null;
      textPreviewContent = null;
      await openMarkdown(filePath);
    } else if (name.endsWith('.html') || name.endsWith('.pdf')) {
      window.open(`/api/files/raw?path=${encodeURIComponent(filePath)}`, '_blank');
    } else if (isTextFile(name)) {
      previewUrl = null;
      mdPreviewPath = '';
      textPreviewContent = null;
      previewName = name;
      previewFullPath = filePath;
      try {
        const res = await fetch(`/api/files/raw?path=${encodeURIComponent(filePath)}`);
        if (res.ok) {
          textPreviewContent = await res.text();
        }
      } catch {}
    } else {
      const dir = filePath.replace(/\/[^/]+$/, '');
      loadDir(dir);
    }
  }

  async function openMarkdown(filePath: string) {
    try {
      const res = await fetch(`/api/files/raw?path=${encodeURIComponent(filePath)}`);
      if (!res.ok) return;
      const text = await res.text();
      mdContent = text;
      mdRendered = marked(text) as string;
      mdPreviewPath = filePath;
      mdEditing = false;
      mdDirty = false;
      previewName = filePath.split('/').pop() || filePath;
      previewFullPath = filePath;
    } catch (e) {
      console.error('Failed to load markdown:', e);
    }
  }

  function onMdEdit() {
    mdRendered = marked(mdContent) as string;
    mdDirty = true;
  }

  async function saveMd() {
    if (!mdPreviewPath || mdSaving) return;
    mdSaving = true;
    try {
      const res = await fetch(`/api/files/raw?path=${encodeURIComponent(mdPreviewPath)}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'text/plain' },
        body: mdContent,
      });
      if (res.ok) mdDirty = false;
    } catch (e) {
      console.error('Save failed:', e);
    } finally {
      mdSaving = false;
    }
  }

  function openFile(name: string) {
    openFilePath(currentPath + '/' + name);
  }

  function closePreview() {
    previewUrl = null;
    mdPreviewPath = '';
    textPreviewContent = null;
    previewName = '';
    previewFullPath = '';
    mdEditing = false;
    mdDirty = false;
    // Return focus to search input if search is active
    if (searchActive) {
      setTimeout(() => searchInputEl?.focus(), 0);
    }
  }

  // Navigate preview to next/prev previewable search result
  function previewNavigate(delta: number) {
    if (searchResults.length === 0) return;

    let idx = selectedIndex;
    for (let i = 0; i < searchResults.length; i++) {
      idx = (idx + delta + searchResults.length) % searchResults.length;
      const name = searchResults[idx].split('/').pop() || '';
      if (isPreviewable(name)) {
        selectedIndex = idx;
        const fullPath = searchResults[idx];
        openFilePath(fullPath);
        scrollResultIntoView(idx);
        return;
      }
    }
  }

  function scrollResultIntoView(idx: number) {
    const el = document.querySelector(`.search-result:nth-child(${idx + 1})`);
    el?.scrollIntoView({ block: 'nearest' });
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

  function previewSelectedResult() {
    if (searchResults.length === 0) return;
    const fullPath = searchResults[selectedIndex];
    openFilePath(fullPath);
  }

  function onSearchKeydown(e: KeyboardEvent) {
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      selectedIndex = Math.min(selectedIndex + 1, searchResults.length - 1);
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      selectedIndex = Math.max(selectedIndex - 1, 0);
    } else if ((e.key === 'Enter' || e.key === ' ') && searchResults.length > 0) {
      // Enter or Space: preview the selected result (like Finder QuickLook)
      e.preventDefault();
      previewSelectedResult();
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
          onclick={() => { selectedIndex = i; previewSelectedResult(); }}
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

  {#if previewUrl || mdPreviewPath || textPreviewContent !== null}
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <div bind:this={previewOverlayEl} class="preview-overlay" onclick={closePreview} onkeydown={(e) => {
      if (e.key === 'Escape') closePreview();
      else if (!mdEditing && (e.key === 'ArrowDown' || e.key === 'ArrowRight')) { e.preventDefault(); previewNavigate(1); }
      else if (!mdEditing && (e.key === 'ArrowUp' || e.key === 'ArrowLeft')) { e.preventDefault(); previewNavigate(-1); }
      else if (!mdEditing && (e.metaKey || e.ctrlKey) && e.key === 's') { e.preventDefault(); saveMd(); }
    }} tabindex="-1">
      <!-- svelte-ignore a11y_no_static_element_interactions -->
      <div class="preview-content" class:wide={!!mdPreviewPath} onclick={(e) => e.stopPropagation()} onkeydown={(e) => {
        if (mdEditing && (e.metaKey || e.ctrlKey) && e.key === 's') { e.preventDefault(); saveMd(); }
      }}>
        <div class="preview-header">
          <span class="preview-title">{previewName}{mdDirty ? ' *' : ''}</span>
          {#if mdPreviewPath}
            <button class="header-btn" class:active={mdEditing} onclick={() => mdEditing = !mdEditing}>
              {mdEditing ? 'Preview' : 'Edit'}
            </button>
            {#if mdDirty}
              <button class="header-btn save-btn" onclick={saveMd} disabled={mdSaving}>
                {mdSaving ? 'Saving...' : 'Save'}
              </button>
            {/if}
          {/if}
          {#if searchResults.length > 1}
            <span class="preview-nav-hint">{selectedIndex + 1} / {searchResults.length} &mdash; arrow keys to navigate</span>
          {/if}
          <button class="close-btn" onclick={closePreview}>&times;</button>
        </div>
        {#if previewUrl}
          <img src={previewUrl} alt={previewName} />
        {:else if mdPreviewPath}
          {#if mdEditing}
            <textarea
              class="md-editor"
              bind:value={mdContent}
              oninput={onMdEdit}
              spellcheck="false"
            ></textarea>
          {:else}
            <div class="md-preview">{@html mdRendered}</div>
          {/if}
        {:else if textPreviewContent !== null}
          <pre class="text-preview">{textPreviewContent}</pre>
        {/if}
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
  .preview-title {
    font-weight: 600;
  }
  .preview-nav-hint {
    font-size: 11px;
    color: var(--fg-dim);
    margin-left: auto;
    margin-right: 8px;
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
  .preview-content.wide {
    width: 90vw;
    max-width: 900px;
    height: 90vh;
  }
  .header-btn {
    padding: 2px 10px;
    font-size: 11px;
    font-family: inherit;
    background: var(--bg);
    color: var(--fg-dim);
    border: 1px solid var(--border);
    border-radius: 3px;
  }
  .header-btn:hover, .header-btn.active {
    color: var(--fg);
    border-color: var(--accent);
  }
  .save-btn {
    color: var(--success);
    border-color: var(--success);
  }
  .md-preview {
    flex: 1;
    overflow-y: auto;
    padding: 16px 24px;
    font-size: 14px;
    line-height: 1.6;
    color: var(--fg);
  }
  .md-preview :global(h1) { font-size: 1.6em; margin: 0.8em 0 0.4em; }
  .md-preview :global(h2) { font-size: 1.3em; margin: 0.8em 0 0.4em; }
  .md-preview :global(h3) { font-size: 1.1em; margin: 0.6em 0 0.3em; }
  .md-preview :global(p) { margin: 0.5em 0; }
  .md-preview :global(code) {
    background: var(--bg-secondary);
    padding: 1px 4px;
    border-radius: 3px;
    font-size: 0.9em;
  }
  .md-preview :global(pre) {
    background: var(--bg-secondary);
    padding: 12px;
    border-radius: 6px;
    overflow-x: auto;
  }
  .md-preview :global(pre code) {
    background: none;
    padding: 0;
  }
  .md-preview :global(ul), .md-preview :global(ol) {
    padding-left: 1.5em;
  }
  .md-preview :global(blockquote) {
    border-left: 3px solid var(--border);
    margin: 0.5em 0;
    padding: 0.5em 1em;
    color: var(--fg-dim);
  }
  .md-preview :global(a) {
    color: var(--accent);
  }
  .md-preview :global(table) {
    border-collapse: collapse;
    width: 100%;
  }
  .md-preview :global(th), .md-preview :global(td) {
    border: 1px solid var(--border);
    padding: 6px 10px;
    text-align: left;
  }
  .text-preview {
    flex: 1;
    overflow: auto;
    padding: 16px 24px;
    margin: 0;
    font-family: 'Menlo', 'Monaco', 'Courier New', monospace;
    font-size: 13px;
    line-height: 1.5;
    color: var(--fg);
    white-space: pre-wrap;
    word-break: break-all;
    tab-size: 4;
  }
  .md-editor {
    flex: 1;
    width: 100%;
    padding: 16px 24px;
    font-size: 14px;
    font-family: 'Menlo', 'Monaco', 'Courier New', monospace;
    line-height: 1.5;
    background: var(--bg);
    color: var(--fg);
    border: none;
    outline: none;
    resize: none;
  }
</style>
