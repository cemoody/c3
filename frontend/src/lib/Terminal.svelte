<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { Terminal } from '@xterm/xterm';
  import { WebLinksAddon } from '@xterm/addon-web-links';
  import '@xterm/xterm/css/xterm.css';

  let {
    onData,
    onFileClick,
    isMobile = false,
    fontSizeOverride = null,
  }: {
    onData: (data: string) => void;
    onFileClick?: (path: string) => void;
    isMobile?: boolean;
    fontSizeOverride?: number | null;
  } = $props();

  // Regex for file paths with extensions — matches both absolute (/path/to/file.ext)
  // and relative (dir/subdir/file.ext, ./file.ext, ../file.ext)
  const FILE_PATH_RE = /((?:\.\.?\/|\/)(?:[\w.@+\-]+\/)*[\w.@+\-]+\.[\w]+|(?:[\w@+\-]+\/)+[\w.@+\-]+\.[\w]+)/g;

  let containerEl: HTMLDivElement;
  let terminal: Terminal;
  let paneCols = 0;
  let paneRows = 0;

  export function write(data: Uint8Array) {
    if (!terminal) return;
    const wasAtBottom = isAtBottom();
    terminal.write(data, () => {
      // Guard against xterm.js viewport scroll corruption: if we were at the
      // bottom before the write but aren't now, something (focus, reflow, or
      // browser scroll anchoring) reset scrollTop — snap back to bottom.
      if (wasAtBottom && !isAtBottom()) {
        terminal.scrollToBottom();
      }
    });
  }

  export function scrollToBottom() {
    terminal?.scrollToBottom();
  }

  export function focusTerminal() {
    if (!terminal?.textarea) return;
    // Don't re-focus if already focused — avoids unnecessary scroll events
    if (document.activeElement === terminal.textarea) return;
    // Save viewport scroll position before focus. Some browsers ignore
    // preventScroll:true and scroll the viewport to show the textarea,
    // which corrupts xterm's scroll state and jumps to the top.
    const viewport = containerEl?.querySelector('.xterm-viewport') as HTMLElement | null;
    const savedScrollTop = viewport?.scrollTop;
    terminal.focus();
    if (viewport != null && savedScrollTop != null && viewport.scrollTop !== savedScrollTop) {
      viewport.scrollTop = savedScrollTop;
    }
  }

  export function isAtBottom(): boolean {
    if (!terminal) return true;
    const buf = terminal.buffer.active;
    return buf.viewportY >= buf.baseY;
  }

  // Calculate the font size that makes `cols` characters fit the container width.
  // On mobile, use a readable font size and allow horizontal scrolling instead.
  function calcFontSize(cols: number): number {
    if (fontSizeOverride !== null) return fontSizeOverride;
    if (!containerEl || cols <= 0) return isMobile ? 12 : 14;
    if (isMobile) return 12; // Fixed readable size; container scrolls horizontally
    const availWidth = containerEl.clientWidth - 20; // 20px for scrollbar + padding
    // Monospace char width ≈ fontSize * 0.6
    const ideal = availWidth / (cols * 0.602);
    const min = 8;
    const max = 16;
    return Math.max(min, Math.min(max, Math.floor(ideal * 10) / 10));
  }

  // Calculate the font size that fits all columns in the container width (no capping).
  export function fitToWidth(): number {
    if (!containerEl || paneCols <= 0) return 12;
    const availWidth = containerEl.clientWidth - 20;
    const ideal = availWidth / (paneCols * 0.602);
    return Math.round(ideal * 10) / 10;
  }

  // Set the terminal to match the pane dimensions, scaling the font to fit.
  // On mobile, cap rows to what fits in the container so content doesn't
  // extend behind the mobile controls.
  export function setDimensions(cols: number, rows: number) {
    if (!terminal || cols <= 0 || rows <= 0) return;
    paneCols = cols;
    paneRows = rows;

    const fontSize = calcFontSize(cols);

    let effectiveRows = rows;
    if (isMobile && containerEl) {
      const availHeight = containerEl.clientHeight;
      // Estimate cell height: fontSize * lineHeight (~1.2)
      const cellHeight = fontSize * 1.2;
      const maxRows = Math.floor(availHeight / cellHeight);
      if (maxRows > 0 && maxRows < rows) {
        effectiveRows = maxRows;
      }
    }

    // Skip if nothing actually changed — unnecessary resize() calls on large
    // scrollback buffers trigger reflows that can reset the viewport position.
    const fontChanged = terminal.options.fontSize !== fontSize;
    const dimsChanged = terminal.cols !== cols || terminal.rows !== effectiveRows;
    if (!fontChanged && !dimsChanged) return;

    const wasAtBottom = isAtBottom();

    if (fontChanged) terminal.options.fontSize = fontSize;
    if (dimsChanged) terminal.resize(cols, effectiveRows);

    // Re-focus after resize — resizing can drop focus
    if (!isMobile) terminal.focus();

    // Restore scroll position after the render settles. Only auto-scroll to
    // bottom if the viewport was already there; otherwise respect the user's
    // scroll position (they may be reading earlier output).
    if (wasAtBottom || isMobile) {
      requestAnimationFrame(() => terminal?.scrollToBottom());
    }
  }

  onMount(() => {
    terminal = new Terminal({
      scrollback: 50000,
      fontSize: fontSizeOverride ?? (isMobile ? 12 : 14),
      fontFamily: "'Menlo', 'Monaco', 'Courier New', monospace",
      cursorBlink: true,
      convertEol: false,
      disableStdin: isMobile,
      theme: {
        background: '#fdf6e3',
        foreground: '#657b83',
        cursor: '#586e75',
        cursorAccent: '#fdf6e3',
        selectionBackground: '#eee8d5',
        selectionForeground: '#586e75',
        black: '#073642',
        red: '#dc322f',
        green: '#859900',
        yellow: '#b58900',
        blue: '#268bd2',
        magenta: '#d33682',
        cyan: '#2aa198',
        white: '#eee8d5',
        brightBlack: '#002b36',
        brightRed: '#cb4b16',
        brightGreen: '#586e75',
        brightYellow: '#657b83',
        brightBlue: '#839496',
        brightMagenta: '#6c71c4',
        brightCyan: '#93a1a1',
        brightWhite: '#fdf6e3',
      },
    });

    terminal.loadAddon(new WebLinksAddon());
    terminal.open(containerEl);

    // Custom link provider for file paths in terminal output
    // Must be registered AFTER terminal.open() so the link system is initialized
    terminal.registerLinkProvider({
      provideLinks(y: number, callback: (links: any[] | undefined) => void) {
        const fileClickFn = onFileClick;
        if (!fileClickFn) { callback(undefined); return; }
        const line = terminal.buffer.active.getLine(y - 1);
        if (!line) { callback(undefined); return; }
        const text = line.translateToString(true);
        const links: any[] = [];
        const re = new RegExp(FILE_PATH_RE.source, 'g');
        let match: RegExpExecArray | null;
        while ((match = re.exec(text)) !== null) {
          const startX = match.index + 1; // xterm ranges are 1-indexed
          const endX = startX + match[0].length - 1;
          const filePath = match[0];
          links.push({
            range: {
              start: { x: startX, y },
              end: { x: endX, y },
            },
            text: filePath,
            activate() {
              fileClickFn(filePath);
            },
          });
        }
        callback(links.length > 0 ? links : undefined);
      },
    });

    if (!isMobile) {
      terminal.onData((data: string) => onData(data));
    }

    // Auto-focus so keystrokes work immediately without clicking
    terminal.focus();

    // Re-focus when the browser tab becomes visible (e.g., after Cmd+] navigation)
    function onVisibility() {
      if (!document.hidden && !isMobile) terminal.focus();
    }
    document.addEventListener('visibilitychange', onVisibility);

    // Also focus on any click within the terminal container
    containerEl.addEventListener('mouseenter', () => {
      if (!isMobile) terminal.focus();
    });

    // Re-fit font on window resize
    const observer = new ResizeObserver(() => {
      if (paneCols > 0 && paneRows > 0) {
        setDimensions(paneCols, paneRows);
      }
    });
    observer.observe(containerEl);

    return () => {
      observer.disconnect();
      document.removeEventListener('visibilitychange', onVisibility);
    };
  });

  // Re-apply dimensions when fontSizeOverride changes
  $effect(() => {
    // Access fontSizeOverride to track it
    const _ = fontSizeOverride;
    if (terminal && paneCols > 0 && paneRows > 0) {
      setDimensions(paneCols, paneRows);
    }
  });

  onDestroy(() => {
    terminal?.dispose();
  });
</script>

<div
  bind:this={containerEl}
  class="terminal-container"
></div>

<style>
  .terminal-container {
    position: absolute;
    inset: 0;
    overflow: hidden;
  }
  .terminal-container :global(.xterm) {
    height: 100%;
  }
  .terminal-container :global(.xterm-viewport) {
    overflow-anchor: none;
  }
  /* On mobile, allow horizontal scroll for readable font size */
  @media (max-width: 768px) {
    .terminal-container {
      overflow-x: auto;
      -webkit-overflow-scrolling: touch;
    }
  }
</style>
