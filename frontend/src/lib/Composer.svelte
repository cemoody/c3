<script lang="ts">
  let { onSend }: { onSend: (text: string) => void } = $props();

  let text = $state('');

  function handleSend() {
    if (text.trim()) {
      onSend(text + '\n');
      text = '';
    }
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  }
</script>

<div class="composer">
  <textarea
    bind:value={text}
    onkeydown={handleKeydown}
    placeholder="Type a message..."
    rows={2}
  ></textarea>
  <button class="send-btn" onclick={handleSend} disabled={!text.trim()}>
    Send
  </button>
</div>

<style>
  .composer {
    display: flex;
    gap: 8px;
    padding: 8px;
    padding-bottom: calc(8px + env(safe-area-inset-bottom, 0px));
    background: var(--bg-secondary, #2d2d2d);
    border-top: 1px solid var(--border, #444);
    z-index: 10;
  }

  textarea {
    flex: 1;
    resize: none;
    font-size: 16px;
    font-family: inherit;
    background: var(--bg, #1e1e1e);
    color: var(--fg, #d4d4d4);
    border: 1px solid var(--border, #444);
    border-radius: 6px;
    padding: 8px;
    outline: none;
  }

  textarea:focus {
    border-color: var(--accent, #0e639c);
  }

  .send-btn {
    align-self: flex-end;
    background: var(--accent, #0e639c);
    color: white;
    border: none;
    border-radius: 6px;
    padding: 8px 16px;
    font-size: 14px;
    min-height: 44px;
  }

  .send-btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .send-btn:not(:disabled):hover {
    background: var(--accent-hover, #1177bb);
  }
</style>
