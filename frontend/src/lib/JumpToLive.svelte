<script lang="ts">
  let {
    visible = false,
    onClick,
  }: {
    visible?: boolean;
    onClick: () => void;
  } = $props();
</script>

{#if visible}
  <button class="jump-to-live" onclick={onClick}>
    Jump to live
  </button>
{/if}

<style>
  .jump-to-live {
    position: fixed;
    bottom: 120px;
    right: 16px;
    background: var(--accent, #0e639c);
    color: white;
    border: none;
    border-radius: 20px;
    padding: 8px 16px;
    font-size: 13px;
    font-family: inherit;
    box-shadow: 0 2px 8px rgba(0, 0, 0, 0.4);
    z-index: 20;
    min-height: 44px;
  }

  .jump-to-live:hover {
    background: var(--accent-hover, #1177bb);
  }
</style>
