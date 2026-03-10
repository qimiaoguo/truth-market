'use client'

import { useState, useEffect } from 'react'
import { useRouter } from 'next/navigation'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/lib/api'
import { useAuthStore } from '@/stores/authStore'
import type { Market } from '@/lib/types'
import { PageHeader } from '@/components/ui/PageHeader'
import { Card } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { Input } from '@/components/ui/Input'
import { Textarea } from '@/components/ui/Textarea'
import { Badge } from '@/components/ui/Badge'
import { Skeleton } from '@/components/ui/Skeleton'
import { EmptyState } from '@/components/ui/EmptyState'

export default function AdminPage() {
  const router = useRouter()
  const queryClient = useQueryClient()
  const user = useAuthStore((s) => s.user)
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated)

  useEffect(() => {
    if (!isAuthenticated || !user?.is_admin) {
      router.push('/')
    }
  }, [isAuthenticated, user, router])

  const [showCreateForm, setShowCreateForm] = useState(false)
  const [title, setTitle] = useState('')
  const [description, setDescription] = useState('')
  const [category, setCategory] = useState('')
  const [isBinary, setIsBinary] = useState(true)
  const [closesAt, setClosesAt] = useState('')
  const [createSuccess, setCreateSuccess] = useState(false)

  const [selectedMarket, setSelectedMarket] = useState<Market | null>(null)
  const [showResolveFlow, setShowResolveFlow] = useState(false)
  const [selectedOutcomeId, setSelectedOutcomeId] = useState<string>('')
  const [resolveSuccess, setResolveSuccess] = useState(false)

  const { data: draftMarkets, isLoading: draftLoading } = useQuery({
    queryKey: ['markets', 'draft'],
    queryFn: async () => {
      const res = await api.listMarkets({ status: 'draft' })
      return res.data?.markets ?? []
    },
    enabled: !!user?.is_admin,
  })

  const { data: openMarkets, isLoading: openLoading } = useQuery({
    queryKey: ['markets', 'open'],
    queryFn: async () => {
      const res = await api.listMarkets({ status: 'open' })
      return res.data?.markets ?? []
    },
    enabled: !!user?.is_admin,
  })

  const { data: closedMarkets, isLoading: marketsLoading } = useQuery({
    queryKey: ['markets', 'closed'],
    queryFn: async () => {
      const res = await api.listMarkets({ status: 'closed' })
      return res.data?.markets ?? []
    },
    enabled: !!user?.is_admin,
  })

  const createMarketMutation = useMutation({
    mutationFn: async () => {
      const outcomes = isBinary ? ['Yes', 'No'] : ['Yes', 'No']
      return api.createMarket({
        title,
        description,
        category,
        market_type: isBinary ? 'binary' : 'multi',
        outcomes,
        closes_at: closesAt,
      })
    },
    onSuccess: () => {
      setCreateSuccess(true)
      setTitle('')
      setDescription('')
      setCategory('')
      setIsBinary(true)
      setClosesAt('')
      setShowCreateForm(false)
      queryClient.invalidateQueries({ queryKey: ['markets'] })
      setTimeout(() => setCreateSuccess(false), 5000)
    },
  })

  const updateStatusMutation = useMutation({
    mutationFn: async ({ id, status }: { id: string; status: string }) => {
      return api.updateMarketStatus(id, status)
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['markets'] })
    },
  })

  const resolveMarketMutation = useMutation({
    mutationFn: async () => {
      if (!selectedMarket || !selectedOutcomeId) return
      return api.resolveMarket(selectedMarket.id, selectedOutcomeId)
    },
    onSuccess: () => {
      setResolveSuccess(true)
      setShowResolveFlow(false)
      setSelectedMarket(null)
      setSelectedOutcomeId('')
      queryClient.invalidateQueries({ queryKey: ['markets'] })
      setTimeout(() => setResolveSuccess(false), 5000)
    },
  })

  const handleCreateSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    createMarketMutation.mutate()
  }

  const handleResolveClick = (market: Market) => {
    setSelectedMarket(market)
    setShowResolveFlow(true)
    setSelectedOutcomeId('')
  }

  const handleConfirmResolve = () => {
    resolveMarketMutation.mutate()
  }

  if (!user?.is_admin) {
    return null
  }

  return (
    <div>
      <PageHeader
        title="Admin Panel"
        gradient
        actions={
          !showCreateForm ? (
            <Button variant="gradient" onClick={() => setShowCreateForm(true)}>
              Create Market
            </Button>
          ) : undefined
        }
      />

      {/* Success Messages */}
      {createSuccess && (
        <div className="mb-6 px-4 py-3 bg-success-50 border border-success-200 rounded-xl text-sm font-semibold text-success-700">
          Market created successfully
        </div>
      )}
      {resolveSuccess && (
        <div className="mb-6 px-4 py-3 bg-success-50 border border-success-200 rounded-xl text-sm font-semibold text-success-700">
          Market resolved successfully
        </div>
      )}

      {/* Create Market Form */}
      {showCreateForm && (
        <Card hover={false} gradient className="mb-10">
          <div className="p-6">
            <h2 className="text-lg font-bold text-neutral-900 mb-5">New Market</h2>
            <form onSubmit={handleCreateSubmit} className="space-y-4">
              <Input
                label="Title"
                type="text"
                value={title}
                onChange={(e) => setTitle(e.target.value)}
                required
                placeholder="e.g., Will ETH hit $10k?"
              />
              <Textarea
                label="Description"
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                required
                rows={3}
                placeholder="Describe the resolution criteria..."
              />
              <Input
                label="Category"
                type="text"
                value={category}
                onChange={(e) => setCategory(e.target.value)}
                required
                placeholder="e.g., crypto"
              />
              <div className="flex items-center gap-2">
                <input
                  id="market-binary"
                  type="checkbox"
                  checked={isBinary}
                  onChange={(e) => setIsBinary(e.target.checked)}
                  className="h-4 w-4 rounded border-neutral-300 text-primary-600 focus:ring-primary-500"
                />
                <label htmlFor="market-binary" className="text-sm font-semibold text-neutral-700">
                  Binary market (Yes/No)
                </label>
              </div>
              <Input
                label="Closes At"
                type="datetime-local"
                value={closesAt}
                onChange={(e) => setClosesAt(e.target.value)}
                required
              />
              <div className="flex items-center gap-3 pt-2">
                <Button
                  type="submit"
                  variant="primary"
                  loading={createMarketMutation.isPending}
                >
                  Create
                </Button>
                <Button
                  type="button"
                  variant="secondary"
                  onClick={() => setShowCreateForm(false)}
                >
                  Cancel
                </Button>
              </div>
              {createMarketMutation.isError && (
                <p className="text-sm font-medium text-danger-600">Failed to create market. Please try again.</p>
              )}
            </form>
          </div>
        </Card>
      )}

      {/* Draft Markets */}
      <section className="mb-10">
        <h2 className="text-sm font-bold uppercase tracking-wider text-neutral-500 mb-4">Draft Markets</h2>
        <Card hover={false}>
          {draftLoading ? (
            <div className="p-4 space-y-2">
              {[...Array(2)].map((_, i) => <Skeleton key={i} variant="table-row" />)}
            </div>
          ) : !draftMarkets || draftMarkets.length === 0 ? (
            <EmptyState title="No draft markets" />
          ) : (
            <div className="divide-y divide-neutral-100">
              {draftMarkets.map((market) => (
                <div key={market.id} className="p-4 flex items-center justify-between">
                  <div className="flex-1 min-w-0">
                    <h3 className="text-sm font-semibold text-neutral-900 truncate">{market.title}</h3>
                    <p className="text-xs text-neutral-500 mt-0.5">
                      <Badge variant="neutral" size="sm">{market.category}</Badge>
                    </p>
                  </div>
                  <Button
                    variant="success"
                    size="sm"
                    loading={updateStatusMutation.isPending}
                    onClick={() => updateStatusMutation.mutate({ id: market.id, status: 'open' })}
                  >
                    Open
                  </Button>
                </div>
              ))}
            </div>
          )}
        </Card>
      </section>

      {/* Open Markets */}
      <section className="mb-10">
        <h2 className="text-sm font-bold uppercase tracking-wider text-neutral-500 mb-4">Open Markets</h2>
        <Card hover={false}>
          {openLoading ? (
            <div className="p-4 space-y-2">
              {[...Array(2)].map((_, i) => <Skeleton key={i} variant="table-row" />)}
            </div>
          ) : !openMarkets || openMarkets.length === 0 ? (
            <EmptyState title="No open markets" />
          ) : (
            <div className="divide-y divide-neutral-100">
              {openMarkets.map((market) => (
                <div key={market.id} className="p-4 flex items-center justify-between">
                  <div className="flex-1 min-w-0">
                    <h3 className="text-sm font-semibold text-neutral-900 truncate">{market.title}</h3>
                    <div className="flex items-center gap-2 mt-1">
                      <Badge variant="neutral" size="sm">{market.category}</Badge>
                      <span className="text-xs text-neutral-400 tabular-nums">Vol: {market.volume}</span>
                    </div>
                  </div>
                  <Button
                    variant="secondary"
                    size="sm"
                    loading={updateStatusMutation.isPending}
                    onClick={() => updateStatusMutation.mutate({ id: market.id, status: 'closed' })}
                  >
                    Close
                  </Button>
                </div>
              ))}
            </div>
          )}
        </Card>
      </section>

      {/* Closed Markets */}
      <section>
        <h2 className="text-sm font-bold uppercase tracking-wider text-neutral-500 mb-4">Closed Markets</h2>
        <Card hover={false}>
          {marketsLoading ? (
            <div className="p-4 space-y-2">
              {[...Array(2)].map((_, i) => <Skeleton key={i} variant="table-row" />)}
            </div>
          ) : !closedMarkets || closedMarkets.length === 0 ? (
            <EmptyState title="No closed markets to resolve" />
          ) : (
            <div className="divide-y divide-neutral-100">
              {closedMarkets.map((market) => (
                <div
                  key={market.id}
                  data-testid="market-row"
                  className="p-4"
                >
                  <div className="flex items-center justify-between">
                    <div className="flex-1 min-w-0">
                      <h3 className="text-sm font-semibold text-neutral-900 truncate">
                        {market.title}
                      </h3>
                      <div className="flex items-center gap-2 mt-1">
                        <Badge variant="neutral" size="sm">{market.category}</Badge>
                        <span className="text-xs text-neutral-400 tabular-nums">Vol: {market.volume}</span>
                      </div>
                    </div>
                    <Button
                      variant="primary"
                      size="sm"
                      onClick={() => handleResolveClick(market)}
                    >
                      Resolve
                    </Button>
                  </div>

                  {showResolveFlow && selectedMarket?.id === market.id && (
                    <div className="mt-4 p-4 bg-neutral-50 rounded-xl border border-neutral-200">
                      <p className="text-sm font-semibold text-neutral-700 mb-3">
                        Select winning outcome:
                      </p>
                      <div className="space-y-2 mb-4">
                        {(market.outcomes ?? []).map((outcome) => (
                          <label
                            key={outcome.id}
                            className="flex items-center gap-2 cursor-pointer"
                          >
                            <input
                              type="radio"
                              name={`resolve-${market.id}`}
                              value={outcome.id}
                              checked={selectedOutcomeId === outcome.id}
                              onChange={() => setSelectedOutcomeId(outcome.id)}
                              className="h-4 w-4 text-primary-600 focus:ring-primary-500"
                            />
                            <span className="text-sm text-neutral-700 font-medium">{outcome.label}</span>
                          </label>
                        ))}
                      </div>
                      <div className="flex items-center gap-2">
                        <Button
                          variant="success"
                          size="sm"
                          disabled={!selectedOutcomeId}
                          loading={resolveMarketMutation.isPending}
                          onClick={handleConfirmResolve}
                        >
                          Confirm
                        </Button>
                        <Button
                          variant="secondary"
                          size="sm"
                          onClick={() => {
                            setShowResolveFlow(false)
                            setSelectedOutcomeId('')
                          }}
                        >
                          Cancel
                        </Button>
                      </div>
                      {resolveMarketMutation.isError && (
                        <p className="mt-2 text-sm font-medium text-danger-600">Failed to resolve market.</p>
                      )}
                    </div>
                  )}
                </div>
              ))}
            </div>
          )}
        </Card>
      </section>
    </div>
  )
}
