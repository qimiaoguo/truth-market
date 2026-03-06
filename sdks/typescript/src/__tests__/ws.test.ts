import { describe, it, expect, beforeEach, vi } from 'vitest';
import { TruthMarketWS } from '../ws';

// ── Minimal in-process WebSocket mock ─────────────────────────────────
// We mock the WebSocket class to test our WS client without a real server.

type MockWSListener = (event: { data: string }) => void;
type MockCloseListener = () => void;
type MockOpenListener = () => void;

class MockWebSocket {
  static instances: MockWebSocket[] = [];

  url: string;
  readyState: number = 0; // CONNECTING
  onopen: MockOpenListener | null = null;
  onmessage: MockWSListener | null = null;
  onclose: MockCloseListener | null = null;
  onerror: ((event: unknown) => void) | null = null;
  sentMessages: string[] = [];

  constructor(url: string) {
    this.url = url;
    MockWebSocket.instances.push(this);
    // Simulate async connection open
    setTimeout(() => {
      this.readyState = 1; // OPEN
      this.onopen?.();
    }, 0);
  }

  send(data: string) {
    this.sentMessages.push(data);
  }

  close() {
    this.readyState = 3; // CLOSED
    this.onclose?.();
  }

  // Helper to simulate receiving a message from the server
  simulateMessage(data: unknown) {
    this.onmessage?.({ data: JSON.stringify(data) });
  }

  // Helper to simulate connection drop
  simulateDisconnect() {
    this.readyState = 3; // CLOSED
    this.onclose?.();
  }
}

// Replace global WebSocket with our mock
vi.stubGlobal('WebSocket', MockWebSocket);

describe('TruthMarketWS', () => {
  beforeEach(() => {
    MockWebSocket.instances = [];
  });

  // ── Test 1: subscribes to market channel ────────────────────────────
  it('subscribes to market channel', async () => {
    const ws = new TruthMarketWS({
      url: 'ws://localhost:8080/ws',
    });

    await ws.connect();

    ws.subscribeMarket('market-1');

    // Get the most recent MockWebSocket instance
    const mockSocket = MockWebSocket.instances[MockWebSocket.instances.length - 1];
    expect(mockSocket.sentMessages).toHaveLength(1);

    const sent = JSON.parse(mockSocket.sentMessages[0]);
    expect(sent).toEqual({
      type: 'subscribe',
      channel: 'market:market-1',
    });
  });

  // ── Test 2: receives orderbook updates ──────────────────────────────
  it('receives orderbook updates', async () => {
    const ws = new TruthMarketWS({
      url: 'ws://localhost:8080/ws',
    });

    await ws.connect();

    const receivedEvents: unknown[] = [];
    ws.on('orderbook_update', (data: unknown) => {
      receivedEvents.push(data);
    });

    ws.subscribeMarket('market-1');

    // Simulate the server pushing an orderbook update
    const mockSocket = MockWebSocket.instances[MockWebSocket.instances.length - 1];
    const orderbookUpdate = {
      type: 'orderbook_update',
      channel: 'market:market-1',
      data: {
        bids: [{ price: '0.60', quantity: '100' }],
        asks: [{ price: '0.65', quantity: '150' }],
      },
    };
    mockSocket.simulateMessage(orderbookUpdate);

    expect(receivedEvents).toHaveLength(1);
    expect(receivedEvents[0]).toEqual(orderbookUpdate.data);
  });

  // ── Test 3: reconnects on disconnect ────────────────────────────────
  it('reconnects on disconnect', async () => {
    const ws = new TruthMarketWS({
      url: 'ws://localhost:8080/ws',
      reconnect: true,
      reconnectDelay: 10, // Fast reconnect for tests
    });

    await ws.connect();

    const initialInstanceCount = MockWebSocket.instances.length;

    // Simulate a disconnection
    const mockSocket = MockWebSocket.instances[MockWebSocket.instances.length - 1];
    mockSocket.simulateDisconnect();

    // Wait for reconnect to trigger (give it time for the setTimeout)
    await new Promise((resolve) => setTimeout(resolve, 50));

    // A new WebSocket instance should have been created for the reconnection
    expect(MockWebSocket.instances.length).toBeGreaterThan(initialInstanceCount);
  });
});
