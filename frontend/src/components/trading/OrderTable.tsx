'use client'

import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useOrders } from '@/hooks/useOrders'
import { api } from '@/lib/api'

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
      <div className="flex items-center justify-center py-12 text-gray-400">
        Loading orders...
      </div>
    )
  }

  if (!orders || orders.length === 0) {
    return (
      <div className="flex items-center justify-center py-12 text-gray-500">
        No open orders
      </div>
    )
  }

  return (
    <div className="overflow-x-auto">
      <table className="w-full text-left text-sm">
        <thead>
          <tr className="border-b border-gray-700 text-gray-400">
            <th className="px-4 py-3 font-medium">Market</th>
            <th className="px-4 py-3 font-medium">Side</th>
            <th className="px-4 py-3 font-medium text-right">Price</th>
            <th className="px-4 py-3 font-medium text-right">Quantity</th>
            <th className="px-4 py-3 font-medium text-right">Filled</th>
            <th className="px-4 py-3 font-medium">Status</th>
            <th className="px-4 py-3 font-medium text-right">Actions</th>
          </tr>
        </thead>
        <tbody>
          {orders.map((order) => (
            <tr
              key={order.id}
              className="border-b border-gray-800 text-gray-200 hover:bg-gray-800/50 transition-colors"
            >
              <td className="px-4 py-3 font-medium">{order.market_id}</td>
              <td className="px-4 py-3">
                <span
                  className={
                    order.side === 'buy'
                      ? 'text-green-400 font-medium'
                      : 'text-red-400 font-medium'
                  }
                >
                  {order.side.toUpperCase()}
                </span>
              </td>
              <td className="px-4 py-3 text-right tabular-nums">
                {order.price}
              </td>
              <td className="px-4 py-3 text-right tabular-nums">
                {order.quantity}
              </td>
              <td className="px-4 py-3 text-right tabular-nums">
                {order.filled_quantity}
              </td>
              <td className="px-4 py-3">
                <span className="rounded-full bg-gray-700 px-2 py-0.5 text-xs text-gray-300">
                  {order.status}
                </span>
              </td>
              <td className="px-4 py-3 text-right">
                {cancelledIds.has(order.id) ? (
                  <span className="text-sm text-gray-400">Cancelled</span>
                ) : confirmingId === order.id ? (
                  <div className="flex items-center justify-end gap-2">
                    <button
                      onClick={() => cancelMutation.mutate(order.id)}
                      disabled={cancelMutation.isPending}
                      className="rounded bg-red-600 px-3 py-1 text-xs font-medium text-white hover:bg-red-700 disabled:opacity-50 transition-colors"
                    >
                      {cancelMutation.isPending ? 'Cancelling...' : 'Confirm'}
                    </button>
                    <button
                      onClick={() => setConfirmingId(null)}
                      className="rounded bg-gray-700 px-3 py-1 text-xs font-medium text-gray-300 hover:bg-gray-600 transition-colors"
                    >
                      Back
                    </button>
                  </div>
                ) : (
                  <button
                    onClick={() => setConfirmingId(order.id)}
                    className="rounded bg-gray-700 px-3 py-1 text-xs font-medium text-gray-300 hover:bg-gray-600 transition-colors"
                  >
                    Cancel
                  </button>
                )}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
