'use client'

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

  return (
    <header className="border-b border-neutral-200 bg-white">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
        <div className="flex items-center justify-between h-16">
          <div className="flex items-center gap-8">
            <Link href="/" className="text-xl font-bold text-primary-600">
              Truth Market
            </Link>
            <nav className="hidden sm:flex items-center gap-1">
              {navLinks.map(({ href, label }) => {
                const isActive = href === '/' ? pathname === '/' : pathname.startsWith(href)
                return (
                  <Link
                    key={href}
                    href={href}
                    className={`px-3 py-2 text-sm font-medium rounded-lg transition-colors ${
                      isActive
                        ? 'text-primary-600 bg-primary-50'
                        : 'text-neutral-600 hover:text-neutral-900 hover:bg-neutral-100'
                    }`}
                  >
                    {label}
                  </Link>
                )
              })}
            </nav>
          </div>
          <WalletConnect />
        </div>
      </div>
    </header>
  )
}
