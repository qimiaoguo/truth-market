interface SkeletonProps {
  variant?: 'text' | 'card' | 'circle' | 'table-row';
  className?: string;
}

export function Skeleton({ variant = 'text', className = '' }: SkeletonProps) {
  const base = 'animate-pulse bg-neutral-200 rounded';

  switch (variant) {
    case 'text':
      return <div className={`${base} h-4 w-full rounded-md ${className}`} />;
    case 'card':
      return <div className={`${base} h-40 w-full rounded-xl ${className}`} />;
    case 'circle':
      return <div className={`${base} h-10 w-10 rounded-full ${className}`} />;
    case 'table-row':
      return (
        <div className={`flex items-center gap-4 py-3 ${className}`}>
          <div className={`${base} h-4 w-1/6 rounded-md`} />
          <div className={`${base} h-4 w-1/4 rounded-md`} />
          <div className={`${base} h-4 w-1/5 rounded-md`} />
          <div className={`${base} h-4 w-1/6 rounded-md`} />
        </div>
      );
  }
}
