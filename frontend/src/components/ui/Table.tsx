import type { HTMLAttributes, TdHTMLAttributes, ThHTMLAttributes } from 'react';

export function Table({ className = '', children, ...props }: HTMLAttributes<HTMLTableElement>) {
  return (
    <div className="w-full overflow-x-auto">
      <table className={`w-full text-sm ${className}`} {...props}>
        {children}
      </table>
    </div>
  );
}

export function Thead({ className = '', children, ...props }: HTMLAttributes<HTMLTableSectionElement>) {
  return (
    <thead className={`border-b border-neutral-200 ${className}`} {...props}>
      {children}
    </thead>
  );
}

export function Tbody({ className = '', children, ...props }: HTMLAttributes<HTMLTableSectionElement>) {
  return (
    <tbody className={`divide-y divide-neutral-100 ${className}`} {...props}>
      {children}
    </tbody>
  );
}

export function Tr({ className = '', children, ...props }: HTMLAttributes<HTMLTableRowElement>) {
  return (
    <tr
      className={`transition-colors hover:bg-neutral-50 ${className}`}
      {...props}
    >
      {children}
    </tr>
  );
}

export function Th({ className = '', children, ...props }: ThHTMLAttributes<HTMLTableCellElement>) {
  return (
    <th
      className={`px-4 py-3 text-left text-[11px] font-bold uppercase tracking-wider text-neutral-500 ${className}`}
      {...props}
    >
      {children}
    </th>
  );
}

export function Td({ className = '', children, ...props }: TdHTMLAttributes<HTMLTableCellElement>) {
  return (
    <td
      className={`px-4 py-3 text-neutral-700 tabular-nums ${className}`}
      {...props}
    >
      {children}
    </td>
  );
}
