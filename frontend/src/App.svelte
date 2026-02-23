<script lang="ts">
  import { onMount } from 'svelte';
  import TerminalView from './lib/Terminal.svelte';
  import StatusBar from './lib/StatusBar.svelte';
  import Composer from './lib/Composer.svelte';
  import QuickActions from './lib/QuickActions.svelte';
  import JumpToLive from './lib/JumpToLive.svelte';
  import SessionPicker from './lib/SessionPicker.svelte';
  import FileBrowser from './lib/FileBrowser.svelte';
  import Toast from './lib/Toast.svelte';
  import FilePreview from './lib/FilePreview.svelte';
  import Settings from './lib/Settings.svelte';
  import { WebSocketClient, type ConnectionState, type PaneState } from './lib/websocket';

  const isMobile = /iPhone|iPad|iPod|Android/i.test(navigator.userAgent);

  type PageMode = 'picker' | 'session' | 'files';

  function getPageMode(): PageMode {
    if (location.pathname.startsWith('/files')) return 'files';
    if (location.pathname.startsWith('/s/')) return 'session';
    return 'picker';
  }

  function getTargetFromPath(): string | null {
    const match = location.pathname.match(/^\/s\/([^/]+)/);
    return match ? decodeURIComponent(match[1]) : null;
  }

  let pageMode = $state<PageMode>(getPageMode());
  let target = $state<string | null>(getTargetFromPath());
  let terminalRef = $state<ReturnType<typeof TerminalView>>();
  let toastRef: ReturnType<typeof Toast>;
  let wsClient: WebSocketClient | null = null;
  let connectionState = $state<ConnectionState>('disconnected');
  let paneState = $state<PaneState>('unknown');
  let showJumpToLive = $state(false);
  let previewFilePath = $state<string | null>(null);
  let scrollCheckInterval: ReturnType<typeof setInterval> | null = null;
  let settingsOpen = $state(false);

  const FONT_SIZE_KEY = 'c3-font-size';
  let fontSizeOverride = $state<number | null>((() => {
    try {
      const saved = localStorage.getItem(FONT_SIZE_KEY);
      return saved ? Number(saved) : null;
    } catch { return null; }
  })());

  function handleFontSizeChange(size: number | null) {
    fontSizeOverride = size;
    if (size !== null) {
      localStorage.setItem(FONT_SIZE_KEY, String(size));
    } else {
      localStorage.removeItem(FONT_SIZE_KEY);
    }
  }

  function handleFitWidth() {
    if (!terminalRef) return;
    const fitted = terminalRef.fitToWidth();
    const clamped = Math.max(4, Math.min(18, Math.round(fitted)));
    fontSizeOverride = clamped;
    localStorage.setItem(FONT_SIZE_KEY, String(clamped));
  }

  function connectToTarget(t: string) {
    const basePath = `/s/${encodeURIComponent(t)}`;
    wsClient = new WebSocketClient({
      onOutput: (data: Uint8Array) => {
        terminalRef?.write(data);
      },
      onStatus: (ps: PaneState, _epoch: number, cols: number, rows: number) => {
        paneState = ps;
        if (cols > 0 && rows > 0) {
          terminalRef?.setDimensions(cols, rows);
        }
      },
      onConnectionState: (state: ConnectionState) => {
        connectionState = state;
      },
      onError: (message: string) => {
        console.error('WS error:', message);
      },
    }, basePath);

    wsClient.connect('tail');
  }

  async function handlePaste(e: ClipboardEvent) {
    const items = e.clipboardData?.items;
    if (!items) return;

    for (const item of items) {
      if (item.type.startsWith('image/')) {
        e.preventDefault();
        const file = item.getAsFile();
        if (!file) continue;

        const form = new FormData();
        form.append('image', file, `paste.${item.type.split('/')[1]}`);

        // Use session-scoped upload if on a session page (injects prompt),
        // otherwise use the general upload endpoint (just saves file).
        const uploadEndpoint = (pageMode === 'session' && target)
          ? `${basePath}/upload`
          : '/api/upload';

        try {
          const res = await fetch(uploadEndpoint, { method: 'POST', body: form });
          if (!res.ok) {
            toastRef?.show(`Upload failed: ${await res.text()}`, 'error');
            return;
          }
          const json = await res.json();
          toastRef?.show(`Saved to ${json.path}`);
        } catch (err) {
          toastRef?.show(`Upload error: ${err}`, 'error');
        }
        return;
      }
    }
  }

  onMount(() => {
    if (pageMode === 'session' && target) {
      connectToTarget(target);
    }

    // Use capture phase so image paste is detected before xterm.js consumes the event
    document.addEventListener('paste', handlePaste, true);

    scrollCheckInterval = setInterval(() => {
      if (terminalRef && !terminalRef.isAtBottom()) {
        showJumpToLive = true;
      } else {
        showJumpToLive = false;
      }
      // Keep terminal focused when document is active, but don't steal
      // focus from inputs (e.g., tab rename, search bar)
      const active = document.activeElement;
      const isInput = active?.tagName === 'INPUT' || active?.tagName === 'TEXTAREA';
      if (pageMode === 'session' && !document.hidden && terminalRef && !isInput) {
        terminalRef.focusTerminal();
      }
    }, 500);

    return () => {
      document.removeEventListener('paste', handlePaste, true);
      wsClient?.disconnect();
      if (scrollCheckInterval) clearInterval(scrollCheckInterval);
    };
  });

  function handleSessionSelect(selectedTarget: string) {
    window.location.href = `/s/${encodeURIComponent(selectedTarget)}/`;
  }

  function handleInput(data: string) {
    wsClient?.sendInput(data);
  }

  function handleJumpToLive() {
    terminalRef?.scrollToBottom();
    showJumpToLive = false;
  }

  async function handleFileClick(filePath: string) {
    // Absolute paths are ready to use
    if (filePath.startsWith('/')) {
      previewFilePath = filePath;
      return;
    }
    // Relative paths: resolve against the pane's current working directory
    try {
      const res = await fetch('/api/sessions');
      if (res.ok) {
        const data = await res.json();
        for (const sess of data.sessions || []) {
          for (const win of sess.windows) {
            for (const pane of win.panes) {
              if (pane.target === target && pane.currentPath) {
                previewFilePath = pane.currentPath.replace(/\/$/, '') + '/' + filePath.replace(/^\.\//, '');
                return;
              }
            }
          }
        }
      }
    } catch {}
    // Fallback: use as-is
    previewFilePath = filePath;
  }

  const basePath = target ? `/s/${encodeURIComponent(target)}` : '';
  const uploadUrl = `${basePath}/upload`;
</script>

{#if pageMode === 'picker'}
  <SessionPicker onSelect={handleSessionSelect} />
{:else}
  <div class="app">
    <StatusBar {connectionState} {paneState} {target} {pageMode} onSettingsToggle={() => settingsOpen = !settingsOpen} />

    {#if pageMode === 'session'}
      <div class="terminal-wrapper">
        <TerminalView
          bind:this={terminalRef}
          onData={handleInput}
          onFileClick={handleFileClick}
          {isMobile}
          {fontSizeOverride}
        />
      </div>

      {#if isMobile}
        <div class="mobile-controls">
          <QuickActions onAction={handleInput} {uploadUrl} />
          <Composer onSend={handleInput} />
        </div>
      {/if}

      <JumpToLive visible={showJumpToLive} onClick={handleJumpToLive} />
    {:else if pageMode === 'files'}
      <div class="files-wrapper">
        <FileBrowser />
      </div>
    {/if}
  </div>
{/if}

{#if previewFilePath}
  <FilePreview path={previewFilePath} onClose={() => previewFilePath = null} />
{/if}

{#if settingsOpen}
  <Settings
    fontSize={fontSizeOverride}
    onFontSizeChange={handleFontSizeChange}
    onFitWidth={handleFitWidth}
    onClose={() => settingsOpen = false}
  />
{/if}

<Toast bind:this={toastRef} />

<style>
  .app {
    display: flex;
    flex-direction: column;
    height: 100%;
  }
  .terminal-wrapper {
    flex: 1;
    min-height: 0;
    position: relative;
    overflow: hidden;
  }
  .files-wrapper {
    flex: 1;
    overflow: hidden;
  }
  .mobile-controls {
    flex-shrink: 0;
    z-index: 10;
  }
</style>
