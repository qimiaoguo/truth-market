'use client'

import { usePositions } from '@/hooks/usePositions'

export function PositionTable() {
  const { data: positions, isLoading } = usePositions()

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-12 text-gray-400">
        Loading positions...
      </div>
    )
  }

  if (!positions || positions.length === 0) {
    return (
      <div className="flex items-center justify-center py-12 text-gray-500">
        No positions yet
      </div>
    )
  }

  return (
    <div className="overflow-x-auto">
      <table className="w-full text-left text-sm">
        <thead>
          <tr className="border-b border-gray-700 text-gray-400">
            <th className="px-4 py-3 font-medium">Market</th>
            <th className="px-4 py-3 font-medium">Outcome</th>
            <th className="px-4 py-3 font-medium text-right">Quantity</th>
            <th className="px-4 py-3 font-medium text-right">Avg Price</th>
            <th className="px-4 py-3 font-medium text-right">Current Value</th>
          </tr>
        </thead>
        <tbody>
          {positions.map((position) => (
            <tr
              key={position.id}
              data-testid="position-row"
              className="border-b border-gray-800 text-gray-200 hover:bg-gray-800/50 transition-colors"
            >
              <td className="px-4 py-3 font-medium">{position.marketId}</td>
              <td className="px-4 py-3">{position.outcomeId}</td>
              <td className="px-4 py-3 text-right tabular-nums">
                {position.quantity}
              </td>
              <td className="px-4 py-3 text-right tabular-nums">
                {position.avgPrice}
              </td>
              <td className="px-4 py-3 text-right tabular-nums">
                {position.currentValue ?? '-'}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
