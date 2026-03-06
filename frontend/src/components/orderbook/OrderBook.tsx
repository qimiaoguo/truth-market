'use client'

import { useQuery } from '@tanstack/react-query'
import { api } from '@/lib/api'

interface OrderBookProps {
  marketId: string
  outcomeId?: string
}

export function OrderBook({ marketId, outcomeId }: OrderBookProps) {
  const { data, isLoading } = useQuery({
    queryKey: ['orderbook', marketId, outcomeId],
    queryFn: () => api.getOrderbook(marketId, outcomeId),
    refetchInterval: 5000,
  })

  const orderbook = data?.data

  const bids = [...(orderbook?.bids ?? [])].sort(
    (a, b) => Number(b.price) - Number(a.price)
  )
  const asks = [...(orderbook?.asks ?? [])].sort(
    (a, b) => Number(a.price) - Number(b.price)
  )

  if (isLoading) {
    return (
      <div className="p-4 text-center text-neutral-500 text-sm">
        Loading orderbook...
      </div>
    )
  }

  return (
    <div className="bg-card rounded-xl border border-card-border overflow-hidden">
      <div className="grid grid-cols-2 divide-x divide-card-border">
        {/* Bids */}
        <div>
          <h3 className="px-4 py-2 text-sm font-semibold text-success-700 bg-success-50 border-b border-card-border">
            Bids
          </h3>
          <div className="divide-y divide-neutral-100">
            {bids.length === 0 ? (
              <div className="px-4 py-3 text-xs text-neutral-400">
                No bids
              </div>
            ) : (
              bids.map((level, i) => (
                <div
                  key={`bid-${i}`}
                  className="flex items-center justify-between px-4 py-2 text-sm"
                >
                  <span className="font-medium text-success-600">
                    {Number(level.price).toFixed(2)}
                  </span>
                  <span className="text-neutral-600">{level.quantity}</span>
                </div>
              ))
            )}
          </div>
        </div>

        {/* Asks */}
        <div>
          <h3 className="px-4 py-2 text-sm font-semibold text-danger-700 bg-danger-50 border-b border-card-border">
            Asks
          </h3>
          <div className="divide-y divide-neutral-100">
            {asks.length === 0 ? (
              <div className="px-4 py-3 text-xs text-neutral-400">
                No asks
              </div>
            ) : (
              asks.map((level, i) => (
                <div
                  key={`ask-${i}`}
                  className="flex items-center justify-between px-4 py-2 text-sm"
                >
                  <span className="font-medium text-danger-600">
                    {Number(level.price).toFixed(2)}
                  </span>
                  <span className="text-neutral-600">{level.quantity}</span>
                </div>
              ))
            )}
          </div>
        </div>
      </div>
    </div>
  )
}
