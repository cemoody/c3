export type ConnectionState = 'disconnected' | 'connecting' | 'replaying' | 'live' | 'error';
export type PaneState = 'connected' | 'missing' | 'unknown';

export interface WSCallbacks {
  onOutput: (data: Uint8Array) => void;
  onStatus: (paneState: PaneState, epoch: number, cols: number, rows: number) => void;
  onConnectionState: (state: ConnectionState) => void;
  onError: (message: string) => void;
}

export class WebSocketClient {
  private ws: WebSocket | null = null;
  private callbacks: WSCallbacks;
  private basePath: string;
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private reconnectDelay = 1000;
  private maxReconnectDelay = 30000;
  private lastReplayMode: 'full' | 'tail' = 'full';
  private lastTailSize: number = 256 * 1024;

  constructor(callbacks: WSCallbacks, basePath: string = '') {
    this.callbacks = callbacks;
    this.basePath = basePath;
  }

  connect(replayMode: 'full' | 'tail' = 'full', tailSize: number = 256 * 1024): void {
    this.lastReplayMode = replayMode;
    this.lastTailSize = tailSize;
    this.cancelReconnect();
    this.callbacks.onConnectionState('connecting');

    const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
    const ws = new WebSocket(`${proto}//${location.host}${this.basePath}/ws`);
    this.ws = ws;

    ws.onopen = () => {
      this.reconnectDelay = 1000;
      this.callbacks.onConnectionState('replaying');
      this.send({ type: 'hello', replayMode, tailSize });
    };

    ws.onmessage = (ev: MessageEvent) => {
      try {
        const msg = JSON.parse(ev.data);
        this.handleMessage(msg);
      } catch {
        // ignore malformed messages
      }
    };

    ws.onclose = () => {
      this.ws = null;
      this.callbacks.onConnectionState('disconnected');
      this.scheduleReconnect();
    };

    ws.onerror = () => {
      this.callbacks.onConnectionState('error');
    };
  }

  disconnect(): void {
    this.cancelReconnect();
    if (this.ws) {
      this.ws.close();
      this.ws = null;
    }
    this.callbacks.onConnectionState('disconnected');
  }

  sendInput(text: string): void {
    const bytes = new TextEncoder().encode(text);
    const b64 = btoa(String.fromCharCode(...bytes));
    this.send({ type: 'input', data: b64 });
  }

  sendResize(cols: number, rows: number): void {
    this.send({ type: 'resize', cols, rows });
  }

  private send(msg: object): void {
    if (this.ws?.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(msg));
    }
  }

  private handleMessage(msg: any): void {
    switch (msg.type) {
      case 'output': {
        const binary = atob(msg.data);
        const bytes = new Uint8Array(binary.length);
        for (let i = 0; i < binary.length; i++) {
          bytes[i] = binary.charCodeAt(i);
        }
        this.callbacks.onOutput(bytes);
        // Once we receive output, we're live (or still replaying, but good enough)
        this.callbacks.onConnectionState('live');
        break;
      }
      case 'status':
        this.callbacks.onStatus(msg.paneState as PaneState, msg.epoch, msg.cols || 0, msg.rows || 0);
        break;
      case 'error':
        this.callbacks.onError(msg.message);
        break;
    }
  }

  private scheduleReconnect(): void {
    this.cancelReconnect();
    this.reconnectTimer = setTimeout(() => {
      this.connect(this.lastReplayMode, this.lastTailSize);
    }, this.reconnectDelay);
    this.reconnectDelay = Math.min(this.reconnectDelay * 2, this.maxReconnectDelay);
  }

  private cancelReconnect(): void {
    if (this.reconnectTimer !== null) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
  }
}
