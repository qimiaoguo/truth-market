'use client'

import { useState } from 'react'
import { api } from '@/lib/api'
import { useAuthStore } from '@/stores/authStore'
import { usePositions } from '@/hooks/usePositions'
import { useRefreshUser } from '@/hooks/useRefreshUser'

interface TradingPanelProps {
  marketId: string
  outcomeId: string
}

export function TradingPanel({ marketId, outcomeId }: TradingPanelProps) {
  const [side, setSide] = useState<'buy' | 'sell'>('buy')
  const [price, setPrice] = useState('')
  const [quantity, setQuantity] = useState('')
  const [message, setMessage] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)
  const user = useAuthStore((s) => s.user)
  const refreshUser = useRefreshUser()
  const { data: positions } = usePositions(marketId)
  const outcomePosition = positions?.find((p) => p.outcome_id === outcomeId)
  const availableQty = outcomePosition ? Number(outcomePosition.quantity) : 0

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!price || !quantity) return

    setSubmitting(true)
    setMessage(null)

    try {
      const res = await api.placeOrder({
        marketId,
        outcomeId,
        side,
        price,
        quantity,
      })

      if (res.ok) {
        setMessage('Order placed')
        setPrice('')
        setQuantity('')
        refreshUser()
      } else {
        setMessage(res.error || 'Failed to place order')
      }
    } catch {
      setMessage('Failed to place order')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="bg-card rounded-xl border border-card-border p-4">
      <div className="flex mb-4" role="tablist">
        <button
          role="tab"
          aria-selected={side === 'buy'}
          onClick={() => { setSide('buy'); setMessage(null) }}
          className={`flex-1 py-2 text-sm font-medium rounded-l-lg border transition-colors cursor-pointer ${
            side === 'buy'
              ? 'bg-success-600 text-white border-success-600'
              : 'bg-white text-neutral-600 border-neutral-300 hover:bg-neutral-50'
          }`}
        >
          Buy
        </button>
        <button
          role="tab"
          aria-selected={side === 'sell'}
          onClick={() => { setSide('sell'); setMessage(null) }}
          className={`flex-1 py-2 text-sm font-medium rounded-r-lg border border-l-0 transition-colors cursor-pointer ${
            side === 'sell'
              ? 'bg-danger-600 text-white border-danger-600'
              : 'bg-white text-neutral-600 border-neutral-300 hover:bg-neutral-50'
          }`}
        >
          Sell
        </button>
      </div>

      {side === 'buy' && user && (
        <div className="mb-3 flex items-center justify-between text-sm text-neutral-500">
          <span>Available</span>
          <span className="font-medium tabular-nums text-neutral-700">
            {Number(user.balance).toLocaleString()} U
          </span>
        </div>
      )}
      {side === 'sell' && (
        <div className="mb-3 flex items-center justify-between text-sm text-neutral-500">
          <span>Position</span>
          <span className="font-medium tabular-nums text-neutral-700">
            {availableQty > 0 ? `${availableQty.toLocaleString()} shares` : 'No position'}
          </span>
        </div>
      )}

      <form onSubmit={handleSubmit} className="space-y-3">
        <div>
          <label htmlFor="trade-price" className="block text-sm font-medium text-neutral-700 mb-1">
            Price
          </label>
          <input
            id="trade-price"
            type="number"
            step="0.01"
            min="0.01"
            max="0.99"
            value={price}
            onChange={(e) => setPrice(e.target.value)}
            placeholder="0.00 - 0.99"
            className="w-full px-3 py-2 text-sm border border-neutral-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent"
          />
        </div>

        <div>
          <label htmlFor="trade-quantity" className="block text-sm font-medium text-neutral-700 mb-1">
            Quantity
          </label>
          <input
            id="trade-quantity"
            type="number"
            step="1"
            min="1"
            value={quantity}
            onChange={(e) => setQuantity(e.target.value)}
            placeholder="Number of shares"
            className="w-full px-3 py-2 text-sm border border-neutral-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent"
          />
        </div>

        {price && quantity && (
          <div className="flex items-center justify-between text-xs text-neutral-500">
            <span>Est. {side === 'buy' ? 'cost' : 'proceeds'}</span>
            <span className="tabular-nums">{(Number(price) * Number(quantity)).toFixed(2)} U</span>
          </div>
        )}

        <button
          type="submit"
          disabled={submitting || !price || !quantity}
          className={`w-full py-2 text-sm font-medium text-white rounded-lg disabled:opacity-50 transition-colors cursor-pointer ${
            side === 'buy'
              ? 'bg-success-600 hover:bg-success-700'
              : 'bg-danger-600 hover:bg-danger-700'
          }`}
        >
          {submitting ? 'Placing...' : 'Place Order'}
        </button>
      </form>

      {message && (
        <div className="mt-3 text-sm text-center font-medium text-success-600">
          {message}
        </div>
      )}
    </div>
  )
}
