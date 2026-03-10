'use client';

import { forwardRef, type ButtonHTMLAttributes, type ReactNode } from 'react';

type ButtonVariant = 'primary' | 'secondary' | 'success' | 'danger' | 'ghost' | 'gradient';
type ButtonSize = 'sm' | 'md' | 'lg';

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant;
  size?: ButtonSize;
  loading?: boolean;
  children: ReactNode;
}

const variantStyles: Record<ButtonVariant, string> = {
  primary:
    'bg-primary-600 text-white hover:bg-primary-700 active:bg-primary-800 shadow-sm hover:shadow-md',
  secondary:
    'bg-neutral-100 text-neutral-700 hover:bg-neutral-200 active:bg-neutral-300 border border-neutral-200',
  success:
    'bg-success-600 text-white hover:bg-success-700 active:bg-success-600 shadow-sm hover:shadow-md',
  danger:
    'bg-danger-600 text-white hover:bg-danger-700 active:bg-danger-600 shadow-sm hover:shadow-md',
  ghost:
    'bg-transparent text-neutral-600 hover:bg-neutral-100 hover:text-neutral-800 active:bg-neutral-200',
  gradient:
    'gradient-primary text-white shadow-md hover:shadow-lg hover:brightness-110 active:brightness-95',
};

const sizeStyles: Record<ButtonSize, string> = {
  sm: 'px-3 py-1.5 text-xs font-semibold gap-1.5',
  md: 'px-4 py-2 text-sm font-semibold gap-2',
  lg: 'px-6 py-3 text-base font-bold gap-2.5',
};

function Spinner({ size }: { size: ButtonSize }) {
  const dim = size === 'sm' ? 'w-3 h-3' : size === 'lg' ? 'w-5 h-5' : 'w-4 h-4';
  return (
    <svg
      className={`${dim} animate-spin`}
      viewBox="0 0 24 24"
      fill="none"
    >
      <circle
        cx="12"
        cy="12"
        r="10"
        stroke="currentColor"
        strokeWidth="3"
        strokeLinecap="round"
        className="opacity-25"
      />
      <path
        d="M4 12a8 8 0 018-8"
        stroke="currentColor"
        strokeWidth="3"
        strokeLinecap="round"
        className="opacity-75"
      />
    </svg>
  );
}

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(
  ({ variant = 'primary', size = 'md', loading = false, disabled, children, className = '', ...props }, ref) => {
    const isDisabled = disabled || loading;

    return (
      <button
        ref={ref}
        disabled={isDisabled}
        className={`
          inline-flex items-center justify-center rounded-lg
          transition-all duration-200 ease-out
          focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 focus-visible:ring-offset-2
          disabled:opacity-50 disabled:cursor-not-allowed disabled:pointer-events-none
          cursor-pointer select-none
          ${variantStyles[variant]}
          ${sizeStyles[size]}
          ${className}
        `}
        {...props}
      >
        {loading && <Spinner size={size} />}
        {children}
      </button>
    );
  }
);

Button.displayName = 'Button';
