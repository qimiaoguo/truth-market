import type { ReactNode } from 'react';

type BadgeVariant = 'primary' | 'success' | 'warning' | 'danger' | 'neutral' | 'accent';
type BadgeSize = 'sm' | 'md';

interface BadgeProps {
  variant?: BadgeVariant;
  size?: BadgeSize;
  children: ReactNode;
  className?: string;
}

const variantStyles: Record<BadgeVariant, string> = {
  primary: 'bg-primary-100 text-primary-700 ring-primary-200',
  success: 'bg-success-100 text-success-700 ring-success-200',
  warning: 'bg-warning-100 text-warning-700 ring-warning-200',
  danger: 'bg-danger-100 text-danger-700 ring-danger-200',
  neutral: 'bg-neutral-100 text-neutral-600 ring-neutral-200',
  accent: 'bg-accent-100 text-accent-700 ring-accent-200',
};

const sizeStyles: Record<BadgeSize, string> = {
  sm: 'px-2 py-0.5 text-[11px]',
  md: 'px-2.5 py-1 text-xs',
};

export function Badge({ variant = 'neutral', size = 'sm', children, className = '' }: BadgeProps) {
  return (
    <span
      className={`
        inline-flex items-center font-bold rounded-full
        ring-1 ring-inset
        ${variantStyles[variant]}
        ${sizeStyles[size]}
        ${className}
      `}
    >
      {children}
    </span>
  );
}
