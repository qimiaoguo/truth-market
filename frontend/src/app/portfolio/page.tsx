'use client'

import { useEffect } from 'react'
import { useRouter } from 'next/navigation'
import { useAuthStore } from '@/stores/authStore'
import { PositionTable } from '@/components/trading/PositionTable'
import { OrderTable } from '@/components/trading/OrderTable'
import { PageHeader } from '@/components/ui/PageHeader'
import { Card } from '@/components/ui/Card'

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
    <div>
      <PageHeader title="Portfolio" gradient />

      {user && (
        <section className="mb-10">
          <h2 className="text-sm font-bold uppercase tracking-wider text-neutral-500 mb-4">Balance</h2>
          <div className="grid gap-4 sm:grid-cols-3">
            <Card gradient>
              <div className="p-5">
                <div className="text-xs font-semibold text-neutral-500 uppercase tracking-wide">Available</div>
                <div className="mt-2 text-2xl font-extrabold tabular-nums text-neutral-900">
                  {Number(user.balance).toLocaleString()} U
                </div>
              </div>
            </Card>
            <Card>
              <div className="p-5">
                <div className="text-xs font-semibold text-neutral-500 uppercase tracking-wide">In Open Orders</div>
                <div className="mt-2 text-2xl font-extrabold tabular-nums text-warning-600">
                  {Number(user.locked_balance).toLocaleString()} U
                </div>
              </div>
            </Card>
            <Card>
              <div className="p-5">
                <div className="text-xs font-semibold text-neutral-500 uppercase tracking-wide">Total</div>
                <div className="mt-2 text-2xl font-extrabold tabular-nums text-neutral-900">
                  {(Number(user.balance) + Number(user.locked_balance)).toLocaleString()} U
                </div>
              </div>
            </Card>
          </div>
        </section>
      )}

      <section className="mb-10">
        <h2 className="text-sm font-bold uppercase tracking-wider text-neutral-500 mb-4">Positions</h2>
        <Card hover={false}>
          <PositionTable />
        </Card>
      </section>

      <section>
        <h2 className="text-sm font-bold uppercase tracking-wider text-neutral-500 mb-4">Open Orders</h2>
        <Card hover={false}>
          <OrderTable />
        </Card>
      </section>
    </div>
  )
}
