'use client';

import Image from 'next/image';
import Link from 'next/link';
import { usePathname } from 'next/navigation';

import { Code, FileCog, PanelLeft, Workflow, AppWindowMac } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Sheet, SheetContent, SheetTrigger } from '@/components/ui/sheet';
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip';
import { Toaster } from '@/components/ui/toaster';
import { ThemeProvider } from '@/components/ui/theme-provider';
import { ModeToggle } from '@/components/ui/mode-toggle';

import './globals.css';

import localFont from 'next/font/local';
import { useEffect } from 'react';
const inter = localFont({ src: './../fonts/Inter-VariableFont_slnt,wght.ttf' });

const NAV_ITEMS = [
  {
    title: 'Nodes',
    Icon: Workflow,
    path: '/',
  },
  {
    title: 'Jobs',
    Icon: FileCog,
    path: '/jobs/',
  },
  {
    title: 'Code',
    Icon: Code,
    path: '/code/',
  },
  {
    title: 'Node Web',
    Icon: AppWindowMac,
    path: '/node/',
  },
];

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  const currentPath = usePathname();

  let title = NAV_ITEMS.find((item) => item.path == currentPath)?.title;

  return (
    <html lang="en">
      <head>
        <title>{`${title ? title + ' - ' : ''}Reef`}</title>
        <link rel="icon" href="/logo-no-text.svg" type="image/svg+xml" />
      </head>
      <body className={inter.className}>
        <ThemeProvider
          attribute="class"
          defaultTheme="dark"
          enableSystem
          disableTransitionOnChange
        >
          <TooltipProvider>
            <div className="flex flex-col h-screen w-full bg-background min-h-svh">
              <aside className="fixed inset-y-0 left-0 z-10 hidden w-14 flex-col border-r bg-card sm:flex">
                <nav className="flex flex-col items-center gap-4 px-2 sm:py-5">
                  <Image
                    src="/logo-no-text.svg"
                    alt="Reef logo"
                    width={36}
                    height={36}
                  />
                  {NAV_ITEMS.map((item) => (
                    <Tooltip key={item.title}>
                      <TooltipTrigger asChild>
                        <Link
                          href={item.path}
                          className={`
                            flex h-9 w-9 items-center justify-center rounded-lg text-muted-foreground transition-colors hover:text-foreground md:h-8 md:w-8
                            ${
                              currentPath === item.path
                                ? 'bg-accent text-accent-foreground'
                                : ''
                            }
                          `}
                        >
                          <item.Icon className="h-5 w-5" />
                          <span className="sr-only">{item.title}</span>
                        </Link>
                      </TooltipTrigger>
                      <TooltipContent side="right">{item.title}</TooltipContent>
                    </Tooltip>
                  ))}
                </nav>
                <nav className="mt-auto flex flex-col items-center gap-4 px-2 sm:py-5">
                  <ModeToggle />
                  {/*
                   <Tooltip>
                     <TooltipTrigger asChild>
                       <Link
                         href="#"
                         className="flex h-9 w-9 items-center justify-center rounded-lg text-muted-foreground transition-colors hover:text-foreground md:h-8 md:w-8"
                       >
                         <Settings className="h-5 w-5" />
                         <span className="sr-only">Settings</span>
                       </Link>
                     </TooltipTrigger>
                     <TooltipContent side="right">Settings</TooltipContent>
                   </Tooltip>
                  */}
                </nav>
              </aside>
              <div className="flex flex-col sm:gap-4 sm:pl-14 min-h-svh">
                <nav className="sm:hidden sticky top-0 z-10 p-4 bg-background border-b">
                  <Sheet>
                    <SheetTrigger asChild>
                      <Button size="icon" variant="outline">
                        <PanelLeft className="h-5 w-5" />
                        <span className="sr-only">Toggle Menu</span>
                      </Button>
                    </SheetTrigger>
                    <SheetContent side="left">
                      <nav className="grid gap-6 text-lg font-medium">
                        {NAV_ITEMS.map((item) => (
                          <Link
                            key={item.title}
                            href={item.path}
                            className={`flex items-center gap-4 px-2.5 ${
                              currentPath === item.path
                                ? 'text-foreground'
                                : 'text-muted-foreground hover:text-foreground'
                            }`}
                          >
                            <item.Icon className="h-5 w-5" />
                            {item.title}
                          </Link>
                        ))}
                      </nav>
                    </SheetContent>
                  </Sheet>
                </nav>

                {children}
              </div>
            </div>
          </TooltipProvider>
        </ThemeProvider>
        <Toaster />
      </body>
    </html>
  );
}
