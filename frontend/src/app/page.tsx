'use client'

import { useMarkets } from '@/hooks/useMarkets'
import { MarketCard } from '@/components/market/MarketCard'

export default function Home() {
  const { data: markets, isLoading, error } = useMarkets({ status: 'open' })

  return (
    <div>
      <h1 className="text-2xl font-bold text-neutral-900 mb-6">Markets</h1>

      {isLoading && (
        <div className="flex items-center justify-center py-16">
          <div className="text-neutral-500">Loading markets...</div>
        </div>
      )}

      {error && (
        <div className="flex items-center justify-center py-16">
          <div className="text-danger-600">Failed to load markets</div>
        </div>
      )}

      {!isLoading && !error && markets && markets.length === 0 && (
        <div className="flex items-center justify-center py-16">
          <div className="text-neutral-500">No markets available</div>
        </div>
      )}

      {markets && markets.length > 0 && (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {markets.map((market) => (
            <MarketCard key={market.id} market={market} />
          ))}
        </div>
      )}
    </div>
  )
}
