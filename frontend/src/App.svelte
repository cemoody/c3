<script lang="ts">
  import { onMount } from 'svelte';
  import TerminalView from './lib/Terminal.svelte';
  import StatusBar from './lib/StatusBar.svelte';
  import Composer from './lib/Composer.svelte';
  import QuickActions from './lib/QuickActions.svelte';
  import JumpToLive from './lib/JumpToLive.svelte';
  import SessionPicker from './lib/SessionPicker.svelte';
  import FileBrowser from './lib/FileBrowser.svelte';
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
  let wsClient: WebSocketClient | null = null;
  let connectionState = $state<ConnectionState>('disconnected');
  let paneState = $state<PaneState>('unknown');
  let showJumpToLive = $state(false);
  let scrollCheckInterval: ReturnType<typeof setInterval> | null = null;

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
    if (!target || pageMode !== 'session') return;
    const items = e.clipboardData?.items;
    if (!items) return;

    for (const item of items) {
      if (item.type.startsWith('image/')) {
        e.preventDefault();
        const file = item.getAsFile();
        if (!file) continue;

        const form = new FormData();
        form.append('image', file, `paste.${item.type.split('/')[1]}`);
        try {
          const res = await fetch(`${basePath}/upload`, { method: 'POST', body: form });
          if (!res.ok) console.error('Upload failed:', await res.text());
        } catch (err) {
          console.error('Paste upload error:', err);
        }
        return;
      }
    }
  }

  onMount(() => {
    if (pageMode === 'session' && target) {
      connectToTarget(target);
    }

    document.addEventListener('paste', handlePaste);

    scrollCheckInterval = setInterval(() => {
      if (terminalRef && !terminalRef.isAtBottom()) {
        showJumpToLive = true;
      } else {
        showJumpToLive = false;
      }
    }, 500);

    return () => {
      document.removeEventListener('paste', handlePaste);
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

  const composerHeight = isMobile ? 110 : 0;
  const basePath = target ? `/s/${encodeURIComponent(target)}` : '';
  const uploadUrl = `${basePath}/upload`;
</script>

{#if pageMode === 'picker'}
  <SessionPicker onSelect={handleSessionSelect} />
{:else}
  <div class="app">
    <StatusBar {connectionState} {paneState} {target} {pageMode} />

    {#if pageMode === 'session'}
      <div class="terminal-wrapper">
        <TerminalView
          bind:this={terminalRef}
          onData={handleInput}
          {isMobile}
          {composerHeight}
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

<style>
  .app {
    display: flex;
    flex-direction: column;
    height: 100%;
  }
  .terminal-wrapper {
    flex: 1;
    overflow: hidden;
  }
  .files-wrapper {
    flex: 1;
    overflow: hidden;
  }
  .mobile-controls {
    position: fixed;
    bottom: 0;
    left: 0;
    right: 0;
    z-index: 10;
  }
</style>
