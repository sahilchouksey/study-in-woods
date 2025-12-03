import * as React from "react"

import { cn } from "@/lib/utils"

function Input({ className, type, ...props }: React.ComponentProps<"input">) {
  return (
    <input
      type={type}
      data-slot="input"
      className={cn(
        "file:text-foreground placeholder:text-muted-foreground selection:bg-primary selection:text-primary-foreground h-9 w-full min-w-0 rounded-md border px-3 py-1 text-base shadow-xs transition-[color,box-shadow] outline-none file:inline-flex file:h-7 file:border-0 file:bg-transparent file:text-sm file:font-medium md:text-sm",
        "bg-white dark:bg-neutral-900 border-neutral-200 dark:border-neutral-800",
        "focus-visible:border-black dark:focus-visible:border-white focus-visible:ring-black/10 dark:focus-visible:ring-white/10 focus-visible:ring-[3px]",
        "disabled:cursor-not-allowed disabled:bg-neutral-100 dark:disabled:bg-neutral-800 disabled:text-neutral-500 dark:disabled:text-neutral-400 disabled:border-neutral-200 dark:disabled:border-neutral-700",
        "aria-invalid:ring-destructive/20 dark:aria-invalid:ring-destructive/40 aria-invalid:border-destructive",
        className
      )}
      {...props}
    />
  )
}

export { Input }
