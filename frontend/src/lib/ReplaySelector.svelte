<script lang="ts">
  let {
    isMobile = false,
    onSelect,
  }: {
    isMobile?: boolean;
    onSelect: (mode: 'full' | 'tail') => void;
  } = $props();
</script>

<div class="overlay">
  <div class="dialog">
    <h3>Connect to session</h3>
    <p>Choose how much history to load:</p>
    <div class="options">
      <button
        class="option"
        class:default={isMobile}
        onclick={() => onSelect('tail')}
      >
        <strong>Recent (fast)</strong>
        <span>Last 256 KB of output</span>
      </button>
      <button
        class="option"
        class:default={!isMobile}
        onclick={() => onSelect('full')}
      >
        <strong>Full history</strong>
        <span>Entire scrollback buffer</span>
      </button>
    </div>
  </div>
</div>

<style>
  .overlay {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.7);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 100;
    padding: 16px;
  }

  .dialog {
    background: var(--bg-secondary, #2d2d2d);
    border: 1px solid var(--border, #444);
    border-radius: 12px;
    padding: 24px;
    max-width: 360px;
    width: 100%;
  }

  h3 {
    margin: 0 0 8px;
    font-size: 16px;
    color: var(--fg, #d4d4d4);
  }

  p {
    margin: 0 0 16px;
    font-size: 13px;
    color: var(--fg-dim, #888);
  }

  .options {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .option {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: 2px;
    padding: 12px 16px;
    background: var(--bg, #1e1e1e);
    border: 1px solid var(--border, #444);
    border-radius: 8px;
    color: var(--fg, #d4d4d4);
    text-align: left;
    font-family: inherit;
  }

  .option.default {
    border-color: var(--accent, #0e639c);
  }

  .option:hover {
    background: #333;
  }

  .option strong {
    font-size: 14px;
  }

  .option span {
    font-size: 12px;
    color: var(--fg-dim, #888);
  }
</style>
