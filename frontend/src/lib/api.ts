import type { ApiResponse, User, Market, Orderbook, Order, Position, UserRanking } from './types'

const API_BASE = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080/api/v1'

class ApiClient {
  private baseUrl: string
  private token: string | null = null

  constructor(baseUrl: string) {
    this.baseUrl = baseUrl
  }

  setToken(token: string | null) {
    this.token = token
  }

  private async request<T>(
    path: string,
    options: RequestInit = {}
  ): Promise<ApiResponse<T>> {
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
      ...((options.headers as Record<string, string>) || {}),
    }

    if (this.token) {
      headers['Authorization'] = `Bearer ${this.token}`
    }

    const response = await fetch(`${this.baseUrl}${path}`, {
      ...options,
      headers,
    })

    return response.json()
  }

  async get<T>(path: string): Promise<ApiResponse<T>> {
    return this.request<T>(path)
  }

  async post<T>(path: string, body?: unknown): Promise<ApiResponse<T>> {
    return this.request<T>(path, {
      method: 'POST',
      body: body ? JSON.stringify(body) : undefined,
    })
  }

  async put<T>(path: string, body?: unknown): Promise<ApiResponse<T>> {
    return this.request<T>(path, {
      method: 'PUT',
      body: body ? JSON.stringify(body) : undefined,
    })
  }

  async delete<T>(path: string): Promise<ApiResponse<T>> {
    return this.request<T>(path, { method: 'DELETE' })
  }

  // Auth
  async getNonce(walletAddress: string) {
    return this.get<{ nonce: string }>(`/auth/nonce?wallet_address=${walletAddress}`)
  }

  async verifySignature(message: string, signature: string, walletAddress: string) {
    return this.post<{ token: string; user: User }>('/auth/verify', { message, signature, wallet_address: walletAddress })
  }

  async getMe() {
    return this.get<User>('/auth/me')
  }

  // Markets
  async listMarkets(params?: { status?: string; category?: string; page?: number; per_page?: number }) {
    const searchParams = new URLSearchParams()
    if (params?.status) searchParams.set('status', params.status)
    if (params?.category) searchParams.set('category', params.category)
    if (params?.page) searchParams.set('page', String(params.page))
    if (params?.per_page) searchParams.set('per_page', String(params.per_page))
    const qs = searchParams.toString()
    return this.get<{ markets: Market[] }>(`/markets${qs ? `?${qs}` : ''}`)
  }

  async getMarket(id: string) {
    return this.get<{ market: Market }>(`/markets/${id}`)
  }

  async getOrderbook(marketId: string, outcomeId?: string) {
    const qs = outcomeId ? `?outcome_id=${outcomeId}` : ''
    return this.get<Orderbook>(`/markets/${marketId}/orderbook${qs}`)
  }

  // Trading
  async mintTokens(marketId: string, quantity: string) {
    return this.post<{ positions: Position[] }>('/trading/mint', { market_id: marketId, quantity })
  }

  async placeOrder(params: { marketId: string; outcomeId: string; side: string; price: string; quantity: string }) {
    return this.post<{ order: Order }>('/trading/orders', {
      market_id: params.marketId,
      outcome_id: params.outcomeId,
      side: params.side,
      price: params.price,
      quantity: params.quantity,
    })
  }

  async cancelOrder(orderId: string) {
    return this.delete<{ order: Order }>(`/trading/orders/${orderId}`)
  }

  async getOrders(params?: { status?: string; market_id?: string; page?: number; per_page?: number }) {
    const searchParams = new URLSearchParams()
    if (params?.status) searchParams.set('status', params.status)
    if (params?.market_id) searchParams.set('market_id', params.market_id)
    if (params?.page) searchParams.set('page', String(params.page))
    if (params?.per_page) searchParams.set('per_page', String(params.per_page))
    const qs = searchParams.toString()
    return this.get<{ orders: Order[] }>(`/trading/orders${qs ? `?${qs}` : ''}`)
  }

  async getPositions(params?: { market_id?: string }) {
    const qs = params?.market_id ? `?market_id=${params.market_id}` : ''
    return this.get<{ positions: Position[] }>(`/trading/positions${qs}`)
  }

  // Rankings
  async getRankings(params?: { dimension?: string; user_type?: string; page?: number; per_page?: number }) {
    const searchParams = new URLSearchParams()
    if (params?.dimension) searchParams.set('dimension', params.dimension)
    if (params?.user_type) searchParams.set('user_type', params.user_type)
    if (params?.page) searchParams.set('page', String(params.page))
    if (params?.per_page) searchParams.set('per_page', String(params.per_page))
    const qs = searchParams.toString()
    return this.get<{ rankings: UserRanking[] }>(`/rankings${qs ? `?${qs}` : ''}`)
  }

  async getUserRankings(userId: string) {
    return this.get<{ rankings: UserRanking[] }>(`/rankings/user/${userId}`)
  }

  // Admin
  async createMarket(params: { title: string; description: string; category: string; market_type: string; outcomes: string[]; closes_at: string }) {
    return this.post<{ market: Market }>('/admin/markets', params)
  }

  async updateMarketStatus(id: string, status: string) {
    return this.post<Market>(`/admin/markets/${id}/status`, { status })
  }

  async resolveMarket(id: string, winningOutcomeId: string) {
    return this.post<{ market: Market }>(`/admin/markets/${id}/resolve`, { winning_outcome_id: winningOutcomeId })
  }
}

export const api = new ApiClient(API_BASE)
