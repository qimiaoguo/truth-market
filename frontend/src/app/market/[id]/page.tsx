'use client'

import { use, useState } from 'react'
import { useMarket } from '@/hooks/useMarket'
import { OrderBook } from '@/components/orderbook/OrderBook'
import { TradingPanel } from '@/components/trading/TradingPanel'
import { MintModal } from '@/components/trading/MintModal'
import { Badge } from '@/components/ui/Badge'
import { Skeleton } from '@/components/ui/Skeleton'
import { EmptyState } from '@/components/ui/EmptyState'

const statusVariant: Record<string, 'success' | 'neutral' | 'primary' | 'danger'> = {
  open: 'success',
  closed: 'neutral',
  resolved: 'primary',
  cancelled: 'danger',
  draft: 'neutral',
}

export default function MarketDetailPage({
  params,
}: {
  params: Promise<{ id: string }>
}) {
  const { id } = use(params)
  const { data: market, isLoading, error } = useMarket(id)
  const [selectedOutcomeId, setSelectedOutcomeId] = useState<string | null>(null)

  if (isLoading) {
    return (
      <div className="space-y-6">
        <Skeleton variant="text" className="h-8 w-2/3" />
        <Skeleton variant="text" className="h-4 w-1/2" />
        <div className="grid gap-4 sm:grid-cols-2">
          <Skeleton variant="card" className="h-20" />
          <Skeleton variant="card" className="h-20" />
        </div>
        <div className="grid gap-6 lg:grid-cols-3">
          <Skeleton variant="card" className="lg:col-span-2 h-64" />
          <Skeleton variant="card" className="h-64" />
        </div>
      </div>
    )
  }

  if (error || !market) {
    return (
      <EmptyState
        icon={
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round">
            <circle cx="12" cy="12" r="10" />
            <path d="M12 8v4m0 4h.01" />
          </svg>
        }
        title="Market not found"
        description="This market may have been removed or doesn't exist."
      />
    )
  }

  const outcomes = market.outcomes ?? []
  const activeOutcomeId = selectedOutcomeId ?? outcomes[0]?.id
  const activeOutcome = outcomes.find((o) => o.id === activeOutcomeId)

  return (
    <div>
      {/* Header */}
      <div className="mb-8">
        <div className="flex items-start justify-between gap-4 mb-3">
          <h1 className="text-2xl font-extrabold tracking-tight text-neutral-900">
            {market.title}
          </h1>
          <Badge variant={statusVariant[market.status] || 'neutral'} size="md">
            {market.status}
          </Badge>
        </div>

        <p className="text-neutral-500 mb-4">{market.description}</p>

        <div className="flex flex-wrap items-center gap-3">
          <Badge variant="neutral" size="md">{market.category}</Badge>
          <span className="text-sm text-neutral-400">|</span>
          <span className="text-sm text-neutral-500 tabular-nums">
            Vol: {Number(market.volume || 0).toLocaleString()} U
          </span>
          <span className="text-sm text-neutral-400">|</span>
          <span className="text-sm text-neutral-500 tabular-nums">
            Liquidity: {Number(market.liquidity || 0).toLocaleString()} U
          </span>
        </div>
      </div>

      {/* Outcomes */}
      <div className="mb-8">
        <h2 className="text-sm font-bold uppercase tracking-wider text-neutral-500 mb-3">Outcomes</h2>
        <div className="grid gap-3 sm:grid-cols-2">
          {outcomes.map((outcome) => {
            const isSelected = outcome.id === activeOutcomeId
            const pct = Math.round(Number(outcome.price) * 100)
            return (
              <button
                key={outcome.id}
                type="button"
                onClick={() => setSelectedOutcomeId(outcome.id)}
                className={`relative flex items-center justify-between p-4 bg-card rounded-xl border text-left cursor-pointer transition-all duration-200 overflow-hidden ${
                  isSelected
                    ? 'ring-2 ring-primary-500 border-primary-400 shadow-sm'
                    : 'border-card-border hover:border-neutral-300 hover:shadow-sm'
                }`}
              >
                {/* Background fill */}
                <div
                  className="absolute inset-0 opacity-5 gradient-primary"
                  style={{ width: `${pct}%` }}
                />
                <span className="relative font-bold text-neutral-900">
                  {outcome.label}
                </span>
                <span className="relative text-xl font-extrabold gradient-text tabular-nums">
                  {pct}¢
                </span>
              </button>
            )
          })}
        </div>
      </div>

      {/* Mint button */}
      <div className="mb-8">
        <MintModal marketId={id} />
      </div>

      {/* Trading + Orderbook grid */}
      <div className="grid gap-6 lg:grid-cols-3">
        <div className="lg:col-span-2">
          <h2 className="text-sm font-bold uppercase tracking-wider text-neutral-500 mb-3">
            Order Book
            {activeOutcome && (
              <span className="ml-2 font-semibold normal-case tracking-normal text-neutral-400">
                — {activeOutcome.label}
              </span>
            )}
          </h2>
          <OrderBook marketId={id} outcomeId={activeOutcomeId} />
        </div>

        <div className="lg:sticky lg:top-24 lg:self-start">
          <h2 className="text-sm font-bold uppercase tracking-wider text-neutral-500 mb-3">
            Trade
            {activeOutcome && (
              <span className="ml-2 font-semibold normal-case tracking-normal text-neutral-400">
                — {activeOutcome.label}
              </span>
            )}
          </h2>
          <TradingPanel
            marketId={id}
            outcomeId={activeOutcomeId ?? ''}
          />
        </div>
      </div>
    </div>
  )
}
