'use client';

interface Tab {
  key: string;
  label: string;
}

interface TabsProps {
  tabs: Tab[];
  activeKey: string;
  onChange: (key: string) => void;
  className?: string;
}

export function Tabs({ tabs, activeKey, onChange, className = '' }: TabsProps) {
  return (
    <div className={`flex gap-1 border-b border-neutral-200 ${className}`}>
      {tabs.map((tab) => {
        const isActive = tab.key === activeKey;
        return (
          <button
            key={tab.key}
            onClick={() => onChange(tab.key)}
            className={`
              relative px-4 py-2.5 text-sm font-semibold transition-colors duration-200
              cursor-pointer select-none rounded-t-lg
              ${isActive
                ? 'text-primary-700'
                : 'text-neutral-500 hover:text-neutral-700 hover:bg-neutral-50'
              }
            `}
          >
            {tab.label}
            {isActive && (
              <span className="absolute bottom-0 left-0 right-0 h-0.5 gradient-primary rounded-full" />
            )}
          </button>
        );
      })}
    </div>
  );
}
