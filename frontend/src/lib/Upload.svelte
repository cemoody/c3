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

<label class="upload-btn" class:uploading>
  <input
    type="file"
    accept="image/png,image/jpeg,image/webp"
    capture="environment"
    onchange={handleFileInput}
    hidden
  />
  {uploading ? 'Uploading...' : 'Image'}
</label>
{#if error}
  <span class="error">{error}</span>
{/if}

<style>
  .upload-btn {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    min-width: 44px;
    min-height: 44px;
    padding: 8px 14px;
    background: var(--bg, #1e1e1e);
    color: var(--fg, #d4d4d4);
    border: 1px solid var(--border, #444);
    border-radius: 20px;
    font-size: 13px;
    font-family: inherit;
    cursor: pointer;
  }

  .upload-btn:hover {
    background: #333;
  }

  .upload-btn.uploading {
    opacity: 0.6;
    pointer-events: none;
  }

  .error {
    color: var(--error, #f44747);
    font-size: 12px;
  }
</style>
