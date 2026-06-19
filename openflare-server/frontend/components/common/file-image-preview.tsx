"use client"

import * as React from "react"
import {ImageOff} from "lucide-react"

import {cn} from "@/lib/utils"
import {getFileUrl, type ImageQuality} from "@/lib/services/upload/utils"

type FileImagePreviewProps = {
  fileId: string | number
  alt: string
  quality?: ImageQuality
  className?: string
  fallbackClassName?: string
  variant?: "compact" | "default"
}

export function FileImagePreview({
  fileId,
  alt,
  quality = "low",
  className,
  fallbackClassName,
  variant = "default",
}: FileImagePreviewProps) {
  const [failed, setFailed] = React.useState(false)
  const src = getFileUrl(fileId, quality)

  React.useEffect(() => {
    setFailed(false)
  }, [fileId, quality])

  if (!src || failed) {
    return (
      <div
        role="img"
        aria-label={`${alt} 加载失败`}
        className={cn(
          "flex size-full flex-col items-center justify-center gap-1 bg-muted/50 text-muted-foreground",
          fallbackClassName,
        )}
      >
        <ImageOff className={cn("shrink-0 opacity-70", variant === "compact" ? "size-3.5" : "size-5")} />
        {variant === "default" && (
          <span className="text-[10px] leading-none">加载失败</span>
        )}
      </div>
    )
  }

  return (
    // eslint-disable-next-line @next/next/no-img-element
    <img
      src={src}
      alt={alt}
      loading="lazy"
      decoding="async"
      className={className}
      onError={() => setFailed(true)}
    />
  )
}