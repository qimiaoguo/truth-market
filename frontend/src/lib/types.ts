export interface User {
  id: string
  walletAddress: string
  displayName?: string
  avatarUrl?: string
  userType: 'human' | 'agent'
  balance: string
  lockedBalance: string
  isAdmin: boolean
  createdAt: string
}

export interface Market {
  id: string
  title: string
  description: string
  marketType: 'binary' | 'multi'
  category: string
  status: 'draft' | 'open' | 'closed' | 'resolved' | 'cancelled'
  outcomes: Outcome[]
  createdBy: string
  volume: string
  liquidity: string
  closesAt: string
  createdAt: string
  resolvedAt?: string
  winningOutcomeId?: string
}

export interface Outcome {
  id: string
  marketId: string
  label: string
  index: number
  price: string
  isWinner: boolean
}

export interface Order {
  id: string
  userId: string
  marketId: string
  outcomeId: string
  side: 'buy' | 'sell'
  price: string
  quantity: string
  filledQuantity: string
  status: 'open' | 'partially_filled' | 'filled' | 'cancelled'
  createdAt: string
  updatedAt: string
}

export interface Trade {
  id: string
  marketId: string
  outcomeId: string
  makerOrderId: string
  takerOrderId: string
  makerUserId: string
  takerUserId: string
  price: string
  quantity: string
  createdAt: string
}

export interface Position {
  id: string
  userId: string
  marketId: string
  outcomeId: string
  quantity: string
  avgPrice: string
  currentValue?: string
  updatedAt: string
}

export interface OrderbookLevel {
  price: string
  quantity: string
  orderCount: number
}

export interface Orderbook {
  bids: OrderbookLevel[]
  asks: OrderbookLevel[]
}

export type RankDimension = 'total_assets' | 'pnl' | 'volume' | 'win_rate' | 'trade_count'

export interface UserRanking {
  userId: string
  walletAddress: string
  userType: 'human' | 'agent'
  dimension: RankDimension
  rank: number
  value: string
  updatedAt: string
}

export interface ApiResponse<T> {
  ok: boolean
  data?: T
  error?: {
    code: string
    message: string
  }
  meta?: {
    page: number
    perPage: number
    total: number
  }
}
