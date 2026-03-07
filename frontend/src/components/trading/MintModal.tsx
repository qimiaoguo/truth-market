'use client'

import { useState } from 'react'
import { api } from '@/lib/api'
import { useRefreshUser } from '@/hooks/useRefreshUser'

interface MintModalProps {
  marketId: string
}

export function MintModal({ marketId }: MintModalProps) {
  const [isOpen, setIsOpen] = useState(false)
  const [quantity, setQuantity] = useState('')
  const [message, setMessage] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)
  const refreshUser = useRefreshUser()

  const handleConfirm = async () => {
    if (!quantity || Number(quantity) <= 0) return

    setSubmitting(true)
    setMessage(null)

    try {
      const res = await api.mintTokens(marketId, quantity)

      if (res.ok) {
        setMessage('Minted')
        setQuantity('')
        refreshUser()
      } else {
        setMessage(res.error || 'Failed to mint')
      }
    } catch {
      setMessage('Failed to mint')
    } finally {
      setSubmitting(false)
    }
  }

  if (!isOpen) {
    return (
      <button
        onClick={() => { setIsOpen(true); setMessage(null) }}
        className="px-4 py-2 text-sm font-medium text-white bg-primary-600 rounded-lg hover:bg-primary-700 transition-colors cursor-pointer"
      >
        Mint
      </button>
    )
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
      <div className="bg-white rounded-xl shadow-xl p-6 w-full max-w-sm mx-4">
        <h2 className="text-lg font-semibold text-neutral-900 mb-4">
          Mint Token Sets
        </h2>

        <div className="mb-4">
          <label htmlFor="mint-quantity" className="block text-sm font-medium text-neutral-700 mb-1">
            Quantity
          </label>
          <input
            id="mint-quantity"
            type="number"
            step="1"
            min="1"
            value={quantity}
            onChange={(e) => setQuantity(e.target.value)}
            placeholder="Number of sets to mint"
            className="w-full px-3 py-2 text-sm border border-neutral-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent"
          />
        </div>

        <div className="flex gap-2">
          <button
            onClick={() => { setIsOpen(false); setMessage(null) }}
            className="flex-1 py-2 text-sm font-medium text-neutral-600 border border-neutral-300 rounded-lg hover:bg-neutral-50 transition-colors cursor-pointer"
          >
            Cancel
          </button>
          <button
            onClick={handleConfirm}
            disabled={submitting || !quantity || Number(quantity) <= 0}
            className="flex-1 py-2 text-sm font-medium text-white bg-primary-600 rounded-lg hover:bg-primary-700 disabled:opacity-50 transition-colors cursor-pointer"
          >
            {submitting ? 'Minting...' : 'Confirm'}
          </button>
        </div>

        {message && (
          <div className="mt-3 text-sm text-center font-medium text-success-600">
            {message}
          </div>
        )}
      </div>
    </div>
  )
}
