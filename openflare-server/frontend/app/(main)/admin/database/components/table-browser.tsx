"use client"

import * as React from "react"
import {useCallback, useEffect, useState} from "react"
import {toast} from "sonner"
import {Layers, RefreshCw} from "lucide-react"

import {Card, CardContent, CardDescription, CardHeader, CardTitle} from "@/components/ui/card"
import {Skeleton} from "@/components/ui/skeleton"
import {Select, SelectContent, SelectItem, SelectTrigger, SelectValue} from "@/components/ui/select"
import {Table, TableBody, TableCell, TableHead, TableHeader, TableRow} from "@/components/ui/table"
import {Button} from "@/components/ui/button"
import type {TableDataResponse} from "@/lib/services/db-manage"
import {DbManageService} from "@/lib/services/db-manage"

/**
 * 格式化数字，每3位加逗号
 */
const formatNumber = (num: number | string) => {
  if (num === undefined || num === null) return "0"
  return num.toString().replace(/\B(?=(\d{3})+(?!\B))/g, ",")
}

/**
 * 格式化表格单元格内容
 */
const formatCellValue = (val: unknown): string => {
  if (val === null || val === undefined) return "-"
  if (typeof val === "boolean") return val ? "true" : "false"
  if (typeof val === "object") {
    try {
      return JSON.stringify(val)
    } catch {
      return "[Object]"
    }
  }
  return String(val)
}

interface TableBrowserProps {
  tables: string[]
  loadingTables: boolean
  refreshTrigger: number
}

export function TableBrowser({ tables, loadingTables, refreshTrigger }: TableBrowserProps) {
  const [selectedTable, setSelectedTable] = useState<string>("")
  const [tableData, setTableData] = useState<TableDataResponse | null>(null)
  const [page, setPage] = useState<number>(1)
  const [loadingData, setLoadingData] = useState<boolean>(false)
  const pageSize = 10

  // 获取具体表数据
  const fetchTableData = useCallback(async (tableName: string, targetPage: number, size: number) => {
    if (!tableName) return
    setLoadingData(true)
    try {
      const data = await DbManageService.getTableData({
        table: tableName,
        page: targetPage,
        pageSize: size,
      })
      setTableData(data)
    } catch (err) {
      toast.error(`获取数据表 ${tableName} 数据失败`, {
        description: err instanceof Error ? err.message : "未知错误",
      })
    } finally {
      setLoadingData(false)
    }
  }, [])

  // 默认选择第一张表
  useEffect(() => {
    if (tables.length > 0 && !selectedTable) {
      setSelectedTable(tables[0])
    }
  }, [tables, selectedTable])

  // 当选中的表、页码或外部刷新触发器改变时获取数据
  useEffect(() => {
    if (selectedTable) {
      fetchTableData(selectedTable, page, pageSize)
    }
  }, [selectedTable, page, pageSize, refreshTrigger, fetchTableData])

  // 处理页码改变
  const handlePageChange = (newPage: number) => {
    setPage(newPage)
  }

  // 处理表切换
  const handleTableChange = (value: string) => {
    setSelectedTable(value)
    setPage(1) // 切换表重置为第一页
  }

  return (
    <Card className="border-border/40 bg-card/50 backdrop-blur-sm shadow-sm">
      <CardHeader className="pb-3 border-b border-dashed flex flex-col md:flex-row md:items-center md:justify-between gap-4">
        <div className="space-y-0.5">
          <CardTitle className="text-sm font-semibold">数据表浏览器</CardTitle>
          <CardDescription className="text-[11px]">浏览数据库中的物理数据表详情及内容</CardDescription>
        </div>

        <div className="flex items-center gap-2">
          {loadingTables ? (
            <Skeleton className="h-8 w-48" />
          ) : (
            <Select value={selectedTable} onValueChange={handleTableChange}>
              <SelectTrigger className="h-8 w-[200px] text-xs bg-background border-border/40">
                <SelectValue placeholder="选择数据表" />
              </SelectTrigger>
              <SelectContent className="max-h-[300px]">
                {tables.map((t) => (
                  <SelectItem key={t} value={t} className="text-xs font-mono">{t}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          )}
        </div>
      </CardHeader>
      <CardContent className="pt-4">
        {loadingData && !tableData ? (
          <div className="space-y-3 py-6">
            <Skeleton className="h-6 w-full" />
            <Skeleton className="h-10 w-full" />
            <Skeleton className="h-10 w-full" />
            <Skeleton className="h-10 w-full" />
          </div>
        ) : tableData && tableData.columns.length > 0 ? (
          <div className="space-y-4">
            {/* 数据表格区域 */}
            <div className="border rounded-md overflow-x-auto max-w-full bg-background/50 relative">
              {loadingData && (
                <div className="absolute inset-0 bg-background/40 backdrop-blur-[1px] flex items-center justify-center z-10">
                  <RefreshCw className="size-5 text-primary animate-spin" />
                </div>
              )}
              <Table className="text-xs">
                <TableHeader className="bg-muted/40 font-semibold sticky top-0">
                  <TableRow>
                    {tableData.columns.map((col) => (
                      <TableHead key={col} className="font-semibold text-foreground py-2.5">{col}</TableHead>
                    ))}
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {tableData.results && tableData.results.length > 0 ? (
                    tableData.results.map((row, rIndex) => (
                      <TableRow key={rIndex} className="hover:bg-muted/10">
                        {tableData.columns.map((col) => (
                          <TableCell key={col} className="font-mono py-2">
                            <span className="truncate max-w-[200px] block" title={formatCellValue(row[col])}>
                              {formatCellValue(row[col])}
                            </span>
                          </TableCell>
                        ))}
                      </TableRow>
                    ))
                  ) : (
                    <TableRow>
                      <TableCell colSpan={tableData.columns.length} className="text-center py-10 text-muted-foreground">
                        表中无记录数据
                      </TableCell>
                    </TableRow>
                  )}
                </TableBody>
              </Table>
            </div>

            {/* 分页控制 */}
            <div className="flex items-center justify-between text-xs text-muted-foreground flex-wrap gap-2 pt-2 border-t border-dashed">
              <div>
                总行数: <span className="font-mono text-foreground font-semibold">{formatNumber(tableData.total)}</span> 条记录
              </div>
              <div className="flex items-center gap-2">
                <Button
                  variant="outline"
                  size="sm"
                  className="h-7 px-2 text-[11px]"
                  onClick={() => handlePageChange(page - 1)}
                  disabled={page <= 1 || loadingData}
                >
                  上一页
                </Button>
                <span className="text-xs px-2 font-mono">
                  第 {page} / {Math.max(1, Math.ceil(tableData.total / pageSize))} 页
                </span>
                <Button
                  variant="outline"
                  size="sm"
                  className="h-7 px-2 text-[11px]"
                  onClick={() => handlePageChange(page + 1)}
                  disabled={page >= Math.ceil(tableData.total / pageSize) || loadingData}
                >
                  下一页
                </Button>
              </div>
            </div>
          </div>
        ) : (
          <div className="flex flex-col items-center justify-center py-10 text-muted-foreground">
            <Layers className="size-8 opacity-45 mb-2" />
            <span className="text-xs">未选择数据表或表结构无法加载</span>
          </div>
        )}
      </CardContent>
    </Card>
  )
}
