import {Card, CardContent, CardDescription, CardHeader, CardTitle} from '@/components/ui/card'
import type {ZoneOverview} from '@/lib/services/openflare'
import {formatDateTime} from '@/lib/utils'

export function ZoneOverviewPanel({overview}: {overview: ZoneOverview}) {
  const boundRoutes = new Set(overview.domains.flatMap((domain) => domain.proxy_route_id ? [domain.proxy_route_id] : [])).size
  const certificates = overview.domains.filter((domain) => domain.cert_id !== null).length
  return <div className="grid gap-4 md:grid-cols-3">
    <Card className="shadow-none"><CardHeader className="pb-2"><CardTitle className="text-sm">域名</CardTitle><CardDescription>显式 FQDN</CardDescription></CardHeader><CardContent className="text-2xl font-semibold">{overview.domains.length}</CardContent></Card>
    <Card className="shadow-none"><CardHeader className="pb-2"><CardTitle className="text-sm">关联路由</CardTitle><CardDescription>已绑定的不同规则</CardDescription></CardHeader><CardContent className="text-2xl font-semibold">{boundRoutes}</CardContent></Card>
    <Card className="shadow-none"><CardHeader className="pb-2"><CardTitle className="text-sm">已配证书</CardTitle><CardDescription>域名级 TLS 绑定</CardDescription></CardHeader><CardContent className="text-2xl font-semibold">{certificates}</CardContent></Card>
    <Card className="shadow-none md:col-span-3"><CardHeader className="pb-2"><CardTitle className="text-sm">Zone 信息</CardTitle></CardHeader><CardContent className="grid gap-3 text-sm sm:grid-cols-2"><div><p className="text-xs text-muted-foreground">根域</p><p className="mt-1 font-medium">{overview.zone.domain}</p></div><div><p className="text-xs text-muted-foreground">创建时间</p><p className="mt-1">{formatDateTime(overview.zone.created_at)}</p></div></CardContent></Card>
  </div>
}
