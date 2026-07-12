import {EmptyStateWithBorder} from '@/components/layout/empty'
import {Card, CardContent, CardDescription, CardHeader, CardTitle} from '@/components/ui/card'
import type {ZoneDomainItem} from '@/lib/services/openflare'
import {Route} from 'lucide-react'

export function ZoneRouteSummary({domains}: {domains: ZoneDomainItem[]}) {
  const groups = Map.groupBy(domains.filter((domain) => domain.proxy_route_id !== null), (domain) => domain.proxy_route_id!)
  return <Card className="shadow-none"><CardHeader><CardTitle className="text-base">路由关联</CardTitle><CardDescription>域名只能关联一个反代路由；路由可关联多个 Zone 域名。</CardDescription></CardHeader><CardContent>{groups.size === 0 ? <EmptyStateWithBorder icon={Route} description="暂无关联反代路由。" /> : <div className="space-y-3">{[...groups].map(([routeID, items]) => <div key={routeID} className="rounded-lg border p-3"><p className="text-sm font-semibold">路由 #{routeID}</p><p className="mt-1 text-xs text-muted-foreground">{items.map((item) => item.domain).join(' · ')}</p></div>)}</div>}</CardContent></Card>
}
