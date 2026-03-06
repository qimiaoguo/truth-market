interface WSOptions {
  url: string;
  reconnect?: boolean;
  reconnectDelay?: number;
}

type EventCallback = (data: unknown) => void;

export class TruthMarketWS {
  private url: string;
  private reconnect: boolean;
  private reconnectDelay: number;
  private ws: WebSocket | null = null;
  private listeners: Map<string, EventCallback[]> = new Map();

  constructor(options: WSOptions) {
    this.url = options.url;
    this.reconnect = options.reconnect ?? false;
    this.reconnectDelay = options.reconnectDelay ?? 1000;
  }

  connect(): Promise<void> {
    return new Promise((resolve) => {
      this.ws = new WebSocket(this.url);
      this.ws.onopen = () => resolve();
      this.ws.onmessage = (event: MessageEvent) => {
        const msg = JSON.parse(event.data as string);
        const eventType = msg.type;
        const callbacks = this.listeners.get(eventType) || [];
        callbacks.forEach((cb) => cb(msg.data));
      };
      this.ws.onclose = () => {
        if (this.reconnect) {
          setTimeout(() => this.connect(), this.reconnectDelay);
        }
      };
    });
  }

  subscribeMarket(marketId: string): void {
    this.send({ type: 'subscribe', channel: `market:${marketId}` });
  }

  on(eventType: string, callback: EventCallback): void {
    const existing = this.listeners.get(eventType) || [];
    existing.push(callback);
    this.listeners.set(eventType, existing);
  }

  private send(data: unknown): void {
    if (this.ws && this.ws.readyState === 1) {
      this.ws.send(JSON.stringify(data));
    }
  }

  close(): void {
    this.reconnect = false;
    this.ws?.close();
  }
}
