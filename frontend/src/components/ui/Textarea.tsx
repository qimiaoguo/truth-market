'use client';

import { forwardRef, type TextareaHTMLAttributes } from 'react';

interface TextareaProps extends TextareaHTMLAttributes<HTMLTextAreaElement> {
  label?: string;
  error?: string;
  helperText?: string;
}

export const Textarea = forwardRef<HTMLTextAreaElement, TextareaProps>(
  ({ label, error, helperText, className = '', id, ...props }, ref) => {
    const textareaId = id || label?.toLowerCase().replace(/\s+/g, '-');

    return (
      <div className="flex flex-col gap-1.5">
        {label && (
          <label
            htmlFor={textareaId}
            className="text-sm font-semibold text-neutral-700"
          >
            {label}
          </label>
        )}
        <textarea
          ref={ref}
          id={textareaId}
          className={`
            w-full px-3 py-2 text-sm rounded-lg resize-y min-h-[80px]
            bg-white border transition-all duration-200
            placeholder:text-neutral-400
            focus:outline-none focus:ring-2 focus:border-transparent
            ${error
              ? 'border-danger-400 focus:ring-danger-500 text-danger-900'
              : 'border-neutral-300 focus:ring-primary-500 text-neutral-900 hover:border-neutral-400'
            }
            disabled:opacity-50 disabled:cursor-not-allowed disabled:bg-neutral-50
            ${className}
          `}
          {...props}
        />
        {error && (
          <p className="text-xs font-medium text-danger-600">{error}</p>
        )}
        {helperText && !error && (
          <p className="text-xs text-neutral-500">{helperText}</p>
        )}
      </div>
    );
  }
);

Textarea.displayName = 'Textarea';
