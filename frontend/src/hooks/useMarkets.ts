import { useQuery } from '@tanstack/react-query'
import { api } from '@/lib/api'

export function useMarkets(params?: { status?: string; category?: string }) {
  return useQuery({
    queryKey: ['markets', params],
    queryFn: async () => {
      const res = await api.listMarkets(params)
      return res.data?.markets ?? []
    },
  })
}
