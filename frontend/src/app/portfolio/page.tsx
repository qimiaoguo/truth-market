'use client'

import { useEffect } from 'react'
import { useRouter } from 'next/navigation'
import { useAuthStore } from '@/stores/authStore'
import { PositionTable } from '@/components/trading/PositionTable'
import { OrderTable } from '@/components/trading/OrderTable'

export default function PortfolioPage() {
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated)
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
