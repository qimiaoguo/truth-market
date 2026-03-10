'use client'

import { useRankings } from '@/hooks/useRankings'
import { Table, Thead, Tbody, Tr, Th, Td } from '@/components/ui/Table'
import { Badge } from '@/components/ui/Badge'
import { Skeleton } from '@/components/ui/Skeleton'
import { EmptyState } from '@/components/ui/EmptyState'
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

const rankStyles: Record<number, string> = {
  1: 'bg-warning-100 text-warning-700 ring-1 ring-warning-300',
  2: 'bg-neutral-200 text-neutral-700 ring-1 ring-neutral-300',
  3: 'bg-accent-100 text-accent-700 ring-1 ring-accent-300',
}

export default function RankingTable({ dimension, userType }: RankingTableProps) {
  const { data: rankings, isLoading, error } = useRankings(dimension, userType)

  if (isLoading) {
    return (
      <div className="p-4 space-y-2">
        {[...Array(5)].map((_, i) => (
          <Skeleton key={i} variant="table-row" />
        ))}
      </div>
    )
  }

  if (error) {
    return (
      <EmptyState
        title="Failed to load rankings"
        description="Something went wrong. Please try again later."
      />
    )
  }

  if (!rankings || rankings.length === 0) {
    return (
      <EmptyState
        title="No rankings available"
        description="Rankings will appear once users start trading."
      />
    )
  }

  return (
    <Table>
      <Thead>
        <Tr>
          <Th className="w-16">Rank</Th>
          <Th>Wallet Address</Th>
          <Th className="w-24">Type</Th>
          <Th className="text-right w-40">{dimensionLabels[dimension]}</Th>
        </Tr>
      </Thead>
      <Tbody>
        {rankings.map((ranking) => (
          <Tr
            key={`${ranking.user_id}-${ranking.dimension}`}
            data-testid="ranking-row"
          >
            <Td>
              <span
                className={`inline-flex items-center justify-center w-8 h-8 rounded-full text-sm font-bold ${
                  rankStyles[ranking.rank] || 'bg-neutral-100 text-neutral-500'
                }`}
              >
                {ranking.rank}
              </span>
            </Td>
            <Td className="font-mono text-sm">
              {truncateAddress(ranking.wallet_address)}
            </Td>
            <Td>
              <Badge variant={ranking.user_type === 'agent' ? 'accent' : 'success'}>
                {ranking.user_type === 'agent' ? 'Agent' : 'Human'}
              </Badge>
            </Td>
            <Td className="text-right font-bold">
              {formatValue(dimension, ranking.value)}
            </Td>
          </Tr>
        ))}
      </Tbody>
    </Table>
  )
}
