'use client'

import { useMarkets } from '@/hooks/useMarkets'
import { MarketCard } from '@/components/market/MarketCard'
import { PageHeader } from '@/components/ui/PageHeader'
import { Skeleton } from '@/components/ui/Skeleton'
import { EmptyState } from '@/components/ui/EmptyState'

export default function Home() {
  const { data: markets, isLoading, error } = useMarkets({ status: 'open' })

  return (
    <div>
      <PageHeader
        title="Markets"
        subtitle="Trade on the outcomes you believe in"
        gradient
      />

      {isLoading && (
        <div className="grid gap-5 sm:grid-cols-2 lg:grid-cols-3">
          {[...Array(6)].map((_, i) => (
            <Skeleton key={i} variant="card" className="h-48" />
          ))}
        </div>
      )}

      {error && (
        <EmptyState
          icon={
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round">
              <circle cx="12" cy="12" r="10" />
              <path d="M12 8v4m0 4h.01" />
            </svg>
          }
          title="Failed to load markets"
          description="Something went wrong. Please try again later."
        />
      )}

      {!isLoading && !error && markets && markets.length === 0 && (
        <EmptyState
          icon={
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round">
              <path d="M3 3l18 18M10.5 10.677a2 2 0 002.823 2.823" />
              <path d="M7.362 7.561C5.68 8.74 4.279 10.42 3 12c1.889 2.991 5.282 6 9 6 1.55 0 3.043-.523 4.395-1.35M12 6c3.718 0 7.111 3.009 9 6-.947 1.496-2.153 2.932-3.547 4.03" />
            </svg>
          }
          title="No markets available"
          description="Check back later for new prediction markets."
        />
      )}

      {markets && markets.length > 0 && (
        <div className="grid gap-5 sm:grid-cols-2 lg:grid-cols-3">
          {markets.map((market) => (
            <MarketCard key={market.id} market={market} />
          ))}
        </div>
      )}
    </div>
  )
}
