import type { ReactNode, HTMLAttributes } from 'react';

interface CardProps extends HTMLAttributes<HTMLDivElement> {
  hover?: boolean;
  gradient?: boolean;
  children: ReactNode;
}

export function Card({ hover = true, gradient = false, children, className = '', ...props }: CardProps) {
  return (
    <div
      className={`
        bg-card rounded-xl border border-card-border
        shadow-sm overflow-hidden
        transition-all duration-300 ease-out
        ${hover ? 'hover:shadow-md hover:-translate-y-0.5 hover:border-neutral-300' : ''}
        ${gradient ? 'border-t-0' : ''}
        ${className}
      `}
      {...props}
    >
      {gradient && (
        <div className="h-1 w-full gradient-primary" />
      )}
      {children}
    </div>
  );
}
