<script lang="ts">
  import { onMount } from 'svelte';
  import TerminalView from './lib/Terminal.svelte';
  import StatusBar from './lib/StatusBar.svelte';
  import Composer from './lib/Composer.svelte';
  import QuickActions from './lib/QuickActions.svelte';
  import JumpToLive from './lib/JumpToLive.svelte';
  import SessionPicker from './lib/SessionPicker.svelte';
  import { WebSocketClient, type ConnectionState, type PaneState } from './lib/websocket';

  const isMobile = /iPhone|iPad|iPod|Android/i.test(navigator.userAgent);

  function getTargetFromPath(): string | null {
    const match = location.pathname.match(/^\/s\/([^/]+)/);
    return match ? decodeURIComponent(match[1]) : null;
  }

  let target = $state<string | null>(getTargetFromPath());
  let terminalRef: ReturnType<typeof TerminalView>;
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
        // Set terminal to match the pane dimensions (don't resize the pane)
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

  onMount(() => {
    if (target) {
      connectToTarget(target);
    }

    scrollCheckInterval = setInterval(() => {
      if (terminalRef && !terminalRef.isAtBottom()) {
        showJumpToLive = true;
      } else {
        showJumpToLive = false;
      }
    }, 500);

    return () => {
      wsClient?.disconnect();
      if (scrollCheckInterval) clearInterval(scrollCheckInterval);
    };
  });

  function handleSessionSelect(selectedTarget: string) {
    const path = `/s/${encodeURIComponent(selectedTarget)}/`;
    window.location.href = path;
  }

  function handleInput(data: string) {
    wsClient?.sendInput(data);
  }

  function handleJumpToLive() {
    terminalRef?.scrollToBottom();
    showJumpToLive = false;
  }

  function handleBackToSessions() {
    window.location.href = '/';
  }

  const composerHeight = isMobile ? 110 : 0;
  const basePath = target ? `/s/${encodeURIComponent(target)}` : '';
  const uploadUrl = `${basePath}/upload`;
</script>

{#if !target}
  <SessionPicker onSelect={handleSessionSelect} />
{:else}
  <div class="app">
    <StatusBar {connectionState} {paneState} onBack={handleBackToSessions} {target} />
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
  .mobile-controls {
    position: fixed;
    bottom: 0;
    left: 0;
    right: 0;
    z-index: 10;
  }
</style>
