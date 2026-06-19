"use client"

import {useEffect, useMemo, useState} from "react"
import {useQuery} from "@tanstack/react-query"

import {Button} from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import {Input} from "@/components/ui/input"
import {Label} from "@/components/ui/label"
import {ProxyRouteService} from "@/lib/services/openflare"

type UptimeKumaSiteSelectModalProps = {
  open: boolean
  selectedSites: string[]
  onOpenChange: (open: boolean) => void
  onSave: (sites: string[]) => void
}

export function UptimeKumaSiteSelectModal({
  open,
  selectedSites,
  onOpenChange,
  onSave,
}: UptimeKumaSiteSelectModalProps) {
  const [searchTerm, setSearchTerm] = useState("")
  const [tempSelected, setTempSelected] = useState<Set<string>>(new Set())

  const routesQuery = useQuery({
    queryKey: ["openflare", "proxy-routes"],
    queryFn: () => ProxyRouteService.list(),
    enabled: open,
  })

  useEffect(() => {
    if (!open) return
    setTempSelected(new Set(selectedSites.map((site) => site.trim()).filter(Boolean)))
    setSearchTerm("")
  }, [open, selectedSites])

  const filteredRoutes = useMemo(() => {
    const routes = routesQuery.data ?? []
    const keyword = searchTerm.trim().toLowerCase()
    if (!keyword) return routes
    return routes.filter(
      (route) =>
        route.site_name.toLowerCase().includes(keyword) ||
        route.primary_domain.toLowerCase().includes(keyword),
    )
  }, [routesQuery.data, searchTerm])

  const toggleSite = (siteName: string) => {
    setTempSelected((previous) => {
      const next = new Set(previous)
      if (next.has(siteName)) next.delete(siteName)
      else next.add(siteName)
      return next
    })
  }

  const handleSelectAll = () => {
    setTempSelected((previous) => {
      const next = new Set(previous)
      filteredRoutes.forEach((route) => next.add(route.site_name))
      return next
    })
  }

  const handleDeselectAll = () => {
    setTempSelected((previous) => {
      const next = new Set(previous)
      filteredRoutes.forEach((route) => next.delete(route.site_name))
      return next
    })
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-2xl">
        <DialogHeader>
          <DialogTitle>选择监控站点</DialogTitle>
          <DialogDescription>
            选择要同步到 Uptime Kuma 的反代站点，支持按站点名称和域名搜索。
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4">
          <div className="space-y-1.5">
            <Label>搜索站点</Label>
            <Input
              value={searchTerm}
              onChange={(event) => setSearchTerm(event.target.value)}
              placeholder="按名称或域名搜索..."
            />
          </div>

          <div className="flex flex-wrap gap-2">
            <Button type="button" variant="outline" size="sm" onClick={handleSelectAll}>
              全选过滤项
            </Button>
            <Button type="button" variant="outline" size="sm" onClick={handleDeselectAll}>
              清空过滤项
            </Button>
          </div>

          <div className="max-h-72 overflow-y-auto rounded-lg border">
            {routesQuery.isLoading ? (
              <p className="p-4 text-sm text-muted-foreground">加载站点列表...</p>
            ) : routesQuery.isError ? (
              <p className="p-4 text-sm text-destructive">
                {routesQuery.error instanceof Error
                  ? routesQuery.error.message
                  : "加载站点失败"}
              </p>
            ) : filteredRoutes.length === 0 ? (
              <p className="p-4 text-sm text-muted-foreground">无匹配的站点</p>
            ) : (
              <table className="w-full text-left text-sm">
                <thead className="sticky top-0 bg-muted/60 text-xs uppercase text-muted-foreground">
                  <tr>
                    <th className="w-12 px-3 py-2">选择</th>
                    <th className="px-3 py-2">站点名称</th>
                    <th className="px-3 py-2">主域名</th>
                  </tr>
                </thead>
                <tbody className="divide-y">
                  {filteredRoutes.map((route) => {
                    const checked = tempSelected.has(route.site_name)
                    return (
                      <tr
                        key={route.id}
                        className="cursor-pointer hover:bg-muted/30"
                        onClick={() => toggleSite(route.site_name)}
                      >
                        <td className="px-3 py-2" onClick={(event) => event.stopPropagation()}>
                          <input
                            type="checkbox"
                            checked={checked}
                            onChange={() => toggleSite(route.site_name)}
                            className="size-4 rounded border-input"
                          />
                        </td>
                        <td className="px-3 py-2 font-medium">{route.site_name}</td>
                        <td className="px-3 py-2 text-muted-foreground">{route.primary_domain}</td>
                      </tr>
                    )
                  })}
                </tbody>
              </table>
            )}
          </div>

          <p className="text-right text-xs text-muted-foreground">
            已选择 {tempSelected.size} 个监控站点
          </p>
        </div>

        <DialogFooter>
          <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
            取消
          </Button>
          <Button
            type="button"
            onClick={() => {
              onSave(Array.from(tempSelected))
              onOpenChange(false)
            }}
          >
            保存选择
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}