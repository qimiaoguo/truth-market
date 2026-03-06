export class TruthMarketError extends Error {
  constructor(
    message: string,
    public code?: string,
    public status?: number,
  ) {
    super(message);
    this.name = 'TruthMarketError';
  }
}

interface ClientOptions {
  baseUrl: string;
  apiKey: string;
  retry?: boolean;
}

export class TruthMarketClient {
  private baseUrl: string;
  private apiKey: string;
  private retry: boolean;

  constructor(options: ClientOptions) {
    this.baseUrl = options.baseUrl;
    this.apiKey = options.apiKey;
    this.retry = options.retry ?? false;
  }

  private async request(
    method: string,
    path: string,
    options?: { body?: unknown; params?: Record<string, string> },
  ): Promise<any> {
    const url = new URL(`${this.baseUrl}${path}`);
    if (options?.params) {
      Object.entries(options.params).forEach(([k, v]) =>
        url.searchParams.set(k, v),
      );
    }

    const headers: Record<string, string> = {
      'X-API-Key': this.apiKey,
    };
    if (options?.body) {
      headers['Content-Type'] = 'application/json';
    }

    let lastError: Error | null = null;
    const maxAttempts = this.retry ? 3 : 1;

    for (let attempt = 0; attempt < maxAttempts; attempt++) {
      let response: Response;
      try {
        response = await fetch(url.toString(), {
          method,
          headers,
          body: options?.body ? JSON.stringify(options.body) : undefined,
        });
      } catch (err) {
        throw new TruthMarketError(
          `Network error: ${(err as Error).message}`,
        );
      }

      const json = await response.json();

      if (response.status === 429 && this.retry && attempt < maxAttempts - 1) {
        const retryAfter = response.headers.get('Retry-After');
        const delay = retryAfter ? parseInt(retryAfter, 10) * 100 : 100;
        await new Promise((resolve) => setTimeout(resolve, delay));
        lastError = new TruthMarketError(
          json.error?.message || 'Rate limited',
          json.error?.code,
          429,
        );
        continue;
      }

      if (!response.ok) {
        throw new TruthMarketError(
          json.error?.message || 'Request failed',
          json.error?.code,
          response.status,
        );
      }

      return json;
    }

    throw lastError || new TruthMarketError('Request failed after retries');
  }

  // Auth
  async getMe() {
    return this.request('GET', '/auth/me');
  }

  // Markets
  async listMarkets(params?: {
    status?: string;
    category?: string;
    page?: number;
    perPage?: number;
  }) {
    const queryParams: Record<string, string> = {};
    if (params?.status) queryParams.status = params.status;
    if (params?.category) queryParams.category = params.category;
    if (params?.page) queryParams.page = String(params.page);
    if (params?.perPage) queryParams.per_page = String(params.perPage);
    return this.request('GET', '/markets', { params: queryParams });
  }

  async getMarket(marketId: string) {
    return this.request('GET', `/markets/${marketId}`);
  }

  async getOrderBook(marketId: string, outcomeId: string) {
    return this.request('GET', `/markets/${marketId}/orderbook`, {
      params: { outcome_id: outcomeId },
    });
  }

  // Trading
  async mintTokens(opts: { marketId: string; quantity: string }) {
    return this.request('POST', '/trading/mint', {
      body: { market_id: opts.marketId, quantity: opts.quantity },
    });
  }

  async placeOrder(opts: {
    marketId: string;
    outcomeId: string;
    side: string;
    price: string;
    quantity: string;
  }) {
    return this.request('POST', '/trading/orders', {
      body: {
        market_id: opts.marketId,
        outcome_id: opts.outcomeId,
        side: opts.side,
        price: opts.price,
        quantity: opts.quantity,
      },
    });
  }

  async cancelOrder(orderId: string) {
    return this.request('DELETE', `/trading/orders/${orderId}`);
  }

  async getPositions(params?: { marketId?: string }) {
    const queryParams: Record<string, string> = {};
    if (params?.marketId) queryParams.market_id = params.marketId;
    return this.request('GET', '/trading/positions', { params: queryParams });
  }

  // Rankings
  async getRankings(params?: {
    dimension?: string;
    userType?: string;
    page?: number;
    perPage?: number;
  }) {
    const queryParams: Record<string, string> = {};
    if (params?.dimension) queryParams.dimension = params.dimension;
    if (params?.userType) queryParams.user_type = params.userType;
    if (params?.page) queryParams.page = String(params.page);
    if (params?.perPage) queryParams.per_page = String(params.perPage);
    return this.request('GET', '/rankings', { params: queryParams });
  }
}
