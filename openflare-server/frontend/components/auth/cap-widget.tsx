'use client'

import {useEffect, useRef, useState} from 'react'
import {CheckCircle2, Loader2, ShieldAlert, ShieldCheck, ShieldQuestion} from 'lucide-react'
import {getCapToken} from '@/lib/cap-solver'

type CapStatus = 'idle' | 'solving' | 'solved' | 'error'

interface CapWidgetProps {
  /** Called with the one-time token once the challenge is solved */
  onToken: (token: string) => void
  /** Called when the challenge fails or encounters an error */
  onError?: (err: Error) => void
  /** Whether to auto-start solving when the component mounts. Defaults to true. */
  autoStart?: boolean
  /** PoW scope sent to the backend */
  scope?: string
}

/**
 * Cap 人机验证小部件
 *
 * autoStart=true（默认）：挂载后立即在后台运行 PoW 求解，完成后回调 onToken。
 * autoStart=false：显示「点击开始验证」按钮，用户手动触发后才开始求解。
 */
export function CapWidget({ onToken, onError, autoStart = true, scope = 'login' }: CapWidgetProps) {
  const [status, setStatus] = useState<CapStatus>('idle')
  const [errorMsg, setErrorMsg] = useState('')
  const solving = useRef(false)

  const solve = async () => {
    if (solving.current) return
    solving.current = true
    setStatus('solving')
    setErrorMsg('')
    try {
      const token = await getCapToken(scope)
      setStatus('solved')
      onToken(token)
    } catch (err) {
      const error = err instanceof Error ? err : new Error(String(err))
      setStatus('error')
      setErrorMsg(error.message)
      onError?.(error)
    } finally {
      solving.current = false
    }
  }

  // Auto-start on mount (only when autoStart is enabled)
  useEffect(() => {
    if (autoStart) {
      void solve()
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  return (
    <div className="flex items-center gap-2 rounded-lg border border-border/60 bg-muted/30 px-3 py-2 text-sm">
      {/* idle + autoStart=false: 手动触发按钮 */}
      {status === 'idle' && !autoStart && (
        <button
          type="button"
          className="flex w-full items-center gap-2 text-left"
          onClick={() => void solve()}
        >
          <ShieldQuestion className="size-4 shrink-0 text-muted-foreground" />
          <span className="flex-1 text-muted-foreground">点击开始人机验证</span>
        </button>
      )}

      {/* idle + autoStart=true: 等待自动开始（极短暂状态） */}
      {status === 'idle' && autoStart && (
        <>
          <ShieldAlert className="size-4 shrink-0 text-muted-foreground" />
          <span className="text-muted-foreground">等待人机验证…</span>
        </>
      )}

      {status === 'solving' && (
        <>
          <Loader2 className="size-4 shrink-0 animate-spin text-primary" />
          <span className="text-muted-foreground">正在完成人机验证…</span>
        </>
      )}

      {status === 'solved' && (
        <>
          <CheckCircle2 className="size-4 shrink-0 text-green-500" />
          <span className="text-green-600 dark:text-green-400">人机验证通过</span>
          <ShieldCheck className="ml-auto size-4 shrink-0 text-green-500" />
        </>
      )}

      {status === 'error' && (
        <button
          type="button"
          className="flex w-full items-center gap-2 text-left"
          onClick={() => void solve()}
        >
          <ShieldAlert className="size-4 shrink-0 text-destructive" />
          <span className="flex-1 text-destructive">
            {errorMsg || '人机验证失败'}
          </span>
          <span className="text-xs text-muted-foreground underline">重试</span>
        </button>
      )}
    </div>
  )
}
