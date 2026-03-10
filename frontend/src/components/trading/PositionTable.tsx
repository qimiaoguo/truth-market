'use client'

import { usePositions } from '@/hooks/usePositions'
import { Table, Thead, Tbody, Tr, Th, Td } from '@/components/ui/Table'
import { Skeleton } from '@/components/ui/Skeleton'
import { EmptyState } from '@/components/ui/EmptyState'

export function PositionTable() {
  const { data: positions, isLoading } = usePositions()

  if (isLoading) {
    return (
      <div className="p-4 space-y-2">
        {[...Array(3)].map((_, i) => (
          <Skeleton key={i} variant="table-row" />
        ))}
      </div>
    )
  }

  if (!positions || positions.length === 0) {
    return (
      <EmptyState
        title="No positions yet"
        description="Your market positions will appear here after you trade."
      />
    )
  }

  return (
    <Table>
      <Thead>
        <Tr>
          <Th>Market</Th>
          <Th>Outcome</Th>
          <Th className="text-right">Quantity</Th>
          <Th className="text-right">Avg Price</Th>
          <Th className="text-right">Current Value</Th>
        </Tr>
      </Thead>
      <Tbody>
        {positions.map((position) => (
          <Tr key={position.id} data-testid="position-row">
            <Td className="font-semibold">{position.market_id}</Td>
            <Td>{position.outcome_id}</Td>
            <Td className="text-right">{position.quantity}</Td>
            <Td className="text-right">{position.avg_price}</Td>
            <Td className="text-right font-semibold">
              {position.current_value ?? '-'}
            </Td>
          </Tr>
        ))}
      </Tbody>
    </Table>
  )
}
