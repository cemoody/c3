<script lang="ts">
  import { onMount } from 'svelte';

  let { onSelect }: { onSelect: (target: string) => void } = $props();

  type Pane = { index: string; currentCommand: string; target: string };
  type Window = { index: string; name: string; panes: Pane[] };
  type Session = { name: string; windows: Window[] };

  let sessions = $state<Session[]>([]);
  let currentTarget = $state('');
  let loading = $state(true);
  let error = $state('');

  async function fetchSessions() {
    loading = true;
    error = '';
    try {
      const res = await fetch('/api/sessions');
      if (!res.ok) throw new Error(await res.text());
      const data = await res.json();
      sessions = data.sessions || [];
      currentTarget = data.currentTarget || '';
    } catch (e: any) {
      error = e.message || 'Failed to load sessions';
    } finally {
      loading = false;
    }
  }

  function selectTarget(target: string) {
    onSelect(target);
  }

  onMount(() => {
    fetchSessions();
    // Refresh every 5 seconds
    const interval = setInterval(fetchSessions, 5000);
    return () => clearInterval(interval);
  });
</script>

<div class="picker-overlay">
  <div class="picker">
    <h2>Select a tmux session</h2>

    {#if loading && sessions.length === 0}
      <p class="hint">Loading sessions...</p>
    {:else if error}
      <p class="error">{error}</p>
    {:else if sessions.length === 0}
      <div class="empty">
        <p>No tmux sessions found.</p>
        <p class="hint">Start one with:</p>
        <pre>tmux new-session -s claude{'\n'}claude</pre>
      </div>
    {:else}
      <div class="sessions">
        {#each sessions as session}
          <div class="session">
            <div class="session-name">{session.name}</div>
            {#each session.windows as window}
              {#each window.panes as pane}
                <button
                  class="pane-btn"
                  class:active={pane.target === currentTarget}
                  onclick={() => selectTarget(pane.target)}
                >
                  <span class="pane-target">{pane.target}</span>
                  <span class="pane-info">{window.name} &mdash; {pane.currentCommand}</span>
                </button>
              {/each}
            {/each}
          </div>
        {/each}
      </div>
    {/if}

    <button class="refresh-btn" onclick={fetchSessions}>Refresh</button>
  </div>
</div>

<style>
  .picker-overlay {
    position: fixed;
    inset: 0;
    background: var(--bg, #1e1e1e);
    display: flex;
    align-items: flex-start;
    justify-content: center;
    z-index: 100;
    padding: 48px 16px;
    overflow-y: auto;
  }

  .picker {
    max-width: 500px;
    width: 100%;
  }

  h2 {
    margin: 0 0 16px;
    font-size: 18px;
    color: var(--fg, #d4d4d4);
  }

  .hint {
    color: var(--fg-dim, #888);
    font-size: 13px;
  }

  .error {
    color: var(--error, #f44747);
    font-size: 13px;
  }

  .empty pre {
    background: var(--bg-secondary, #2d2d2d);
    padding: 12px;
    border-radius: 6px;
    font-size: 13px;
    color: var(--success, #4ec9b0);
  }

  .sessions {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .session {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .session-name {
    font-size: 12px;
    color: var(--fg-dim, #888);
    text-transform: uppercase;
    letter-spacing: 0.5px;
    padding: 0 4px;
  }

  .pane-btn {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: 2px;
    padding: 10px 14px;
    background: var(--bg-secondary, #2d2d2d);
    border: 1px solid var(--border, #444);
    border-radius: 8px;
    color: var(--fg, #d4d4d4);
    text-align: left;
    font-family: inherit;
    width: 100%;
  }

  .pane-btn:hover {
    border-color: var(--accent, #0e639c);
    background: #333;
  }

  .pane-btn.active {
    border-color: var(--success, #4ec9b0);
  }

  .pane-target {
    font-size: 14px;
    font-weight: 600;
  }

  .pane-info {
    font-size: 12px;
    color: var(--fg-dim, #888);
  }

  .refresh-btn {
    margin-top: 16px;
    padding: 8px 16px;
    background: var(--bg-secondary, #2d2d2d);
    color: var(--fg-dim, #888);
    border: 1px solid var(--border, #444);
    border-radius: 6px;
    font-size: 13px;
    font-family: inherit;
  }

  .refresh-btn:hover {
    color: var(--fg, #d4d4d4);
    border-color: var(--accent, #0e639c);
  }
</style>
