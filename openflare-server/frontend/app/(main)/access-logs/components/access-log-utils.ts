export type AccessLogTab = "list" | "folds" | "ip-summary" | "ip-trend"

export type SearchDraft = {
  nodeId: string
  remoteAddr: string
  host: string
  path: string
}

export const PAGE_SIZE_OPTIONS = [20, 50, 100, 200]

export const DETAIL_SORT_OPTIONS = [
  { value: "logged_at:desc", label: "时间从新到旧" },
  { value: "logged_at:asc", label: "时间从旧到新" },
  { value: "status_code:desc", label: "状态码从高到低" },
  { value: "status_code:asc", label: "状态码从低到高" },
  { value: "remote_addr:asc", label: "IP 正序" },
  { value: "remote_addr:desc", label: "IP 倒序" },
]

export const FOLD_SORT_OPTIONS = [
  { value: "bucket_started_at:desc", label: "时间桶从新到旧" },
  { value: "bucket_started_at:asc", label: "时间桶从旧到新" },
  { value: "request_count:desc", label: "访问次数从高到低" },
  { value: "request_count:asc", label: "访问次数从低到高" },
]

export const IP_SORT_OPTIONS = [
  { value: "total_requests:desc", label: "总访问次数从高到低" },
  { value: "total_requests:asc", label: "总访问次数从低到高" },
  { value: "recent_requests:desc", label: "3 小时访问次数从高到低" },
  { value: "last_seen_at:desc", label: "最后访问时间从新到旧" },
]

export function parseSortValue(value: string) {
  const [sortBy = "logged_at", sortOrder = "desc"] = value.split(":")
  return {
    sortBy,
    sortOrder: sortOrder === "asc" ? ("asc" as const) : ("desc" as const),
  }
}

export function formatCompactNumber(value: number) {
  return new Intl.NumberFormat("zh-CN", {
    notation: value >= 10000 ? "compact" : "standard",
    maximumFractionDigits: 1,
  }).format(value)
}