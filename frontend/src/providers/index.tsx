'use client'

import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { WagmiProvider } from 'wagmi'
import { useState, useEffect } from 'react'
import { useAuthStore } from '@/stores/authStore'
import { api } from '@/lib/api'
import { wagmiConfig } from '@/lib/wagmi'

function AuthSync() {
  const token = useAuthStore((s) => s.token)
  useEffect(() => {
    api.setToken(token)
  }, [token])
  return null
}

export function Providers({ children }: { children: React.ReactNode }) {
  const [queryClient] = useState(() => new QueryClient({
    defaultOptions: {
      queries: {
        staleTime: 30_000,
        retry: 1,
      },
    },
  }))

  return (
    <WagmiProvider config={wagmiConfig}>
      <QueryClientProvider client={queryClient}>
        <AuthSync />
        {children}
      </QueryClientProvider>
    </WagmiProvider>
  )
}
