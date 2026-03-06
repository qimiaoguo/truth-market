import { useQuery } from '@tanstack/react-query'
import { api } from '@/lib/api'

export function usePositions(marketId?: string) {
  return useQuery({
    queryKey: ['positions', marketId],
    queryFn: async () => {
      const res = await api.getPositions(
        marketId ? { market_id: marketId } : undefined
      )
      return res.data?.positions ?? []
    },
  })
}
