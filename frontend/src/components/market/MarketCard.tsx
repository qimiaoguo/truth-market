'use client'

import Link from 'next/link'
import type { Market } from '@/lib/types'

const statusColors: Record<string, string> = {
  open: 'bg-success-100 text-success-700',
  closed: 'bg-neutral-100 text-neutral-600',
  resolved: 'bg-primary-100 text-primary-700',
  cancelled: 'bg-danger-100 text-danger-700',
  draft: 'bg-neutral-100 text-neutral-500',
}

export function MarketCard({ market }: { market: Market }) {
  const outcomes = market.outcomes ?? []

  return (
    <Link
      href={`/market/${market.id}`}
      data-testid="market-card"
      className="block p-5 bg-card rounded-xl border border-card-border hover:bg-card-hover hover:shadow-sm transition-all"
    >
      <div className="flex items-start justify-between gap-3 mb-3">
        <h3 className="text-base font-semibold text-neutral-900 leading-tight">
          {market.title}
        </h3>
        <span
          className={`shrink-0 px-2 py-0.5 text-xs font-medium rounded-full ${
            statusColors[market.status] || 'bg-neutral-100 text-neutral-600'
          }`}
        >
          {market.status}
        </span>
      </div>

      <div className="flex items-center gap-2 mb-3">
        <span className="text-xs font-medium text-neutral-500 bg-neutral-100 px-2 py-0.5 rounded">
          {market.category}
        </span>
      </div>

      {outcomes.length > 0 && (
        <div className="flex items-center gap-3 mb-3">
          {outcomes.map((outcome) => (
            <span key={outcome.id} className="text-sm">
              <span className="text-neutral-500">{outcome.label}</span>{' '}
              <span className="font-semibold text-neutral-800">
                {Math.round(Number(outcome.price) * 100)}¢
              </span>
            </span>
          ))}
        </div>
      )}

      <div className="text-xs text-neutral-400">
        Vol: {Number(market.volume || 0).toLocaleString()} U
      </div>
    </Link>
  )
}
