import { useQuery } from '@tanstack/react-query'
import { api } from '@/lib/api'

export function useOrders(params?: { status?: string; marketId?: string }) {
  return useQuery({
    queryKey: ['orders', params],
    queryFn: async () => {
      const res = await api.getOrders({
        status: params?.status,
        market_id: params?.marketId,
      })
      return res.data?.orders ?? []
    },
  })
}
