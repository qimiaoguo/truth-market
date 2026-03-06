'use client'

import { useState, useEffect } from 'react'
import { useRouter } from 'next/navigation'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/lib/api'
import { useAuthStore } from '@/stores/authStore'
import type { Market } from '@/lib/types'

export default function AdminPage() {
  const router = useRouter()
  const queryClient = useQueryClient()
  const user = useAuthStore((s) => s.user)
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated)

  // Redirect non-admin users
  useEffect(() => {
    if (!isAuthenticated || !user?.isAdmin) {
      router.push('/')
    }
  }, [isAuthenticated, user, router])

  // State for create market form
  const [showCreateForm, setShowCreateForm] = useState(false)
  const [title, setTitle] = useState('')
  const [description, setDescription] = useState('')
  const [category, setCategory] = useState('')
  const [isBinary, setIsBinary] = useState(true)
  const [closesAt, setClosesAt] = useState('')
  const [createSuccess, setCreateSuccess] = useState(false)

  // State for resolve flow
  const [selectedMarket, setSelectedMarket] = useState<Market | null>(null)
  const [showResolveFlow, setShowResolveFlow] = useState(false)
  const [selectedOutcomeId, setSelectedOutcomeId] = useState<string>('')
  const [resolveSuccess, setResolveSuccess] = useState(false)

  // Fetch closed markets
  const { data: closedMarkets, isLoading: marketsLoading } = useQuery({
    queryKey: ['markets', 'closed'],
    queryFn: async () => {
      const res = await api.listMarkets({ status: 'closed' })
      return res.data?.markets ?? []
    },
    enabled: !!user?.isAdmin,
  })

  // Create market mutation
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

  // Resolve market mutation
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

  // Don't render if not admin
  if (!user?.isAdmin) {
    return null
  }

  return (
    <div className="min-h-screen bg-surface">
      <div className="max-w-5xl mx-auto px-4 py-8">
        <h1 className="text-3xl font-bold text-neutral-900 mb-8">Admin Panel</h1>

        {/* Success Messages */}
        {createSuccess && (
          <div className="mb-6 p-4 bg-success-50 border border-success-200 rounded-lg text-success-700 font-medium">
            Market created
          </div>
        )}
        {resolveSuccess && (
          <div className="mb-6 p-4 bg-success-50 border border-success-200 rounded-lg text-success-700 font-medium">
            Resolved
          </div>
        )}

        {/* Create Market Section */}
        <section className="mb-10">
          <div className="flex items-center justify-between mb-4">
            <h2 className="text-xl font-semibold text-neutral-800">Create Market</h2>
            {!showCreateForm && (
              <button
                onClick={() => setShowCreateForm(true)}
                className="px-4 py-2 bg-primary-600 text-white rounded-lg font-medium hover:bg-primary-700 transition-colors"
              >
                Create Market
              </button>
            )}
          </div>

          {showCreateForm && (
            <div className="bg-card rounded-xl border border-card-border shadow-sm p-6">
              <form onSubmit={handleCreateSubmit} className="space-y-4">
                <div>
                  <label htmlFor="market-title" className="block text-sm font-medium text-neutral-700 mb-1">
                    Title
                  </label>
                  <input
                    id="market-title"
                    type="text"
                    value={title}
                    onChange={(e) => setTitle(e.target.value)}
                    required
                    className="w-full px-3 py-2 border border-neutral-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-primary-500"
                    placeholder="e.g., Will ETH hit $10k?"
                  />
                </div>

                <div>
                  <label htmlFor="market-description" className="block text-sm font-medium text-neutral-700 mb-1">
                    Description
                  </label>
                  <textarea
                    id="market-description"
                    value={description}
                    onChange={(e) => setDescription(e.target.value)}
                    required
                    rows={3}
                    className="w-full px-3 py-2 border border-neutral-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-primary-500 resize-none"
                    placeholder="Describe the resolution criteria..."
                  />
                </div>

                <div>
                  <label htmlFor="market-category" className="block text-sm font-medium text-neutral-700 mb-1">
                    Category
                  </label>
                  <input
                    id="market-category"
                    type="text"
                    value={category}
                    onChange={(e) => setCategory(e.target.value)}
                    required
                    className="w-full px-3 py-2 border border-neutral-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-primary-500"
                    placeholder="e.g., crypto"
                  />
                </div>

                <div className="flex items-center gap-2">
                  <input
                    id="market-binary"
                    type="checkbox"
                    checked={isBinary}
                    onChange={(e) => setIsBinary(e.target.checked)}
                    className="h-4 w-4 rounded border-neutral-300 text-primary-600 focus:ring-primary-500"
                  />
                  <label htmlFor="market-binary" className="text-sm font-medium text-neutral-700">
                    Binary
                  </label>
                </div>

                <div>
                  <label htmlFor="market-closes-at" className="block text-sm font-medium text-neutral-700 mb-1">
                    Closes At
                  </label>
                  <input
                    id="market-closes-at"
                    type="datetime-local"
                    value={closesAt}
                    onChange={(e) => setClosesAt(e.target.value)}
                    required
                    className="w-full px-3 py-2 border border-neutral-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-primary-500"
                  />
                </div>

                <div className="flex items-center gap-3 pt-2">
                  <button
                    type="submit"
                    disabled={createMarketMutation.isPending}
                    className="px-4 py-2 bg-primary-600 text-white rounded-lg font-medium hover:bg-primary-700 transition-colors disabled:opacity-50"
                  >
                    {createMarketMutation.isPending ? 'Creating...' : 'Create'}
                  </button>
                  <button
                    type="button"
                    onClick={() => setShowCreateForm(false)}
                    className="px-4 py-2 bg-neutral-100 text-neutral-700 rounded-lg font-medium hover:bg-neutral-200 transition-colors"
                  >
                    Cancel
                  </button>
                </div>

                {createMarketMutation.isError && (
                  <p className="text-sm text-danger-600">Failed to create market. Please try again.</p>
                )}
              </form>
            </div>
          )}
        </section>

        {/* Closed Markets Section */}
        <section>
          <h2 className="text-xl font-semibold text-neutral-800 mb-4">Closed Markets</h2>

          <div className="bg-card rounded-xl border border-card-border shadow-sm">
            {marketsLoading ? (
              <div className="flex items-center justify-center py-12">
                <p className="text-neutral-500">Loading closed markets...</p>
              </div>
            ) : !closedMarkets || closedMarkets.length === 0 ? (
              <div className="flex items-center justify-center py-12">
                <p className="text-neutral-500">No closed markets to resolve.</p>
              </div>
            ) : (
              <div className="divide-y divide-neutral-100">
                {closedMarkets.map((market) => (
                  <div
                    key={market.id}
                    data-testid="market-row"
                    className="p-4 hover:bg-neutral-50 transition-colors cursor-pointer"
                    onClick={() => setSelectedMarket(market)}
                  >
                    <div className="flex items-center justify-between">
                      <div className="flex-1 min-w-0">
                        <h3 className="text-sm font-medium text-neutral-900 truncate">
                          {market.title}
                        </h3>
                        <p className="text-xs text-neutral-500 mt-0.5">
                          {market.category} &middot; Volume: {market.volume}
                        </p>
                      </div>
                      <button
                        onClick={(e) => {
                          e.stopPropagation()
                          handleResolveClick(market)
                        }}
                        className="ml-4 px-3 py-1.5 text-sm bg-primary-600 text-white rounded-lg font-medium hover:bg-primary-700 transition-colors"
                      >
                        Resolve
                      </button>
                    </div>

                    {/* Resolve Flow for this market */}
                    {showResolveFlow && selectedMarket?.id === market.id && (
                      <div
                        className="mt-4 p-4 bg-neutral-50 rounded-lg border border-neutral-200"
                        onClick={(e) => e.stopPropagation()}
                      >
                        <p className="text-sm font-medium text-neutral-700 mb-3">
                          Select winning outcome:
                        </p>
                        <div className="space-y-2 mb-4">
                          {market.outcomes.map((outcome) => (
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
                              <span className="text-sm text-neutral-700">{outcome.label}</span>
                            </label>
                          ))}
                        </div>
                        <div className="flex items-center gap-2">
                          <button
                            onClick={handleConfirmResolve}
                            disabled={!selectedOutcomeId || resolveMarketMutation.isPending}
                            className="px-3 py-1.5 text-sm bg-success-600 text-white rounded-lg font-medium hover:bg-success-700 transition-colors disabled:opacity-50"
                          >
                            {resolveMarketMutation.isPending ? 'Resolving...' : 'Confirm'}
                          </button>
                          <button
                            onClick={() => {
                              setShowResolveFlow(false)
                              setSelectedOutcomeId('')
                            }}
                            className="px-3 py-1.5 text-sm bg-neutral-200 text-neutral-700 rounded-lg font-medium hover:bg-neutral-300 transition-colors"
                          >
                            Cancel
                          </button>
                        </div>
                        {resolveMarketMutation.isError && (
                          <p className="mt-2 text-sm text-danger-600">Failed to resolve market.</p>
                        )}
                      </div>
                    )}
                  </div>
                ))}
              </div>
            )}
          </div>
        </section>
      </div>
    </div>
  )
}
