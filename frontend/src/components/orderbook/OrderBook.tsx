'use client'

import { useQuery } from '@tanstack/react-query'
import { api } from '@/lib/api'
import { Card } from '@/components/ui/Card'
import { Skeleton } from '@/components/ui/Skeleton'

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

  // Max quantity for depth visualization
  const maxQty = Math.max(
    ...bids.map((b) => Number(b.quantity)),
    ...asks.map((a) => Number(a.quantity)),
    1
  )

  if (isLoading) {
    return (
      <Card hover={false}>
        <div className="p-4 space-y-2">
          {[...Array(5)].map((_, i) => (
            <Skeleton key={i} variant="table-row" />
          ))}
        </div>
      </Card>
    )
  }

  return (
    <Card hover={false}>
      <div className="grid grid-cols-2 divide-x divide-card-border">
        {/* Bids */}
        <div>
          <div className="px-4 py-2.5 text-xs font-bold uppercase tracking-wider text-success-700 bg-success-50/50 border-b border-card-border">
            Bids
          </div>
          <div className="divide-y divide-neutral-100">
            {bids.length === 0 ? (
              <div className="px-4 py-6 text-xs text-neutral-400 text-center">
                No bids
              </div>
            ) : (
              bids.map((level, i) => {
                const depthPct = (Number(level.quantity) / maxQty) * 100
                return (
                  <div
                    key={`bid-${i}`}
                    className="relative flex items-center justify-between px-4 py-2 text-sm"
                  >
                    <div
                      className="absolute inset-y-0 left-0 bg-success-100/40"
                      style={{ width: `${depthPct}%` }}
                    />
                    <span className="relative font-semibold text-success-600 tabular-nums">
                      {Number(level.price).toFixed(2)}
                    </span>
                    <span className="relative text-neutral-600 tabular-nums">{level.quantity}</span>
                  </div>
                )
              })
            )}
          </div>
        </div>

        {/* Asks */}
        <div>
          <div className="px-4 py-2.5 text-xs font-bold uppercase tracking-wider text-danger-700 bg-danger-50/50 border-b border-card-border">
            Asks
          </div>
          <div className="divide-y divide-neutral-100">
            {asks.length === 0 ? (
              <div className="px-4 py-6 text-xs text-neutral-400 text-center">
                No asks
              </div>
            ) : (
              asks.map((level, i) => {
                const depthPct = (Number(level.quantity) / maxQty) * 100
                return (
                  <div
                    key={`ask-${i}`}
                    className="relative flex items-center justify-between px-4 py-2 text-sm"
                  >
                    <div
                      className="absolute inset-y-0 right-0 bg-danger-100/40"
                      style={{ width: `${depthPct}%` }}
                    />
                    <span className="relative font-semibold text-danger-600 tabular-nums">
                      {Number(level.price).toFixed(2)}
                    </span>
                    <span className="relative text-neutral-600 tabular-nums">{level.quantity}</span>
                  </div>
                )
              })
            )}
          </div>
        </div>
      </div>
    </Card>
  )
}
