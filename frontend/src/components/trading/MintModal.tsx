'use client'

import { useState } from 'react'
import { api } from '@/lib/api'
import { useRefreshUser } from '@/hooks/useRefreshUser'
import { Modal } from '@/components/ui/Modal'
import { Input } from '@/components/ui/Input'
import { Button } from '@/components/ui/Button'

interface MintModalProps {
  marketId: string
}

export function MintModal({ marketId }: MintModalProps) {
  const [isOpen, setIsOpen] = useState(false)
  const [quantity, setQuantity] = useState('')
  const [message, setMessage] = useState<{ text: string; type: 'success' | 'error' } | null>(null)
  const [submitting, setSubmitting] = useState(false)
  const refreshUser = useRefreshUser()

  const handleConfirm = async () => {
    if (!quantity || Number(quantity) <= 0) return

    setSubmitting(true)
    setMessage(null)

    try {
      const res = await api.mintTokens(marketId, quantity)

      if (res.ok) {
        setMessage({ text: 'Tokens minted successfully', type: 'success' })
        setQuantity('')
        refreshUser()
      } else {
        setMessage({ text: res.error || 'Failed to mint', type: 'error' })
      }
    } catch {
      setMessage({ text: 'Failed to mint', type: 'error' })
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <>
      <Button
        variant="gradient"
        size="md"
        onClick={() => { setIsOpen(true); setMessage(null) }}
      >
        Mint Token Set
      </Button>

      <Modal
        open={isOpen}
        onClose={() => setIsOpen(false)}
        title="Mint Token Sets"
        footer={
          <>
            <Button variant="secondary" size="md" onClick={() => setIsOpen(false)}>
              Cancel
            </Button>
            <Button
              variant="primary"
              size="md"
              loading={submitting}
              disabled={!quantity || Number(quantity) <= 0}
              onClick={handleConfirm}
            >
              Confirm Mint
            </Button>
          </>
        }
      >
        <div className="space-y-4">
          <p className="text-sm text-neutral-500">
            Minting creates a complete set of outcome tokens. Cost equals the quantity in U.
          </p>
          <Input
            label="Quantity"
            type="number"
            step="1"
            min="1"
            value={quantity}
            onChange={(e) => setQuantity(e.target.value)}
            placeholder="Number of sets to mint"
          />
          {quantity && Number(quantity) > 0 && (
            <div className="flex items-center justify-between px-3 py-2 rounded-lg bg-neutral-50 border border-neutral-200">
              <span className="text-xs font-semibold text-neutral-500">Cost</span>
              <span className="text-sm font-bold tabular-nums text-neutral-800">{Number(quantity).toLocaleString()} U</span>
            </div>
          )}
          {message && (
            <div className={`text-sm text-center font-semibold px-3 py-2 rounded-lg ${
              message.type === 'success'
                ? 'bg-success-50 text-success-700'
                : 'bg-danger-50 text-danger-700'
            }`}>
              {message.text}
            </div>
          )}
        </div>
      </Modal>
    </>
  )
}
