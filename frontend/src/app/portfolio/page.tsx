'use client'

import { useEffect } from 'react'
import { useRouter } from 'next/navigation'
import { useAuthStore } from '@/stores/authStore'
import { PositionTable } from '@/components/trading/PositionTable'
import { OrderTable } from '@/components/trading/OrderTable'

export default function PortfolioPage() {
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated)
  const user = useAuthStore((s) => s.user)
  const router = useRouter()

  useEffect(() => {
    if (!isAuthenticated) {
      router.replace('/')
    }
  }, [isAuthenticated, router])

  if (!isAuthenticated) {
    return null
  }

  return (
    <div className="mx-auto max-w-6xl px-4 py-8">
      <h1 className="mb-8 text-3xl font-bold text-white">Portfolio</h1>

      {user && (
        <section className="mb-10">
          <h2 className="mb-4 text-xl font-semibold text-gray-200">Balance</h2>
          <div className="grid gap-4 sm:grid-cols-3">
            <div className="rounded-lg border border-gray-800 bg-gray-900 p-4">
              <div className="text-sm text-gray-400">Available</div>
              <div className="mt-1 text-2xl font-bold tabular-nums text-white">
                {Number(user.balance).toLocaleString()} U
              </div>
            </div>
            <div className="rounded-lg border border-gray-800 bg-gray-900 p-4">
              <div className="text-sm text-gray-400">In Open Orders</div>
              <div className="mt-1 text-2xl font-bold tabular-nums text-yellow-400">
                {Number(user.locked_balance).toLocaleString()} U
              </div>
            </div>
            <div className="rounded-lg border border-gray-800 bg-gray-900 p-4">
              <div className="text-sm text-gray-400">Total</div>
              <div className="mt-1 text-2xl font-bold tabular-nums text-white">
                {(Number(user.balance) + Number(user.locked_balance)).toLocaleString()} U
              </div>
            </div>
          </div>
        </section>
      )}

      <section className="mb-10">
        <h2 className="mb-4 text-xl font-semibold text-gray-200">Positions</h2>
        <div className="rounded-lg border border-gray-800 bg-gray-900">
          <PositionTable />
        </div>
      </section>

      <section>
        <h2 className="mb-4 text-xl font-semibold text-gray-200">
          Open Orders
        </h2>
        <div className="rounded-lg border border-gray-800 bg-gray-900">
          <OrderTable />
        </div>
      </section>
    </div>
  )
}
