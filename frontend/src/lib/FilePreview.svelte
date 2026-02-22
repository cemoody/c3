<script lang="ts">
  import { onMount } from 'svelte';
  import { marked } from 'marked';

  let {
    path,
    onClose,
  }: {
    path: string;
    onClose: () => void;
  } = $props();

  let textContent = $state<string | null>(null);
  let htmlContent = $state<string | null>(null);
  let loading = $state(true);
  let error = $state<string | null>(null);

  const fileName = path.split('/').pop() || path;
  const ext = fileName.includes('.') ? fileName.split('.').pop()!.toLowerCase() : '';
  const rawUrl = `/api/files/raw?path=${encodeURIComponent(path)}`;

  const imageExts = new Set(['png', 'jpg', 'jpeg', 'gif', 'webp', 'svg', 'bmp']);
  const markdownExts = new Set(['md', 'markdown', 'mdx']);
  const textExts = new Set([
    'txt', 'log', 'csv', 'json', 'yaml', 'yml', 'toml', 'xml',
    'py', 'js', 'ts', 'jsx', 'tsx', 'sh', 'bash', 'zsh',
    'go', 'rs', 'c', 'cpp', 'h', 'hpp', 'java', 'rb', 'php',
    'css', 'scss', 'less', 'html', 'sql', 'r', 'lua',
    'env', 'ini', 'conf', 'cfg', 'properties',
    'svelte', 'vue', 'swift', 'kt', 'scala',
    'makefile', 'dockerfile',
  ]);
  const openInTabExts = new Set(['pdf']);

  type FileType = 'image' | 'markdown' | 'text' | 'open-in-tab' | 'download';

  function getFileType(): FileType {
    if (imageExts.has(ext)) return 'image';
    if (markdownExts.has(ext)) return 'markdown';
    if (textExts.has(ext)) return 'text';
    if (openInTabExts.has(ext)) return 'open-in-tab';
    return 'download';
  }

  const fileType = getFileType();

  // For open-in-tab types, just open and close the modal
  if (fileType === 'open-in-tab') {
    window.open(rawUrl, '_blank');
    onClose();
  }

  onMount(async () => {
    if (fileType === 'markdown') {
      try {
        const res = await fetch(rawUrl);
        if (!res.ok) { error = `Failed to load: ${res.status}`; return; }
        const text = await res.text();
        htmlContent = await marked(text);
      } catch (e: any) {
        error = e.message || 'Failed to load file';
      } finally {
        loading = false;
      }
    } else if (fileType === 'text') {
      try {
        const res = await fetch(rawUrl);
        if (!res.ok) { error = `Failed to load: ${res.status}`; return; }
        textContent = await res.text();
      } catch (e: any) {
        error = e.message || 'Failed to load file';
      } finally {
        loading = false;
      }
    } else {
      loading = false;
    }
  });

  function handleBackdrop(e: MouseEvent) {
    if ((e.target as HTMLElement).classList.contains('fp-backdrop')) {
      onClose();
    }
  }

  // Use capture phase so Escape works even when xterm.js has focus
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
<div class="fp-backdrop" onclick={handleBackdrop}>
  <div class="fp-panel">
    <div class="fp-header">
      <span class="fp-filename" title={path}>{fileName}</span>
      <span class="fp-path">{path}</span>
      <div class="fp-actions">
        <a class="fp-download" href={rawUrl} download={fileName} title="Download">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/></svg>
        </a>
        <button class="fp-close" onclick={onClose}>&times;</button>
      </div>
    </div>

    <div class="fp-body">
      {#if error}
        <div class="fp-error">{error}</div>
      {:else if loading}
        <div class="fp-loading">Loading...</div>
      {:else if fileType === 'image'}
        <img class="fp-image" src={rawUrl} alt={fileName} />
      {:else if fileType === 'markdown' && htmlContent}
        <div class="fp-markdown">{@html htmlContent}</div>
      {:else if fileType === 'text' && textContent !== null}
        <pre class="fp-text">{textContent}</pre>
      {:else if fileType === 'download'}
        <div class="fp-download-prompt">
          <p>This file type cannot be previewed.</p>
          <a class="fp-download-btn" href={rawUrl} download={fileName}>Download {fileName}</a>
        </div>
      {/if}
    </div>
  </div>
</div>

<style>
  .fp-backdrop {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.6);
    z-index: 1100;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 24px;
  }

  .fp-panel {
    background: var(--bg, #fdf6e3);
    border: 1px solid var(--border, #444);
    border-radius: 12px;
    max-width: 90vw;
    max-height: 90vh;
    width: 800px;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    animation: fp-pop 0.12s ease-out;
  }

  @keyframes fp-pop {
    from { transform: scale(0.95); opacity: 0; }
    to { transform: scale(1); opacity: 1; }
  }

  @media (max-width: 480px) {
    .fp-backdrop {
      padding: 8px;
    }
    .fp-panel {
      max-width: 100%;
      max-height: 100%;
      width: 100%;
      border-radius: 8px;
    }
  }

  .fp-header {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 12px 16px;
    border-bottom: 1px solid var(--border, #444);
    flex-shrink: 0;
  }

  .fp-filename {
    font-size: 14px;
    font-weight: 600;
    color: var(--fg, #657b83);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .fp-path {
    font-size: 11px;
    color: var(--fg-dim, #93a1a1);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    flex: 1;
    min-width: 0;
  }

  .fp-actions {
    display: flex;
    align-items: center;
    gap: 4px;
    flex-shrink: 0;
  }

  .fp-download {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 32px;
    height: 32px;
    color: var(--fg-dim, #93a1a1);
    border-radius: 4px;
    text-decoration: none;
  }
  .fp-download:hover {
    background: var(--bg-secondary, #eee8d5);
    color: var(--fg, #657b83);
  }
  .fp-download svg {
    width: 16px;
    height: 16px;
  }

  .fp-close {
    background: none;
    border: none;
    color: var(--fg-dim, #93a1a1);
    font-size: 22px;
    cursor: pointer;
    padding: 4px 8px;
    line-height: 1;
    border-radius: 4px;
  }
  .fp-close:hover {
    background: var(--bg-secondary, #eee8d5);
    color: var(--fg, #657b83);
  }

  .fp-body {
    flex: 1;
    overflow: auto;
    padding: 16px;
  }

  .fp-error {
    color: var(--error, #dc322f);
    font-size: 14px;
  }

  .fp-loading {
    color: var(--fg-dim, #93a1a1);
    font-size: 14px;
    text-align: center;
    padding: 32px;
  }

  .fp-image {
    max-width: 100%;
    max-height: 70vh;
    display: block;
    margin: 0 auto;
    border-radius: 4px;
  }

  .fp-markdown {
    font-size: 14px;
    line-height: 1.6;
    color: var(--fg, #657b83);
  }
  .fp-markdown :global(h1),
  .fp-markdown :global(h2),
  .fp-markdown :global(h3) {
    color: var(--fg, #657b83);
    margin: 1em 0 0.5em;
  }
  .fp-markdown :global(pre) {
    background: var(--bg-secondary, #eee8d5);
    padding: 12px;
    border-radius: 4px;
    overflow-x: auto;
  }
  .fp-markdown :global(code) {
    font-family: 'Menlo', 'Monaco', 'Courier New', monospace;
    font-size: 13px;
  }
  .fp-markdown :global(img) {
    max-width: 100%;
  }

  .fp-text {
    font-family: 'Menlo', 'Monaco', 'Courier New', monospace;
    font-size: 13px;
    line-height: 1.5;
    color: var(--fg, #657b83);
    background: var(--bg-secondary, #eee8d5);
    padding: 12px;
    border-radius: 4px;
    margin: 0;
    overflow-x: auto;
    white-space: pre-wrap;
    word-break: break-all;
  }

  .fp-download-prompt {
    text-align: center;
    padding: 32px;
    color: var(--fg-dim, #93a1a1);
  }
  .fp-download-prompt p {
    margin-bottom: 16px;
    font-size: 14px;
  }

  .fp-download-btn {
    display: inline-flex;
    align-items: center;
    padding: 10px 24px;
    background: var(--accent, #268bd2);
    color: white;
    border-radius: 6px;
    font-size: 14px;
    text-decoration: none;
    min-height: 44px;
  }
  .fp-download-btn:hover {
    opacity: 0.9;
  }
</style>
