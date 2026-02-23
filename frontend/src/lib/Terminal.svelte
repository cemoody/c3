<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { Terminal } from '@xterm/xterm';
  import { WebLinksAddon } from '@xterm/addon-web-links';
  import '@xterm/xterm/css/xterm.css';

  let {
    onData,
    onFileClick,
    isMobile = false,
    composerHeight = 0,
  }: {
    onData: (data: string) => void;
    onFileClick?: (path: string) => void;
    isMobile?: boolean;
    composerHeight?: number;
  } = $props();

  // Regex for file paths with extensions — matches both absolute (/path/to/file.ext)
  // and relative (dir/subdir/file.ext, ./file.ext, ../file.ext)
  const FILE_PATH_RE = /((?:\.\.?\/|\/)(?:[\w.@+\-]+\/)*[\w.@+\-]+\.[\w]+|(?:[\w@+\-]+\/)+[\w.@+\-]+\.[\w]+)/g;

  let containerEl: HTMLDivElement;
  let terminal: Terminal;
  let paneCols = 0;
  let paneRows = 0;

  export function write(data: Uint8Array) {
    terminal?.write(data);
  }

  export function scrollToBottom() {
    terminal?.scrollToBottom();
  }

  export function focusTerminal() {
    terminal?.focus();
  }

  export function isAtBottom(): boolean {
    if (!terminal) return true;
    const buf = terminal.buffer.active;
    return buf.viewportY >= buf.baseY;
  }

  // Calculate the font size that makes `cols` characters fit the container width.
  // On mobile, use a readable font size and allow horizontal scrolling instead.
  function calcFontSize(cols: number): number {
    if (!containerEl || cols <= 0) return isMobile ? 12 : 14;
    if (isMobile) return 12; // Fixed readable size; container scrolls horizontally
    const availWidth = containerEl.clientWidth - 20; // 20px for scrollbar + padding
    // Monospace char width ≈ fontSize * 0.6
    const ideal = availWidth / (cols * 0.602);
    const min = 8;
    const max = 16;
    return Math.max(min, Math.min(max, Math.floor(ideal * 10) / 10));
  }

  // Set the terminal to match the pane dimensions, scaling the font to fit.
  export function setDimensions(cols: number, rows: number) {
    if (!terminal || cols <= 0 || rows <= 0) return;
    paneCols = cols;
    paneRows = rows;

    const fontSize = calcFontSize(cols);
    terminal.options.fontSize = fontSize;
    terminal.resize(cols, rows);
    // Re-focus after resize — resizing can drop focus
    if (!isMobile) terminal.focus();
    // On mobile, scroll to bottom so the input prompt is visible
    terminal.scrollToBottom();
  }

  onMount(() => {
    terminal = new Terminal({
      scrollback: 50000,
      fontSize: isMobile ? 12 : 14,
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

  onDestroy(() => {
    terminal?.dispose();
  });
</script>

<div
  bind:this={containerEl}
  class="terminal-container"
  style:padding-bottom="{composerHeight}px"
></div>

<style>
  .terminal-container {
    width: 100%;
    height: 100%;
    overflow: hidden;
  }
  /* On mobile, allow horizontal scroll for readable font size */
  @media (max-width: 768px) {
    .terminal-container {
      overflow-x: auto;
      -webkit-overflow-scrolling: touch;
    }
  }
</style>
