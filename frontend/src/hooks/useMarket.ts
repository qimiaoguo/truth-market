import { useQuery } from '@tanstack/react-query'
import { api } from '@/lib/api'

export function useMarket(id: string) {
  return useQuery({
    queryKey: ['market', id],
    queryFn: async () => {
      const res = await api.getMarket(id)
      return res.data?.market ?? null
    },
    enabled: !!id,
  })
}
