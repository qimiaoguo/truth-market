'use client'

import { use } from 'react'
import { useMarket } from '@/hooks/useMarket'
import { OrderBook } from '@/components/orderbook/OrderBook'
import { TradingPanel } from '@/components/trading/TradingPanel'
import { MintModal } from '@/components/trading/MintModal'

const statusColors: Record<string, string> = {
  open: 'bg-success-100 text-success-700',
  closed: 'bg-neutral-100 text-neutral-600',
  resolved: 'bg-primary-100 text-primary-700',
  cancelled: 'bg-danger-100 text-danger-700',
  draft: 'bg-neutral-100 text-neutral-500',
}

export default function MarketDetailPage({
  params,
}: {
  params: Promise<{ id: string }>
}) {
  const { id } = use(params)
  const { data: market, isLoading, error } = useMarket(id)

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-16">
        <div className="text-neutral-500">Loading market...</div>
      </div>
    )
  }

  if (error || !market) {
    return (
      <div className="flex items-center justify-center py-16">
        <div className="text-danger-600">Market not found</div>
      </div>
    )
  }

  const outcomes = market.outcomes ?? []
  const firstOutcome = outcomes[0]

  return (
    <div>
      {/* Header */}
      <div className="mb-6">
        <div className="flex items-start justify-between gap-4 mb-2">
          <h1 className="text-2xl font-bold text-neutral-900">
            {market.title}
          </h1>
          <span
            className={`shrink-0 px-3 py-1 text-sm font-medium rounded-full ${
              statusColors[market.status] || 'bg-neutral-100 text-neutral-600'
            }`}
          >
            {market.status}
          </span>
        </div>

        <p className="text-neutral-600 mb-3">{market.description}</p>

        <div className="flex items-center gap-4 text-sm text-neutral-500">
          <span>Category: {market.category}</span>
          <span>Volume: {Number(market.volume || 0).toLocaleString()} U</span>
          <span>Liquidity: {Number(market.liquidity || 0).toLocaleString()} U</span>
        </div>
      </div>

      {/* Outcomes */}
      <div className="mb-6">
        <h2 className="text-lg font-semibold text-neutral-900 mb-3">Outcomes</h2>
        <div className="grid gap-3 sm:grid-cols-2">
          {outcomes.map((outcome) => (
            <div
              key={outcome.id}
              className="flex items-center justify-between p-4 bg-card rounded-xl border border-card-border"
            >
              <span className="font-medium text-neutral-900">
                {outcome.label}
              </span>
              <span className="text-lg font-bold text-primary-600">
                {Math.round(Number(outcome.price) * 100)}¢
              </span>
            </div>
          ))}
        </div>
      </div>

      {/* Mint button */}
      <div className="mb-6">
        <MintModal marketId={id} />
      </div>

      {/* Trading + Orderbook grid */}
      <div className="grid gap-6 lg:grid-cols-3">
        <div className="lg:col-span-2">
          <h2 className="text-lg font-semibold text-neutral-900 mb-3">Order Book</h2>
          <OrderBook marketId={id} outcomeId={firstOutcome?.id} />
        </div>

        <div>
          <h2 className="text-lg font-semibold text-neutral-900 mb-3">Trade</h2>
          <TradingPanel
            marketId={id}
            outcomeId={firstOutcome?.id ?? ''}
          />
        </div>
      </div>
    </div>
  )
}
