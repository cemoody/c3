<script lang="ts">
  import Modal from './Modal.svelte';

  let {
    fontSize,
    onFontSizeChange,
    onFitWidth,
    onClose,
  }: {
    fontSize: number | null;
    onFontSizeChange: (size: number | null) => void;
    onFitWidth: () => void;
    onClose: () => void;
  } = $props();

  let sliderValue = $state(fontSize ?? 12);

  function handleSlider(e: Event) {
    const val = parseInt((e.target as HTMLInputElement).value);
    sliderValue = val;
    onFontSizeChange(val);
  }

  function handleReset() {
    sliderValue = 12;
    onFontSizeChange(null);
  }
</script>

<Modal title="Settings" {onClose} panelWidth="300px">
  <div class="settings-body">
    <div class="setting-group">
      <label class="setting-label" for="font-size-slider">
        Font Size
        <span class="setting-value">{fontSize !== null ? `${sliderValue}px` : 'Auto'}</span>
      </label>
      <div class="slider-row">
        <span class="slider-bound">4</span>
        <input
          id="font-size-slider"
          type="range"
          min="4"
          max="18"
          step="1"
          value={sliderValue}
          oninput={handleSlider}
          class="font-slider"
        />
        <span class="slider-bound">18</span>
      </div>
      <div class="setting-actions">
        <button class="action-btn" onclick={onFitWidth}>Fit to Screen</button>
        <button class="action-btn secondary" onclick={handleReset}>Reset to Auto</button>
      </div>
    </div>
  </div>
</Modal>

<style>
  .settings-body {
    padding: 16px;
    flex: 1;
    overflow-y: auto;
  }

  .setting-group {
    display: flex;
    flex-direction: column;
    gap: 10px;
  }

  .setting-label {
    display: flex;
    justify-content: space-between;
    align-items: center;
    font-size: 13px;
    color: var(--fg);
    font-weight: 500;
  }

  .setting-value {
    font-size: 12px;
    color: var(--fg-dim);
    font-weight: 400;
    font-family: 'Menlo', 'Monaco', 'Courier New', monospace;
  }

  .slider-row {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .slider-bound {
    font-size: 10px;
    color: var(--fg-dim);
    flex-shrink: 0;
    min-width: 16px;
    text-align: center;
  }

  .font-slider {
    flex: 1;
    height: 4px;
    appearance: none;
    -webkit-appearance: none;
    background: var(--border);
    border-radius: 2px;
    outline: none;
  }
  .font-slider::-webkit-slider-thumb {
    appearance: none;
    -webkit-appearance: none;
    width: 16px;
    height: 16px;
    border-radius: 50%;
    background: var(--accent);
    cursor: pointer;
  }
  .font-slider::-moz-range-thumb {
    width: 16px;
    height: 16px;
    border-radius: 50%;
    background: var(--accent);
    cursor: pointer;
    border: none;
  }

  .setting-actions {
    display: flex;
    gap: 8px;
  }

  .action-btn {
    flex: 1;
    padding: 8px 12px;
    background: var(--accent);
    color: white;
    border: none;
    border-radius: 4px;
    font-size: 12px;
    font-family: inherit;
    cursor: pointer;
    min-height: 36px;
  }
  .action-btn:hover {
    opacity: 0.9;
  }
  .action-btn.secondary {
    background: var(--bg);
    color: var(--fg);
    border: 1px solid var(--border);
  }
  .action-btn.secondary:hover {
    border-color: var(--fg-dim);
  }
</style>
