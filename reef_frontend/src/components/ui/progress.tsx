'use client';

import * as React from 'react';
import * as ProgressPrimitive from '@radix-ui/react-progress';

import { cn } from '@/lib/utils';
import { useTheme } from 'next-themes';

const Progress = React.forwardRef<
  React.ElementRef<typeof ProgressPrimitive.Root>,
  React.ComponentPropsWithoutRef<typeof ProgressPrimitive.Root>
>(({ className, value, ...props }, ref) => {
  const { theme } = useTheme();
  const isDarkMode = theme === 'dark';

  return (
    <ProgressPrimitive.Root
      ref={ref}
      className={cn(
        'relative h-4 w-full overflow-hidden rounded-full bg-secondary',
        className
      )}
      {...props}
    >
      <ProgressPrimitive.Indicator
        className="h-full w-full flex-1 bg-primary transition-all"
        style={{
          transform: `translateX(-${100 - (value || 0)}%)`,
          background: isDarkMode
            ? 'linear-gradient(90deg, rgba(170,220,200, 1) 0%, rgba(143,149,255,1) 65%, rgba(255,158,253,1) 100%)'
            : 'linear-gradient(90deg, rgba(85,255,206,1) 0%, rgba(113,94,252,1) 50%, rgba(249,176,225,1) 100%)',
          opacity: isDarkMode ? '100%' : '80%',
          filter: isDarkMode ? 'brightness(70%)' : '',
        }}
      />
    </ProgressPrimitive.Root>
  );
});
Progress.displayName = ProgressPrimitive.Root.displayName;

export { Progress };
