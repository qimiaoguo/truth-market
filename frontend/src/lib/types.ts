export interface User {
  id: string
  wallet_address: string
  display_name?: string
  avatar_url?: string
  user_type: 'human' | 'agent'
  balance: string
  locked_balance: string
  is_admin: boolean
  created_at: string
}

export interface Market {
  id: string
  title: string
  description: string
  market_type: 'binary' | 'multi'
  category: string
  status: 'draft' | 'open' | 'closed' | 'resolved' | 'cancelled'
  outcomes: Outcome[]
  created_by: string
  volume: string
  liquidity: string
  end_time: string
  created_at: string
  resolved_at?: string
  winning_outcome_id?: string
}

export interface Outcome {
  id: string
  market_id: string
  label: string
  index: number
  price: string
  is_winner: boolean
}

export interface Order {
  id: string
  user_id: string
  market_id: string
  outcome_id: string
  side: 'buy' | 'sell'
  price: string
  quantity: string
  filled_quantity: string
  status: 'open' | 'partially_filled' | 'filled' | 'cancelled'
  created_at: string
  updated_at: string
}

export interface Trade {
  id: string
  market_id: string
  outcome_id: string
  maker_order_id: string
  taker_order_id: string
  maker_user_id: string
  taker_user_id: string
  price: string
  quantity: string
  created_at: string
}

export interface Position {
  id: string
  user_id: string
  market_id: string
  outcome_id: string
  quantity: string
  avg_price: string
  current_value?: string
  updated_at: string
}

export interface OrderbookLevel {
  price: string
  quantity: string
  order_count: number
}

export interface Orderbook {
  bids: OrderbookLevel[]
  asks: OrderbookLevel[]
}

export type RankDimension = 'total_assets' | 'pnl' | 'volume' | 'win_rate' | 'trade_count'

export interface UserRanking {
  user_id: string
  wallet_address: string
  user_type: 'human' | 'agent'
  dimension: RankDimension
  rank: number
  value: string
  updated_at: string
}

export interface ApiResponse<T> {
  ok: boolean
  data?: T
  error?: string
  meta?: {
    page: number
    per_page: number
    total: number
  }
}
