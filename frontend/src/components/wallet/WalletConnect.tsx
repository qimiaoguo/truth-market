'use client'

import { useState, useEffect, useCallback } from 'react'
import { useAccount, useConnect, useDisconnect, useSignMessage } from 'wagmi'
import { injected } from 'wagmi/connectors'
import { useAuthStore } from '@/stores/authStore'
import { api } from '@/lib/api'
import type { User } from '@/lib/types'

export function WalletConnect() {
  const { isAuthenticated, user, setAuth, clearAuth } = useAuthStore()
  const [authenticating, setAuthenticating] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const { address, isConnected } = useAccount()
  const { connectAsync, connectors } = useConnect()
  const { disconnectAsync } = useDisconnect()
  const { signMessageAsync } = useSignMessage()

  // Authenticate with backend after wallet connects
  const authenticateWithBackend = useCallback(async (walletAddress: string) => {
    setAuthenticating(true)
    setError(null)
    try {
      // Step 1: Get nonce from backend
      const nonceRes = await api.getNonce(walletAddress)
      if (!nonceRes.ok || !nonceRes.data) {
        throw new Error('Failed to get nonce')
      }
      const nonce = nonceRes.data.nonce

      // Step 2: Construct SIWE message
      const domain = typeof window !== 'undefined' ? window.location.host : 'localhost:3000'
      const origin = typeof window !== 'undefined' ? window.location.origin : 'http://localhost:3000'
      const issuedAt = new Date().toISOString()
      const message = [
        `${domain} wants you to sign in with your Ethereum account:`,
        walletAddress,
        '',
        'Sign in to Truth Market',
        '',
        `URI: ${origin}`,
        `Version: 1`,
        `Chain ID: 1`,
        `Nonce: ${nonce}`,
        `Issued At: ${issuedAt}`,
      ].join('\n')

      // Step 3: Sign with wallet
      let signature: string
      try {
        signature = await signMessageAsync({ message })
      } catch {
        // User rejected signature, disconnect wallet
        await disconnectAsync()
        setError('Signature rejected')
        return
      }

      // Step 4: Verify with backend
      const verifyRes = await api.verifySignature(message, signature, walletAddress)
      if (verifyRes.ok && verifyRes.data) {
        setAuth(verifyRes.data.user, verifyRes.data.token)
      } else {
        throw new Error('Verification failed')
      }
    } catch (err) {
      console.error('Auth error:', err)
      setError(err instanceof Error ? err.message : 'Authentication failed')
      // Disconnect on auth failure
      try { await disconnectAsync() } catch { /* ignore */ }
    } finally {
      setAuthenticating(false)
    }
  }, [signMessageAsync, disconnectAsync, setAuth])

  // When wallet connects, auto-authenticate with backend
  useEffect(() => {
    if (isConnected && address && !isAuthenticated && !authenticating) {
      authenticateWithBackend(address)
    }
  }, [isConnected, address, isAuthenticated, authenticating, authenticateWithBackend])

  const handleConnect = async () => {
    setError(null)
    try {
      // Try injected connector (MetaMask, etc.)
      const injectedConnector = connectors.find(c => c.id === 'injected') || connectors[0]
      if (injectedConnector) {
        await connectAsync({ connector: injectedConnector })
      } else {
        // Fallback: try injected directly
        await connectAsync({ connector: injected() })
      }
    } catch (err) {
      console.error('Connect error:', err)
      // If no wallet found, use mock for demo
      await handleMockConnect()
    }
  }

  // Mock fallback for development without a wallet
  const handleMockConnect = async () => {
    setAuthenticating(true)
    setError(null)
    try {
      const mockAddress = `0x${Array.from({ length: 40 }, () => Math.floor(Math.random() * 16).toString(16)).join('')}`

      const nonceRes = await api.getNonce(mockAddress)
      if (nonceRes.ok && nonceRes.data) {
        const nonce = nonceRes.data.nonce
        const message = [
          `localhost:3000 wants you to sign in with your Ethereum account:`,
          mockAddress,
          '',
          'Sign in to Truth Market',
          '',
          `URI: http://localhost:3000`,
          `Version: 1`,
          `Chain ID: 1`,
          `Nonce: ${nonce}`,
          `Issued At: ${new Date().toISOString()}`,
        ].join('\n')
        const mockSignature = '0xmocksignature'
        const verifyRes = await api.verifySignature(message, mockSignature, mockAddress)
        if (verifyRes.ok && verifyRes.data) {
          setAuth(verifyRes.data.user, verifyRes.data.token)
          return
        }
      }
      // If API fails, use full mock
      const mockUser: User = {
        id: 'mock-user-1',
        wallet_address: mockAddress,
        display_name: 'Demo User',
        user_type: 'human',
        balance: '1000',
        locked_balance: '0',
        is_admin: false,
        created_at: new Date().toISOString(),
      }
      setAuth(mockUser, 'mock-token')
    } finally {
      setAuthenticating(false)
    }
  }

  const handleDisconnect = async () => {
    clearAuth()
    api.setToken(null)
    try { await disconnectAsync() } catch { /* ignore if not connected */ }
  }

  if (isAuthenticated && user) {
    return (
      <div className="flex items-center gap-3">
        <span className="text-sm font-medium text-neutral-700">
          {Number(user.balance).toLocaleString()} U
        </span>
        <span className="text-xs text-neutral-500 hidden sm:inline" title={user.wallet_address}>
          {user.wallet_address.slice(0, 6)}...{user.wallet_address.slice(-4)}
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
    <div className="flex items-center gap-2">
      <button
        onClick={handleConnect}
        disabled={authenticating}
        className="px-4 py-2 text-sm font-medium text-white bg-primary-600 rounded-lg hover:bg-primary-700 disabled:opacity-50 transition-colors cursor-pointer"
      >
        {authenticating ? 'Signing in...' : 'Connect Wallet'}
      </button>
      {!authenticating && (
        <button
          onClick={handleMockConnect}
          className="px-3 py-2 text-sm text-neutral-500 hover:text-neutral-700 transition-colors cursor-pointer"
          title="Demo mode without wallet"
        >
          Demo
        </button>
      )}
      {error && (
        <span className="text-xs text-red-500">{error}</span>
      )}
    </div>
  )
}
