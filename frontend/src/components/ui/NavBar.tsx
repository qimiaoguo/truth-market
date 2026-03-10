'use client'

import { useState } from 'react'
import Link from 'next/link'
import { usePathname } from 'next/navigation'
import { WalletConnect } from '@/components/wallet/WalletConnect'

const navLinks = [
  { href: '/', label: 'Markets' },
  { href: '/portfolio', label: 'Portfolio' },
  { href: '/rankings', label: 'Rankings' },
  { href: '/admin', label: 'Admin' },
]

export function NavBar() {
  const pathname = usePathname()
  const [mobileOpen, setMobileOpen] = useState(false)

  return (
    <header className="sticky top-0 z-50 border-b border-neutral-200/60 bg-white/80 backdrop-blur-xl">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
        <div className="flex items-center justify-between h-16">
          {/* Logo + Desktop Nav */}
          <div className="flex items-center gap-8">
            <Link href="/" className="text-xl font-extrabold tracking-tight gradient-text">
              Truth Market
            </Link>
            <nav className="hidden sm:flex items-center gap-1">
              {navLinks.map(({ href, label }) => {
                const isActive = href === '/' ? pathname === '/' : pathname.startsWith(href)
                return (
                  <Link
                    key={href}
                    href={href}
                    className={`relative px-3 py-2 text-sm font-semibold rounded-lg transition-all duration-200 ${
                      isActive
                        ? 'text-primary-700 bg-primary-50'
                        : 'text-neutral-500 hover:text-neutral-800 hover:bg-neutral-100'
                    }`}
                  >
                    {label}
                    {isActive && (
                      <span className="absolute bottom-0 left-3 right-3 h-0.5 gradient-primary rounded-full" />
                    )}
                  </Link>
                )
              })}
            </nav>
          </div>

          {/* Right side */}
          <div className="flex items-center gap-3">
            <WalletConnect />
            {/* Mobile hamburger */}
            <button
              onClick={() => setMobileOpen(!mobileOpen)}
              className="sm:hidden p-2 rounded-lg text-neutral-500 hover:text-neutral-800 hover:bg-neutral-100 transition-colors cursor-pointer"
            >
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round">
                {mobileOpen ? (
                  <path d="M18 6L6 18M6 6l12 12" />
                ) : (
                  <>
                    <path d="M4 6h16M4 12h16M4 18h16" />
                  </>
                )}
              </svg>
            </button>
          </div>
        </div>
      </div>

      {/* Mobile menu */}
      {mobileOpen && (
        <div className="sm:hidden border-t border-neutral-200/60 bg-white/95 backdrop-blur-xl">
          <nav className="px-4 py-3 flex flex-col gap-1">
            {navLinks.map(({ href, label }) => {
              const isActive = href === '/' ? pathname === '/' : pathname.startsWith(href)
              return (
                <Link
                  key={href}
                  href={href}
                  onClick={() => setMobileOpen(false)}
                  className={`px-3 py-2.5 text-sm font-semibold rounded-lg transition-colors ${
                    isActive
                      ? 'text-primary-700 bg-primary-50'
                      : 'text-neutral-600 hover:text-neutral-800 hover:bg-neutral-50'
                  }`}
                >
                  {label}
                </Link>
              )
            })}
          </nav>
        </div>
      )}
    </header>
  )
}
