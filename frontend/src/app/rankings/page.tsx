'use client'

import { useState } from 'react'
import RankingTable from '@/components/ranking/RankingTable'
import { PageHeader } from '@/components/ui/PageHeader'
import { Tabs } from '@/components/ui/Tabs'
import { Card } from '@/components/ui/Card'
import type { RankDimension } from '@/lib/types'

const dimensions: { key: string; label: string }[] = [
  { key: 'total_assets', label: 'Total Assets' },
  { key: 'pnl', label: 'PnL' },
  { key: 'volume', label: 'Volume' },
  { key: 'win_rate', label: 'Win Rate' },
  { key: 'trade_count', label: 'Trade Count' },
]

const userTypes: { key: string | undefined; label: string }[] = [
  { key: undefined, label: 'All' },
  { key: 'human', label: 'Humans' },
  { key: 'agent', label: 'Agents' },
]

export default function RankingsPage() {
  const [selectedDimension, setSelectedDimension] = useState<RankDimension>('total_assets')
  const [selectedUserType, setSelectedUserType] = useState<string | undefined>(undefined)

  return (
    <div>
      <PageHeader title="Leaderboard" gradient />

      {/* Dimension Tabs */}
      <Tabs
        tabs={dimensions}
        activeKey={selectedDimension}
        onChange={(key) => setSelectedDimension(key as RankDimension)}
        className="mb-6"
      />

      {/* User Type Filter */}
      <div className="mb-6 flex items-center gap-2">
        <span className="text-sm font-semibold text-neutral-500 mr-2">Filter:</span>
        {userTypes.map((ut) => (
          <button
            key={ut.label}
            onClick={() => setSelectedUserType(ut.key)}
            className={`px-3.5 py-1.5 text-sm rounded-full font-bold transition-all duration-200 cursor-pointer ${
              selectedUserType === ut.key
                ? 'gradient-primary text-white shadow-sm'
                : 'bg-neutral-100 text-neutral-600 hover:bg-neutral-200'
            }`}
          >
            {ut.label}
          </button>
        ))}
      </div>

      {/* Rankings Table */}
      <Card hover={false}>
        <RankingTable dimension={selectedDimension} userType={selectedUserType} />
      </Card>
    </div>
  )
}
