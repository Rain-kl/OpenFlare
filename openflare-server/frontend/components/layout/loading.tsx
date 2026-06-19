import * as React from "react"
import {motion} from "motion/react"
import {cn} from "@/lib/utils"
import {FileTextIcon} from "lucide-react"

/**
 * 加载页面组件
 *
 * 用于统一显示加载状态
 * @example
 * ```tsx
 * <LoadingPage text="系统" badgeText="系统" />
 * ```
 * @param {string} text - 显示的文本内容，默认为"系统"
 * @param {string} badgeText - 显示的徽章文本，默认为"系统"
 * @returns {React.ReactNode} 加载页面组件
 */

export function AppleSpinner({ className = "size-8 text-muted-foreground" }: { className?: string }) {
  const spokes = Array.from({ length: 12 })
  return (
    <div className={cn("relative flex items-center justify-center", className)}>
      <style dangerouslySetInnerHTML={{ __html: `
        @keyframes apple-spinner-fade {
          0% { opacity: 1; }
          100% { opacity: 0.15; }
        }
        .apple-spinner-spoke {
          animation: apple-spinner-fade 1.2s linear infinite;
        }
      `}} />
      <div className="relative size-full">
        {spokes.map((_, i) => (
          <div
            key={i}
            className="apple-spinner-spoke absolute top-0 left-[46%] w-[8%] h-[28%] bg-current rounded-full"
            style={{
              transform: `rotate(${i * 30}deg)`,
              transformOrigin: "50% 178.5%",
              animationDelay: `${(i - 12) * 0.1}s`,
            }}
          />
        ))}
      </div>
    </div>
  )
}

/**
 * 加载页面组件
 *
 * 用于统一显示加载状态
 */
export function LoadingPage(props: { text?: string; badgeText?: string }) {
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  const _ = props
  return (
    <div className="absolute inset-0 z-50 overflow-hidden font-sans bg-background/80 backdrop-blur-md text-foreground flex items-center justify-center">
      <motion.div
        initial={{ opacity: 0, scale: 0.96 }}
        animate={{ opacity: 1, scale: 1 }}
        transition={{ duration: 0.25, ease: "easeOut" }}
        className="flex items-center justify-center"
      >
        <AppleSpinner className="size-9 text-muted-foreground/80" />
      </motion.div>
    </div>
  )
}

interface LoadingStateProps extends React.ComponentProps<"div"> {
  title?: string
  description?: string
  icon?: React.ComponentType<{ className?: string }>
  iconSize?: "sm" | "md" | "lg"
}

/**
 * 加载状态展示组件
 * 用于统一显示加载中的状态
 */
export function LoadingState({
  title = "加载中",
  description = "正在获取活动数据...",
  icon: Icon = FileTextIcon,
  className,
  iconSize = "md",
}: LoadingStateProps) {
  const iconSizes = { sm: "size-8", md: "size-10", lg: "size-14" }
  const iconInnerSizes = { sm: "size-4", md: "size-5", lg: "size-7" }

  return (
    <div className={cn("flex flex-col items-center justify-center py-12 text-center", className)}>
      <div className={cn(
        "rounded-full bg-muted flex items-center justify-center mb-2 animate-pulse",
        iconSizes[iconSize]
      )}>
        <Icon className={cn("text-muted-foreground", iconInnerSizes[iconSize])} />
      </div>

      {title && (
        <h3 className="text-sm font-medium mb-1 animate-pulse">
          {title}
        </h3>
      )}

      {description && (
        <p className="text-xs text-muted-foreground max-w-md animate-pulse">
          {description}
        </p>
      )}
    </div>
  )
}

/**
 * 带边框的加载状态组件
 */
export function LoadingStateWithBorder(props: LoadingStateProps) {
  return (
    <div className="border border-dashed rounded-lg">
      <LoadingState {...props} />
    </div>
  )
}
