'use client'

import { useState } from 'react'
import { useAuthStore } from '@/stores/authStore'
import { api } from '@/lib/api'
import type { User } from '@/lib/types'

export function WalletConnect() {
  const { isAuthenticated, user, setAuth, clearAuth } = useAuthStore()
  const [connecting, setConnecting] = useState(false)

  const handleConnect = async () => {
    setConnecting(true)
    try {
      const mockAddress = `0x${Array.from({ length: 40 }, () => Math.floor(Math.random() * 16).toString(16)).join('')}`

      // Try real API first
      try {
        const nonceRes = await api.getNonce(mockAddress)
        if (nonceRes.ok && nonceRes.data) {
          const nonce = nonceRes.data.nonce
          const mockSignature = '0xmocksignature'
          const verifyRes = await api.verifySignature(nonce, mockSignature)
          if (verifyRes.ok && verifyRes.data) {
            setAuth(verifyRes.data.user, verifyRes.data.token)
            return
          }
        }
      } catch {
        // API not available, fall back to mock
      }

      // Mock fallback for demo/testing
      const mockUser: User = {
        id: 'mock-user-1',
        walletAddress: mockAddress,
        displayName: 'Demo User',
        userType: 'human',
        balance: '1000',
        lockedBalance: '0',
        isAdmin: false,
        createdAt: new Date().toISOString(),
      }
      setAuth(mockUser, 'mock-token')
    } finally {
      setConnecting(false)
    }
  }

  const handleDisconnect = () => {
    clearAuth()
    api.setToken(null)
  }

  if (isAuthenticated && user) {
    return (
      <div className="flex items-center gap-3">
        <span className="text-sm font-medium text-neutral-700">
          {Number(user.balance).toLocaleString()} U
        </span>
        <button
          onClick={handleDisconnect}
          className="px-3 py-1.5 text-sm rounded-lg border border-neutral-300 text-neutral-600 hover:bg-neutral-100 transition-colors cursor-pointer"
        >
          Disconnect
        </button>
      </div>
    )
  }

  return (
    <button
      onClick={handleConnect}
      disabled={connecting}
      className="px-4 py-2 text-sm font-medium text-white bg-primary-600 rounded-lg hover:bg-primary-700 disabled:opacity-50 transition-colors cursor-pointer"
    >
      {connecting ? 'Connecting...' : 'Connect Wallet'}
    </button>
  )
}
