import {type ClassValue, clsx} from "clsx"
import {twMerge} from "tailwind-merge"

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

export function formatDateTime(dateStr: string | Date) {
  try {
    const date = typeof dateStr === 'string' ? new Date(dateStr) : dateStr
    return new Intl.DateTimeFormat('zh-CN', {
      timeZone: 'Asia/Shanghai',
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
      hour12: false
    }).format(date)
  } catch {
    return String(dateStr)
  }
}

/**
 * 格式化日期为本地时间字符串（带时区）
 * @param date 要格式化的日期
 * @returns 格式化后的日期字符串，如 "2024-01-15T00:00:00+08:00"
 */
export function formatLocalDate(date: Date): string {
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  const hours = String(date.getHours()).padStart(2, '0')
  const minutes = String(date.getMinutes()).padStart(2, '0')
  const seconds = String(date.getSeconds()).padStart(2, '0')
  return `${ year }-${ month }-${ day }T${ hours }:${ minutes }:${ seconds }+08:00`
}

type Base64Buffer = {
  from: (input: string, encoding: 'utf-8') => { toString: (encoding: 'base64') => string }
}

/**
 * Base64 编码
 * @param value 待编码字符串
 * @returns Base64 编码后的字符串
 */
export function encodeBase64(value: string): string {
  if (typeof globalThis.btoa === 'function') {
    return globalThis.btoa(value)
  }

  const bufferConstructor = (globalThis as typeof globalThis & { Buffer?: Base64Buffer }).Buffer
  if (bufferConstructor) {
    return bufferConstructor.from(value, 'utf-8').toString('base64')
  }

  throw new Error('当前环境不支持 Base64 编码')
}

/**
 * 生成交易缓存的唯一键
 * @param params 交易查询参数
 * @returns 缓存键字符串
 */
export function generateTransactionCacheKey(params: {
  types?: string[]
  statuses?: string[]
  payee_transfer_status?: string
  client_id?: string
  page?: number
  page_size?: number
  startTime?: string
  endTime?: string
  id?: string
  order_name?: string
  payer_username?: string
  payee_username?: string
}): string {
  const typesKey = params.types?.length ? params.types.sort().join(',') : 'all'
  const statusesKey = params.statuses?.length ? params.statuses.sort().join(',') : 'all'
  const transferStatusKey = params.payee_transfer_status || 'all'
  const clientIdKey = params.client_id || 'all'
  const startTimeKey = params.startTime || 'no-start'
  const endTimeKey = params.endTime || 'no-end'
  const idKey = params.id || 'no-id'
  const orderNameKey = params.order_name || 'no-name'
  const payerKey = params.payer_username || 'no-payer'
  const payeeKey = params.payee_username || 'no-payee'

  return `${ typesKey }_${ statusesKey }_${ transferStatusKey }_${ clientIdKey }_${ params.page }_${ params.page_size }_${ startTimeKey }_${ endTimeKey }_${ idKey }_${ orderNameKey }_${ payerKey }_${ payeeKey }`
}

/**
 * 验证并净化重定向目标 URL，防止 Open Redirect 和 XSS 攻击。
 * 只允许以单个斜杠 "/" 开头的相对路径，拒绝 "//"、协议、反斜杠、控制字符等编码变体。
 */
export function safeRedirectTarget(url: string | null | undefined, fallback = "/"): string {
  if (!url) return fallback

  // 1. 拒绝包含控制字符、空白字符或反斜杠的 URL
  if (/[\u0000-\u001F\u007F-\u009F\s\\]/.test(url)) {
    return fallback
  }

  // 2. 必须以单个 '/' 开头，且不能以 '//' 开头
  if (!url.startsWith("/") || url.startsWith("//")) {
    return fallback
  }

  // 3. 递归 URL 解码并检测潜在的绕过载荷
  try {
    let decoded = url
    let prev = ""
    let attempts = 0
    while (decoded !== prev && decoded.includes("%") && attempts < 3) {
      prev = decoded
      decoded = decodeURIComponent(decoded)
      attempts++
    }

    // 检查解码后的内容是否包含控制字符、空白字符或反斜杠
    if (/[\u0000-\u001F\u007F-\u009F\s\\]/.test(decoded)) {
      return fallback
    }

    // 检查解码后的内容是否以单个 '/' 开头，且不能以 '//' 开头
    if (!decoded.startsWith("/") || decoded.startsWith("//")) {
      return fallback
    }

    // 提取路径部分（即问号 ? 或井号 # 之前的内容），确保其中不含冒号（防止 scheme）、双斜杠或反斜杠
    const pathPart = decoded.split(/[?#]/)[0]
    if (pathPart.includes(":") || pathPart.includes("//") || pathPart.includes("\\")) {
      return fallback
    }
  } catch {
    return fallback
  }

  return url
}

