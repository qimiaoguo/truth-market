import { useQuery } from '@tanstack/react-query'
import { api } from '@/lib/api'
import type { RankDimension } from '@/lib/types'

export function useRankings(dimension: RankDimension, userType?: string) {
  return useQuery({
    queryKey: ['rankings', dimension, userType],
    queryFn: async () => {
      const res = await api.getRankings({ dimension, user_type: userType })
      return res.data?.rankings ?? []
    },
  })
}
