import type { ReactNode } from 'react';

interface PageHeaderProps {
  title: string;
  subtitle?: string;
  gradient?: boolean;
  actions?: ReactNode;
}

export function PageHeader({ title, subtitle, gradient = false, actions }: PageHeaderProps) {
  return (
    <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 mb-8">
      <div>
        <h1
          className={`text-3xl font-extrabold tracking-tight ${
            gradient ? 'gradient-text' : 'text-neutral-900'
          }`}
        >
          {title}
        </h1>
        {subtitle && (
          <p className="mt-1 text-sm text-neutral-500">{subtitle}</p>
        )}
      </div>
      {actions && <div className="flex items-center gap-3">{actions}</div>}
    </div>
  );
}
