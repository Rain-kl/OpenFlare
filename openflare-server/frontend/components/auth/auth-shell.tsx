"use client"

import type {ReactNode} from "react"
import {motion} from "motion/react"
import {WavesIcon} from "lucide-react"

import {cn} from "@/lib/utils"

interface AuthShellProps {
  children: ReactNode
  wide?: boolean
}

export function AuthShell({children, wide = false}: AuthShellProps) {
  return (
    <main className="relative flex min-h-screen w-full items-center justify-center overflow-x-hidden bg-background px-4 py-6 sm:px-8 sm:py-8 [@media(max-height:700px)]:py-3">
      <div
        aria-hidden="true"
        className="pointer-events-none absolute inset-0 bg-[linear-gradient(to_right,var(--border)_1px,transparent_1px),linear-gradient(to_bottom,var(--border)_1px,transparent_1px)] bg-[size:56px_56px] opacity-45 [mask-image:linear-gradient(to_bottom,transparent,black_12%,black_88%,transparent)]"
      />
      <div
        aria-hidden="true"
        className="pointer-events-none absolute inset-y-0 left-1/2 w-px -translate-x-1/2 bg-border/70"
      />
      <motion.section
        initial={{opacity: 0, y: 18}}
        animate={{opacity: 1, y: 0}}
        transition={{duration: 0.45, ease: "easeOut"}}
        className={cn(
          "relative w-full border-x border-dashed border-border/80 bg-background/95 px-6 py-8 backdrop-blur-sm sm:px-10 sm:py-10 [@media(max-height:700px)]:py-5",
          wide ? "max-w-2xl" : "max-w-xl",
        )}
      >
        {children}
      </motion.section>
    </main>
  )
}

interface AuthHeadingProps {
  title: string
  description: string
  siteName?: string
}

export function AuthHeading({
  title,
  description,
  siteName = "OpenFlare",
}: AuthHeadingProps) {
  return (
    <header className="flex flex-col gap-6 [@media(max-height:700px)]:gap-3">
      <div className="flex items-center gap-3">
        <span className="flex size-9 items-center justify-center rounded-full bg-foreground text-background [@media(max-height:700px)]:size-8">
          <WavesIcon aria-hidden="true" />
        </span>
        <span className="font-semibold tracking-tight [@media(min-height:761px)]:text-lg">{siteName}</span>
      </div>
      <div className="flex flex-col gap-1.5 [@media(max-height:700px)]:gap-1">
        <h1 className="text-2xl font-semibold tracking-tight text-foreground sm:text-3xl [@media(min-height:900px)]:sm:text-4xl">
          {title}
        </h1>
        <p className="text-sm text-muted-foreground [@media(min-height:900px)]:text-base">{description}</p>
      </div>
    </header>
  )
}
