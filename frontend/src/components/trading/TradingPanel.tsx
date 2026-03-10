'use client'

import { useState } from 'react'
import { api } from '@/lib/api'
import { useAuthStore } from '@/stores/authStore'
import { usePositions } from '@/hooks/usePositions'
import { useRefreshUser } from '@/hooks/useRefreshUser'
import { Card } from '@/components/ui/Card'
import { Input } from '@/components/ui/Input'
import { Button } from '@/components/ui/Button'

interface TradingPanelProps {
  marketId: string
  outcomeId: string
}

export function TradingPanel({ marketId, outcomeId }: TradingPanelProps) {
  const [side, setSide] = useState<'buy' | 'sell'>('buy')
  const [price, setPrice] = useState('')
  const [quantity, setQuantity] = useState('')
  const [message, setMessage] = useState<{ text: string; type: 'success' | 'error' } | null>(null)
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
        setMessage({ text: 'Order placed successfully', type: 'success' })
        setPrice('')
        setQuantity('')
        refreshUser()
      } else {
        setMessage({ text: res.error || 'Failed to place order', type: 'error' })
      }
    } catch {
      setMessage({ text: 'Failed to place order', type: 'error' })
    } finally {
      setSubmitting(false)
    }
  }

  const estTotal = price && quantity ? (Number(price) * Number(quantity)).toFixed(2) : null

  return (
    <Card hover={false}>
      <div className="p-5">
        {/* Buy/Sell Toggle */}
        <div className="flex mb-5" role="tablist">
          <button
            role="tab"
            aria-selected={side === 'buy'}
            onClick={() => { setSide('buy'); setMessage(null) }}
            className={`flex-1 py-2.5 text-sm font-bold rounded-l-lg border transition-all duration-200 cursor-pointer ${
              side === 'buy'
                ? 'bg-success-600 text-white border-success-600 shadow-sm'
                : 'bg-white text-neutral-500 border-neutral-200 hover:bg-neutral-50'
            }`}
          >
            Buy
          </button>
          <button
            role="tab"
            aria-selected={side === 'sell'}
            onClick={() => { setSide('sell'); setMessage(null) }}
            className={`flex-1 py-2.5 text-sm font-bold rounded-r-lg border border-l-0 transition-all duration-200 cursor-pointer ${
              side === 'sell'
                ? 'bg-danger-600 text-white border-danger-600 shadow-sm'
                : 'bg-white text-neutral-500 border-neutral-200 hover:bg-neutral-50'
            }`}
          >
            Sell
          </button>
        </div>

        {/* Balance/Position Info */}
        {side === 'buy' && user && (
          <div className="mb-4 flex items-center justify-between text-sm">
            <span className="text-neutral-500">Available</span>
            <span className="font-bold tabular-nums text-neutral-700">
              {Number(user.balance).toLocaleString()} U
            </span>
          </div>
        )}
        {side === 'sell' && (
          <div className="mb-4 flex items-center justify-between text-sm">
            <span className="text-neutral-500">Position</span>
            <span className="font-bold tabular-nums text-neutral-700">
              {availableQty > 0 ? `${availableQty.toLocaleString()} shares` : 'No position'}
            </span>
          </div>
        )}

        <form onSubmit={handleSubmit} className="space-y-4">
          <Input
            label="Price"
            type="number"
            step="0.01"
            min="0.01"
            max="0.99"
            value={price}
            onChange={(e) => setPrice(e.target.value)}
            placeholder="0.01 - 0.99"
          />

          <Input
            label="Quantity"
            type="number"
            step="1"
            min="1"
            value={quantity}
            onChange={(e) => setQuantity(e.target.value)}
            placeholder="Number of shares"
          />

          {estTotal && (
            <div className="flex items-center justify-between px-3 py-2 rounded-lg bg-neutral-50 border border-neutral-200">
              <span className="text-xs font-semibold text-neutral-500">
                Est. {side === 'buy' ? 'cost' : 'proceeds'}
              </span>
              <span className="text-sm font-bold tabular-nums text-neutral-800">{estTotal} U</span>
            </div>
          )}

          <Button
            type="submit"
            variant={side === 'buy' ? 'success' : 'danger'}
            size="md"
            loading={submitting}
            disabled={!price || !quantity}
            className="w-full"
          >
            Place {side === 'buy' ? 'Buy' : 'Sell'} Order
          </Button>
        </form>

        {message && (
          <div className={`mt-4 text-sm text-center font-semibold px-3 py-2 rounded-lg ${
            message.type === 'success'
              ? 'bg-success-50 text-success-700'
              : 'bg-danger-50 text-danger-700'
          }`}>
            {message.text}
          </div>
        )}
      </div>
    </Card>
  )
}
