<script lang="ts">
  let { uploadUrl = '/api/upload' }: { uploadUrl?: string } = $props();

  let uploading = $state(false);
  let error = $state('');

  async function handleFile(file: File) {
    uploading = true;
    error = '';
    const form = new FormData();
    form.append('image', file);
    try {
      const res = await fetch(uploadUrl, { method: 'POST', body: form });
      if (!res.ok) {
        throw new Error(await res.text());
      }
    } catch (e: any) {
      error = e.message || 'Upload failed';
    } finally {
      uploading = false;
    }
  }

  function handleFileInput(e: Event) {
    const input = e.target as HTMLInputElement;
    if (input.files?.[0]) {
      handleFile(input.files[0]);
      input.value = '';
    }
  }
</script>

<label class="action-icon" class:uploading title="Upload image">
  <input
    type="file"
    accept="image/png,image/jpeg,image/webp"
    capture="environment"
    onchange={handleFileInput}
    hidden
  />
  {#if uploading}
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="icon spin"><circle cx="12" cy="12" r="10"/><path d="M12 6v6l4 2"/></svg>
  {:else}
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="icon"><rect x="3" y="3" width="18" height="18" rx="2"/><circle cx="8.5" cy="8.5" r="1.5"/><path d="m21 15-5-5L5 21"/></svg>
  {/if}
</label>
{#if error}
  <span class="error">{error}</span>
{/if}

<style>
  .action-icon {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 44px;
    height: 44px;
    background: var(--bg, #1e1e1e);
    color: var(--fg, #d4d4d4);
    border: 1px solid var(--border, #444);
    border-radius: 10px;
    cursor: pointer;
    flex-shrink: 0;
  }

  .action-icon:active {
    background: var(--accent, #0e639c);
    color: white;
  }

  .action-icon.uploading {
    opacity: 0.6;
    pointer-events: none;
  }

  .icon {
    width: 20px;
    height: 20px;
  }

  .spin {
    animation: spin 1s linear infinite;
  }

  @keyframes spin {
    to { transform: rotate(360deg); }
  }

  .error {
    color: var(--error, #f44747);
    font-size: 12px;
  }
</style>
