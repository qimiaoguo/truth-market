'use client'

import { useState, useEffect, useCallback } from 'react'
import { useAccount, useConnect, useDisconnect, useSignMessage } from 'wagmi'
import { useAuthStore } from '@/stores/authStore'
import { api } from '@/lib/api'
import type { User } from '@/lib/types'
import { Button } from '@/components/ui/Button'
import { Badge } from '@/components/ui/Badge'
import { Modal } from '@/components/ui/Modal'

/** Known wallet metadata for nice display */
const WALLET_META: Record<string, { name: string; icon: string }> = {
  'io.metamask': {
    name: 'MetaMask',
    icon: '🦊',
  },
  metaMaskSDK: {
    name: 'MetaMask',
    icon: '🦊',
  },
  'app.phantom': {
    name: 'Phantom',
    icon: '👻',
  },
  coinbaseWalletSDK: {
    name: 'Coinbase Wallet',
    icon: '🔵',
  },
  walletConnect: {
    name: 'WalletConnect',
    icon: '🔗',
  },
  injected: {
    name: 'Browser Wallet',
    icon: '🌐',
  },
}

function getWalletDisplay(connector: { id: string; name: string }) {
  const meta = WALLET_META[connector.id]
  return {
    name: meta?.name || connector.name || connector.id,
    icon: meta?.icon || '💳',
  }
}

export function WalletConnect() {
  const { isAuthenticated, user, setAuth, clearAuth } = useAuthStore()
  const [authenticating, setAuthenticating] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [showWalletModal, setShowWalletModal] = useState(false)
  const [connectingId, setConnectingId] = useState<string | null>(null)

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

  const handleConnectWallet = async (connector: (typeof connectors)[number]) => {
    setError(null)
    setConnectingId(connector.id)
    try {
      await connectAsync({ connector })
      setShowWalletModal(false)
    } catch (err) {
      console.error('Connect error:', err)
      setError('Failed to connect wallet')
    } finally {
      setConnectingId(null)
    }
  }

  // Mock fallback for development without a wallet
  const handleMockConnect = async () => {
    setShowWalletModal(false)
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

  // Deduplicate connectors by name (wagmi can register duplicates)
  const uniqueConnectors = connectors.filter((c, i) => {
    const display = getWalletDisplay(c)
    return connectors.findIndex((other) => getWalletDisplay(other).name === display.name) === i
  })

  if (isAuthenticated && user) {
    return (
      <div className="flex items-center gap-3">
        <Badge variant="primary" size="md">
          {Number(user.balance).toLocaleString()} U
        </Badge>
        <span
          className="text-xs font-medium text-neutral-500 hidden sm:inline cursor-default"
          title={user.wallet_address}
        >
          {user.wallet_address.slice(0, 6)}...{user.wallet_address.slice(-4)}
        </span>
        <Button variant="secondary" size="sm" onClick={handleDisconnect}>
          Disconnect
        </Button>
      </div>
    )
  }

  return (
    <>
      <div className="flex items-center gap-2">
        <Button
          variant="gradient"
          size="sm"
          onClick={() => setShowWalletModal(true)}
          loading={authenticating}
        >
          {authenticating ? 'Signing in...' : 'Connect Wallet'}
        </Button>
        {error && (
          <span className="text-xs text-danger-500">{error}</span>
        )}
      </div>

      <Modal
        open={showWalletModal}
        onClose={() => setShowWalletModal(false)}
        title="Connect Wallet"
      >
        <div className="space-y-2">
          {uniqueConnectors.map((connector) => {
            const display = getWalletDisplay(connector)
            const isLoading = connectingId === connector.id
            return (
              <button
                key={connector.id}
                onClick={() => handleConnectWallet(connector)}
                disabled={isLoading}
                className="w-full flex items-center gap-3 px-4 py-3 rounded-xl border border-neutral-200 hover:border-primary-300 hover:bg-primary-50/50 transition-all duration-200 cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed group"
              >
                <span className="text-2xl">{display.icon}</span>
                <span className="text-sm font-bold text-neutral-800 group-hover:text-primary-700 transition-colors">
                  {display.name}
                </span>
                {isLoading && (
                  <svg className="ml-auto h-4 w-4 animate-spin text-primary-500" viewBox="0 0 24 24" fill="none">
                    <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
                    <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z" />
                  </svg>
                )}
                {!isLoading && (
                  <svg className="ml-auto h-4 w-4 text-neutral-300 group-hover:text-primary-400 transition-colors" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round">
                    <path d="M9 18l6-6-6-6" />
                  </svg>
                )}
              </button>
            )
          })}

          {/* Divider */}
          <div className="flex items-center gap-3 py-2">
            <div className="flex-1 h-px bg-neutral-200" />
            <span className="text-xs font-medium text-neutral-400">or</span>
            <div className="flex-1 h-px bg-neutral-200" />
          </div>

          {/* Demo mode */}
          <button
            onClick={handleMockConnect}
            className="w-full flex items-center gap-3 px-4 py-3 rounded-xl border border-dashed border-neutral-300 hover:border-accent-300 hover:bg-accent-50/50 transition-all duration-200 cursor-pointer group"
          >
            <span className="text-2xl">🎮</span>
            <div className="text-left">
              <span className="text-sm font-bold text-neutral-700 group-hover:text-accent-700 transition-colors block">
                Demo Mode
              </span>
              <span className="text-xs text-neutral-400">
                Try without a wallet
              </span>
            </div>
          </button>
        </div>
      </Modal>
    </>
  )
}
