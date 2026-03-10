'use client'

import Link from 'next/link'
import type { Market } from '@/lib/types'
import { Badge } from '@/components/ui/Badge'

const statusVariant: Record<string, 'success' | 'neutral' | 'primary' | 'danger'> = {
  open: 'success',
  closed: 'neutral',
  resolved: 'primary',
  cancelled: 'danger',
  draft: 'neutral',
}

export function MarketCard({ market }: { market: Market }) {
  const outcomes = market.outcomes ?? []

  return (
    <Link
      href={`/market/${market.id}`}
      data-testid="market-card"
      className="group block bg-card rounded-xl border border-card-border overflow-hidden
        transition-all duration-300 ease-out
        hover:shadow-lg hover:-translate-y-1 hover:border-primary-200"
    >
      {/* Gradient top accent */}
      <div className="h-1 w-full gradient-primary opacity-60 group-hover:opacity-100 transition-opacity" />

      <div className="p-5">
        <div className="flex items-start justify-between gap-3 mb-3">
          <h3 className="text-base font-bold text-neutral-900 leading-tight group-hover:text-primary-700 transition-colors">
            {market.title}
          </h3>
          <Badge variant={statusVariant[market.status] || 'neutral'}>
            {market.status}
          </Badge>
        </div>

        <div className="mb-4">
          <Badge variant="neutral" size="sm">
            {market.category}
          </Badge>
        </div>

        {outcomes.length > 0 && (
          <div className="space-y-2 mb-4">
            {outcomes.map((outcome) => {
              const pct = Math.round(Number(outcome.price) * 100)
              return (
                <div key={outcome.id} className="flex items-center gap-3">
                  <span className="text-xs font-medium text-neutral-500 w-10 shrink-0">
                    {outcome.label}
                  </span>
                  <div className="flex-1 h-2 bg-neutral-100 rounded-full overflow-hidden">
                    <div
                      className="h-full rounded-full gradient-primary transition-all duration-500"
                      style={{ width: `${pct}%` }}
                    />
                  </div>
                  <span className="text-sm font-bold text-neutral-800 tabular-nums w-10 text-right">
                    {pct}¢
                  </span>
                </div>
              )
            })}
          </div>
        )}

        <div className="flex items-center gap-2 text-xs text-neutral-400">
          <svg className="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round">
            <path d="M12 2v20M17 5H9.5a3.5 3.5 0 000 7h5a3.5 3.5 0 010 7H6" />
          </svg>
          <span className="font-medium">Vol: {Number(market.volume || 0).toLocaleString()} U</span>
        </div>
      </div>
    </Link>
  )
}
