import type { ReactNode } from 'react';

interface ContainerProps {
  className?: string;
  children: ReactNode;
}

export function Container({ className = '', children }: ContainerProps) {
  return (
    <div className={`max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 ${className}`}>
      {children}
    </div>
  );
}
