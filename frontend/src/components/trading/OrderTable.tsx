'use client'

import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useOrders } from '@/hooks/useOrders'
import { api } from '@/lib/api'
import { Table, Thead, Tbody, Tr, Th, Td } from '@/components/ui/Table'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { Skeleton } from '@/components/ui/Skeleton'
import { EmptyState } from '@/components/ui/EmptyState'

export function OrderTable() {
  const { data: orders, isLoading } = useOrders({ status: 'open' })
  const queryClient = useQueryClient()
  const [confirmingId, setConfirmingId] = useState<string | null>(null)
  const [cancelledIds, setCancelledIds] = useState<Set<string>>(new Set())

  const cancelMutation = useMutation({
    mutationFn: (orderId: string) => api.cancelOrder(orderId),
    onSuccess: (_data, orderId) => {
      setCancelledIds((prev) => new Set(prev).add(orderId))
      setConfirmingId(null)
      queryClient.invalidateQueries({ queryKey: ['orders'] })
    },
  })

  if (isLoading) {
    return (
      <div className="p-4 space-y-2">
        {[...Array(3)].map((_, i) => (
          <Skeleton key={i} variant="table-row" />
        ))}
      </div>
    )
  }

  if (!orders || orders.length === 0) {
    return (
      <EmptyState
        title="No open orders"
        description="Your active orders will appear here."
      />
    )
  }

  return (
    <Table>
      <Thead>
        <Tr>
          <Th>Market</Th>
          <Th>Side</Th>
          <Th className="text-right">Price</Th>
          <Th className="text-right">Quantity</Th>
          <Th className="text-right">Filled</Th>
          <Th>Status</Th>
          <Th className="text-right">Actions</Th>
        </Tr>
      </Thead>
      <Tbody>
        {orders.map((order) => (
          <Tr key={order.id}>
            <Td className="font-semibold">{order.market_id}</Td>
            <Td>
              <Badge variant={order.side === 'buy' ? 'success' : 'danger'}>
                {order.side.toUpperCase()}
              </Badge>
            </Td>
            <Td className="text-right">{order.price}</Td>
            <Td className="text-right">{order.quantity}</Td>
            <Td className="text-right">{order.filled_quantity}</Td>
            <Td>
              <Badge variant="neutral">{order.status}</Badge>
            </Td>
            <Td className="text-right">
              {cancelledIds.has(order.id) ? (
                <span className="text-sm text-neutral-400">Cancelled</span>
              ) : confirmingId === order.id ? (
                <div className="flex items-center justify-end gap-2">
                  <Button
                    variant="danger"
                    size="sm"
                    loading={cancelMutation.isPending}
                    onClick={() => cancelMutation.mutate(order.id)}
                  >
                    Confirm
                  </Button>
                  <Button
                    variant="secondary"
                    size="sm"
                    onClick={() => setConfirmingId(null)}
                  >
                    Back
                  </Button>
                </div>
              ) : (
                <Button
                  variant="secondary"
                  size="sm"
                  onClick={() => setConfirmingId(order.id)}
                >
                  Cancel
                </Button>
              )}
            </Td>
          </Tr>
        ))}
      </Tbody>
    </Table>
  )
}
