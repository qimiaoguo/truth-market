import { useCallback } from 'react'
import { api } from '@/lib/api'
import { useAuthStore } from '@/stores/authStore'

export function useRefreshUser() {
  const updateUser = useAuthStore((s) => s.updateUser)

  return useCallback(async () => {
    try {
      const res = await api.getMe()
      if (res.ok && res.data) {
        updateUser(res.data)
      }
    } catch {
      // Silently fail — balance will update on next page load.
    }
  }, [updateUser])
}
