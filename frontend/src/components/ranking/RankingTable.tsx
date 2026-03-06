'use client'

import { useRankings } from '@/hooks/useRankings'
import type { RankDimension } from '@/lib/types'

interface RankingTableProps {
  dimension: RankDimension
  userType?: string
}

const dimensionLabels: Record<RankDimension, string> = {
  total_assets: 'Total Assets',
  pnl: 'PnL',
  volume: 'Volume',
  win_rate: 'Win Rate',
  trade_count: 'Trade Count',
}

function formatValue(dimension: RankDimension, value: string): string {
  if (dimension === 'win_rate') {
    const pct = parseFloat(value) * 100
    return `${pct.toFixed(1)}%`
  }
  if (dimension === 'trade_count') {
    return value
  }
  const num = parseFloat(value)
  if (num >= 1_000_000) return `$${(num / 1_000_000).toFixed(2)}M`
  if (num >= 1_000) return `$${(num / 1_000).toFixed(2)}K`
  return `$${num.toFixed(2)}`
}

function truncateAddress(address: string): string {
  if (address.length <= 12) return address
  return `${address.slice(0, 6)}...${address.slice(-4)}`
}

export default function RankingTable({ dimension, userType }: RankingTableProps) {
  const { data: rankings, isLoading, error } = useRankings(dimension, userType)

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-16">
        <div className="text-neutral-500">Loading rankings...</div>
      </div>
    )
  }

  if (error) {
    return (
      <div className="flex items-center justify-center py-16">
        <div className="text-danger-500">Failed to load rankings.</div>
      </div>
    )
  }

  if (!rankings || rankings.length === 0) {
    return (
      <div className="flex items-center justify-center py-16">
        <div className="text-neutral-500">No rankings available yet.</div>
      </div>
    )
  }

  return (
    <div className="overflow-x-auto">
      <table className="w-full border-collapse">
        <thead>
          <tr className="border-b border-neutral-200 text-left text-sm font-medium text-neutral-500">
            <th className="py-3 pr-4 pl-4 w-16">Rank</th>
            <th className="py-3 pr-4">Wallet Address</th>
            <th className="py-3 pr-4 w-24">Type</th>
            <th className="py-3 pr-4 pl-4 text-right w-40">{dimensionLabels[dimension]}</th>
          </tr>
        </thead>
        <tbody>
          {rankings.map((ranking) => (
            <tr
              key={`${ranking.userId}-${ranking.dimension}`}
              data-testid="ranking-row"
              className="border-b border-neutral-100 hover:bg-neutral-50 transition-colors"
            >
              <td className="py-3 pr-4 pl-4">
                <span
                  className={`inline-flex items-center justify-center w-8 h-8 rounded-full text-sm font-semibold ${
                    ranking.rank === 1
                      ? 'bg-yellow-100 text-yellow-800'
                      : ranking.rank === 2
                        ? 'bg-neutral-200 text-neutral-700'
                        : ranking.rank === 3
                          ? 'bg-orange-100 text-orange-800'
                          : 'bg-neutral-100 text-neutral-600'
                  }`}
                >
                  {ranking.rank}
                </span>
              </td>
              <td className="py-3 pr-4 font-mono text-sm">
                {truncateAddress(ranking.walletAddress)}
              </td>
              <td className="py-3 pr-4">
                <span
                  className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${
                    ranking.userType === 'agent'
                      ? 'bg-primary-100 text-primary-700'
                      : 'bg-success-100 text-success-700'
                  }`}
                >
                  {ranking.userType === 'agent' ? 'Agent' : 'Human'}
                </span>
              </td>
              <td className="py-3 pr-4 pl-4 text-right font-medium tabular-nums">
                {formatValue(dimension, ranking.value)}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
