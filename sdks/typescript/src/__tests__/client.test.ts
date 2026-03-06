import { http, HttpResponse } from 'msw';
import { setupServer } from 'msw/node';
import { describe, it, expect, beforeAll, afterAll, afterEach } from 'vitest';
import { TruthMarketClient, TruthMarketError } from '../client';

const BASE_URL = 'http://localhost:8080/api/v1';

const server = setupServer();

beforeAll(() => server.listen({ onUnhandledRequest: 'error' }));
afterEach(() => server.resetHandlers());
afterAll(() => server.close());

function createClient(opts?: { retry?: boolean }) {
  return new TruthMarketClient({
    baseUrl: BASE_URL,
    apiKey: 'tm_test_key_12345',
    ...(opts?.retry !== undefined ? { retry: opts.retry } : {}),
  });
}

describe('TruthMarketClient', () => {
  // ── Test 1: authenticates with API key ──────────────────────────────
  it('authenticates with API key', async () => {
    let capturedHeaders: Record<string, string> = {};

    server.use(
      http.get(`${BASE_URL}/auth/me`, ({ request }) => {
        capturedHeaders = Object.fromEntries(request.headers.entries());
        return HttpResponse.json({
          ok: true,
          data: {
            user: {
              id: 'user-1',
              wallet_address: '0xabc',
              user_type: 'agent',
            },
          },
        });
      }),
    );

    const client = createClient();
    const result = await client.getMe();

    expect(capturedHeaders['x-api-key']).toBe('tm_test_key_12345');
    expect(result.data.user.id).toBe('user-1');
    expect(result.data.user.wallet_address).toBe('0xabc');
    expect(result.data.user.user_type).toBe('agent');
  });

  // ── Test 2: lists markets ───────────────────────────────────────────
  it('lists markets', async () => {
    server.use(
      http.get(`${BASE_URL}/markets`, () => {
        return HttpResponse.json({
          ok: true,
          data: {
            markets: [
              { id: 'market-1', title: 'Will it rain tomorrow?' },
              { id: 'market-2', title: 'Will BTC hit 100k?' },
            ],
          },
          meta: { page: 1, per_page: 20, total: 2 },
        });
      }),
    );

    const client = createClient();
    const result = await client.listMarkets();

    expect(result.data.markets).toHaveLength(2);
    expect(result.data.markets[0].id).toBe('market-1');
    expect(result.data.markets[1].id).toBe('market-2');
    expect(result.meta).toEqual({ page: 1, per_page: 20, total: 2 });
  });

  // ── Test 3: gets market detail with orderbook ───────────────────────
  it('gets market detail with orderbook', async () => {
    server.use(
      http.get(`${BASE_URL}/markets/market-1`, () => {
        return HttpResponse.json({
          ok: true,
          data: {
            market: {
              id: 'market-1',
              title: 'Will it rain tomorrow?',
              status: 'active',
            },
            outcomes: [
              { id: 'outcome-1', label: 'Yes', price: '0.65' },
              { id: 'outcome-2', label: 'No', price: '0.35' },
            ],
          },
        });
      }),
      http.get(`${BASE_URL}/markets/market-1/orderbook`, ({ request }) => {
        const url = new URL(request.url);
        const outcomeId = url.searchParams.get('outcome_id');
        expect(outcomeId).toBe('outcome-1');
        return HttpResponse.json({
          ok: true,
          data: {
            bids: [
              { price: '0.60', quantity: '100' },
              { price: '0.55', quantity: '200' },
            ],
            asks: [
              { price: '0.65', quantity: '150' },
              { price: '0.70', quantity: '80' },
            ],
          },
        });
      }),
    );

    const client = createClient();

    const marketResult = await client.getMarket('market-1');
    expect(marketResult.data.market.id).toBe('market-1');
    expect(marketResult.data.market.status).toBe('active');
    expect(marketResult.data.outcomes).toHaveLength(2);

    const orderbookResult = await client.getOrderBook('market-1', 'outcome-1');
    expect(orderbookResult.data.bids).toHaveLength(2);
    expect(orderbookResult.data.asks).toHaveLength(2);
    expect(orderbookResult.data.bids[0].price).toBe('0.60');
    expect(orderbookResult.data.asks[0].price).toBe('0.65');
  });

  // ── Test 4: mints tokens ────────────────────────────────────────────
  it('mints tokens', async () => {
    let capturedBody: unknown;

    server.use(
      http.post(`${BASE_URL}/trading/mint`, async ({ request }) => {
        capturedBody = await request.json();
        return HttpResponse.json({
          ok: true,
          data: {
            positions: [
              { outcome_id: 'outcome-1', quantity: '100', label: 'Yes' },
              { outcome_id: 'outcome-2', quantity: '100', label: 'No' },
            ],
          },
        });
      }),
    );

    const client = createClient();
    const result = await client.mintTokens({
      marketId: 'market-1',
      quantity: '100',
    });

    expect(capturedBody).toEqual({
      market_id: 'market-1',
      quantity: '100',
    });
    expect(result.data.positions).toHaveLength(2);
    expect(result.data.positions[0].outcome_id).toBe('outcome-1');
    expect(result.data.positions[1].quantity).toBe('100');
  });

  // ── Test 5: places limit order ──────────────────────────────────────
  it('places limit order', async () => {
    let capturedBody: unknown;

    server.use(
      http.post(`${BASE_URL}/trading/orders`, async ({ request }) => {
        capturedBody = await request.json();
        return HttpResponse.json({
          ok: true,
          data: {
            order: {
              id: 'order-1',
              status: 'open',
              market_id: 'm-1',
              outcome_id: 'o-1',
              side: 'buy',
              price: '0.65',
              quantity: '50',
            },
          },
        });
      }),
    );

    const client = createClient();
    const result = await client.placeOrder({
      marketId: 'm-1',
      outcomeId: 'o-1',
      side: 'buy',
      price: '0.65',
      quantity: '50',
    });

    expect(capturedBody).toEqual({
      market_id: 'm-1',
      outcome_id: 'o-1',
      side: 'buy',
      price: '0.65',
      quantity: '50',
    });
    expect(result.data.order.id).toBe('order-1');
    expect(result.data.order.status).toBe('open');
    expect(result.data.order.side).toBe('buy');
    expect(result.data.order.price).toBe('0.65');
  });

  // ── Test 6: cancels order ───────────────────────────────────────────
  it('cancels order', async () => {
    server.use(
      http.delete(`${BASE_URL}/trading/orders/order-1`, () => {
        return HttpResponse.json({
          ok: true,
          data: {
            order: {
              id: 'order-1',
              status: 'cancelled',
            },
          },
        });
      }),
    );

    const client = createClient();
    const result = await client.cancelOrder('order-1');

    expect(result.data.order.id).toBe('order-1');
    expect(result.data.order.status).toBe('cancelled');
  });

  // ── Test 7: gets positions ──────────────────────────────────────────
  it('gets positions', async () => {
    server.use(
      http.get(`${BASE_URL}/trading/positions`, () => {
        return HttpResponse.json({
          ok: true,
          data: {
            positions: [
              {
                market_id: 'market-1',
                outcome_id: 'outcome-1',
                quantity: '50',
                avg_price: '0.60',
              },
              {
                market_id: 'market-1',
                outcome_id: 'outcome-2',
                quantity: '30',
                avg_price: '0.40',
              },
            ],
          },
        });
      }),
    );

    const client = createClient();
    const result = await client.getPositions();

    expect(result.data.positions).toHaveLength(2);
    expect(result.data.positions[0].market_id).toBe('market-1');
    expect(result.data.positions[0].quantity).toBe('50');
    expect(result.data.positions[1].avg_price).toBe('0.40');
  });

  // ── Test 8: gets rankings ───────────────────────────────────────────
  it('gets rankings', async () => {
    server.use(
      http.get(`${BASE_URL}/rankings`, ({ request }) => {
        const url = new URL(request.url);
        expect(url.searchParams.get('dimension')).toBe('pnl');
        expect(url.searchParams.get('user_type')).toBe('human');
        return HttpResponse.json({
          ok: true,
          data: {
            rankings: [
              { rank: 1, user_id: 'user-1', value: '5000.00' },
              { rank: 2, user_id: 'user-2', value: '3200.00' },
              { rank: 3, user_id: 'user-3', value: '1800.00' },
            ],
          },
          meta: { page: 1, per_page: 20, total: 3 },
        });
      }),
    );

    const client = createClient();
    const result = await client.getRankings({
      dimension: 'pnl',
      userType: 'human',
    });

    expect(result.data.rankings).toHaveLength(3);
    expect(result.data.rankings[0].rank).toBe(1);
    expect(result.data.rankings[0].value).toBe('5000.00');
    expect(result.meta).toEqual({ page: 1, per_page: 20, total: 3 });
  });

  // ── Test 9: handles rate limit errors with retry ────────────────────
  it('handles rate limit errors with retry', async () => {
    let requestCount = 0;

    server.use(
      http.get(`${BASE_URL}/markets`, () => {
        requestCount++;
        if (requestCount === 1) {
          return HttpResponse.json(
            {
              ok: false,
              error: {
                code: 'RATE_LIMITED',
                message: 'Too many requests',
              },
            },
            { status: 429, headers: { 'Retry-After': '1' } },
          );
        }
        return HttpResponse.json({
          ok: true,
          data: {
            markets: [{ id: 'market-1', title: 'Will it rain tomorrow?' }],
          },
          meta: { page: 1, per_page: 20, total: 1 },
        });
      }),
    );

    const client = createClient({ retry: true });
    const result = await client.listMarkets();

    expect(requestCount).toBe(2);
    expect(result.data.markets).toHaveLength(1);
    expect(result.data.markets[0].id).toBe('market-1');
  });

  // ── Test 10: handles network errors gracefully ──────────────────────
  it('handles network errors gracefully', async () => {
    server.use(
      http.get(`${BASE_URL}/markets`, () => {
        return HttpResponse.error();
      }),
    );

    const client = createClient();

    await expect(client.listMarkets()).rejects.toThrow(TruthMarketError);
    await expect(client.listMarkets()).rejects.toThrow(/network/i);
  });
});
