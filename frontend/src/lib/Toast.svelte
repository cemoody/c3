<script lang="ts">
  type ToastItem = { id: number; message: string; type: 'success' | 'error' };

  let toasts = $state<ToastItem[]>([]);
  let nextId = 0;

  export function show(message: string, type: 'success' | 'error' = 'success') {
    const id = nextId++;
    toasts = [...toasts, { id, message, type }];
    setTimeout(() => {
      toasts = toasts.filter(t => t.id !== id);
    }, 4000);
  }
</script>

{#if toasts.length > 0}
  <div class="toast-container">
    {#each toasts as toast (toast.id)}
      <div class="toast" class:error={toast.type === 'error'}>
        {toast.message}
      </div>
    {/each}
  </div>
{/if}

<style>
  .toast-container {
    position: fixed;
    bottom: 20px;
    right: 20px;
    z-index: 9999;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  .toast {
    padding: 10px 16px;
    background: var(--bg-secondary);
    color: var(--fg);
    border: 1px solid var(--success);
    border-radius: 6px;
    font-size: 13px;
    font-family: inherit;
    box-shadow: 0 2px 8px rgba(0, 0, 0, 0.3);
    max-width: 400px;
    word-break: break-all;
    animation: slideIn 0.2s ease-out;
  }
  .toast.error {
    border-color: var(--error);
  }
  @keyframes slideIn {
    from { transform: translateX(20px); opacity: 0; }
    to { transform: translateX(0); opacity: 1; }
  }
</style>
