'use client'

import { useState } from 'react'
import RankingTable from '@/components/ranking/RankingTable'
import type { RankDimension } from '@/lib/types'

const dimensions: { key: RankDimension; label: string }[] = [
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
    <div className="min-h-screen bg-surface">
      <div className="max-w-5xl mx-auto px-4 py-8">
        <h1 className="text-3xl font-bold text-neutral-900 mb-8">Leaderboard</h1>

        {/* Dimension Tabs */}
        <div className="mb-6" role="tablist" aria-label="Ranking dimensions">
          <div className="flex border-b border-neutral-200">
            {dimensions.map((dim) => (
              <button
                key={dim.key}
                role="tab"
                aria-selected={selectedDimension === dim.key}
                onClick={() => setSelectedDimension(dim.key)}
                className={`px-4 py-2.5 text-sm font-medium transition-colors relative ${
                  selectedDimension === dim.key
                    ? 'text-primary-600 after:absolute after:bottom-0 after:left-0 after:right-0 after:h-0.5 after:bg-primary-600'
                    : 'text-neutral-500 hover:text-neutral-700'
                }`}
              >
                {dim.label}
              </button>
            ))}
          </div>
        </div>

        {/* User Type Filter */}
        <div className="mb-6 flex items-center gap-2">
          <span className="text-sm text-neutral-500 mr-2">Filter:</span>
          {userTypes.map((ut) => (
            <button
              key={ut.label}
              role="button"
              onClick={() => setSelectedUserType(ut.key)}
              className={`px-3.5 py-1.5 text-sm rounded-full font-medium transition-colors ${
                selectedUserType === ut.key
                  ? 'bg-primary-600 text-white'
                  : 'bg-neutral-100 text-neutral-600 hover:bg-neutral-200'
              }`}
            >
              {ut.label}
            </button>
          ))}
        </div>

        {/* Rankings Table */}
        <div className="bg-card rounded-xl border border-card-border shadow-sm">
          <RankingTable dimension={selectedDimension} userType={selectedUserType} />
        </div>
      </div>
    </div>
  )
}
