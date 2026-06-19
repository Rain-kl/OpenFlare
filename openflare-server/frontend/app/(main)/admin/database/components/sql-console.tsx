"use client"

import * as React from "react"
import {useRef, useState} from "react"
import {useTheme} from "next-themes"
import CodeMirror from "@uiw/react-codemirror"
import {sql} from "@codemirror/lang-sql"
import {toast} from "sonner"
import {ArrowLeft, Play, RefreshCw, Terminal, Trash2,} from "lucide-react"

import {Button} from "@/components/ui/button"
import {Select, SelectContent, SelectItem, SelectTrigger, SelectValue} from "@/components/ui/select"
import {Table, TableBody, TableCell, TableHead, TableHeader, TableRow} from "@/components/ui/table"
import type {ExecuteSQLResponse} from "@/lib/services/db-manage"
import {DbManageService} from "@/lib/services/db-manage"

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

interface SQLConsoleProps {
  dbType?: string // "postgres" | "sqlite"
  onClose: () => void
}

export function SQLConsole({ dbType, onClose }: SQLConsoleProps) {
  // SQL 控制台状态
  const [sqlQuery, setSqlQuery] = useState<string>("")
  const [executingSQL, setExecutingSQL] = useState<boolean>(false)
  const [sqlResult, setSqlResult] = useState<ExecuteSQLResponse | null>(null)
  const [sqlError, setSqlError] = useState<string | null>(null)

  // 拖拽与主题状态
  const containerRef = useRef<HTMLDivElement>(null)
  const [editorHeight, setEditorHeight] = useState<number>(240)
  const { resolvedTheme } = useTheme()
  const cmTheme = resolvedTheme === "dark" ? "dark" : "light"

  const handleMouseDown = (e: React.MouseEvent) => {
    e.preventDefault()
    const startY = e.clientY
    const startHeight = editorHeight

    const handleMouseMove = (moveEvent: MouseEvent) => {
      if (!containerRef.current) return
      const deltaY = moveEvent.clientY - startY
      const containerHeight = containerRef.current.getBoundingClientRect().height
      const newHeight = startHeight + deltaY

      // Limit editor height between 80px and containerHeight - 80px
      if (newHeight > 80 && newHeight < containerHeight - 80) {
        setEditorHeight(newHeight)
      }
    }

    const handleMouseUp = () => {
      document.removeEventListener("mousemove", handleMouseMove)
      document.removeEventListener("mouseup", handleMouseUp)
    }

    document.addEventListener("mousemove", handleMouseMove)
    document.addEventListener("mouseup", handleMouseUp)
  }

  // 执行 SQL 查询
  const handleExecuteSQL = async () => {
    if (!sqlQuery.trim()) return
    setExecutingSQL(true)
    setSqlResult(null)
    setSqlError(null)
    try {
      const result = await DbManageService.executeSQL(sqlQuery)
      setSqlResult(result)
      toast.success("SQL 执行成功")
    } catch (err) {
      setSqlError(err instanceof Error ? err.message : "未知执行错误")
      toast.error("SQL 执行失败")
    } finally {
      setExecutingSQL(false)
    }
  }

  // Preset SQL 选择器
  const handlePresetSQLChange = (query: string) => {
    setSqlQuery(query)
  }

  const handleClose = () => {
    setSqlResult(null)
    setSqlError(null)
    onClose()
  }

  return (
    <div className="py-6 px-1 space-y-6 w-full">
      {/* 顶部控制与标题 */}
      <div className="flex items-center justify-between pb-2">
        <div className="flex items-center gap-3">
          <Button
            variant="outline"
            size="icon"
            className="h-8 w-8"
            onClick={handleClose}
          >
            <ArrowLeft className="size-4" />
          </Button>
          <div className="flex items-center gap-2">
            <Terminal className="size-5 text-primary" />
            <div>
              <h1 className="text-2xl font-semibold tracking-tight">SQL 查询终端</h1>
            </div>
          </div>
        </div>
        <div className="text-xs text-muted-foreground font-mono">
          {dbType === "postgres" ? "PostgreSQL Connected" : "SQLite Connected"}
        </div>
      </div>

      {/* 类似于 VS Code 的单个整体编辑器+结果区域，固定高度，有分水岭拖拽调整大小 */}
      <div
        ref={containerRef}
        className="w-full border border-border/40 bg-card/60 backdrop-blur-md rounded-lg overflow-hidden flex flex-col shadow-sm h-[calc(100vh-140px)] min-h-[500px]"
      >
        {/* 顶部编辑工具栏 */}
        <div className="flex items-center justify-between px-4 py-2 border-b bg-muted/40 shrink-0 gap-4 flex-wrap">
          <div className="flex items-center gap-2">
            <Terminal className="size-4 text-primary" />
            <span className="text-xs font-semibold">SQL 编辑器</span>
          </div>

          <div className="flex items-center gap-3 flex-wrap">
            {/* 快速模板 */}
            <div className="flex items-center gap-1.5">
              <span className="text-[11px] text-muted-foreground">快速模板:</span>
              <Select onValueChange={handlePresetSQLChange}>
                <SelectTrigger className="h-7 w-[180px] text-[11px] bg-background">
                  <SelectValue placeholder="选择预设 SQL" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="SELECT * FROM users LIMIT 10;">查询用户 (SELECT users)</SelectItem>
                  <SelectItem value="SELECT * FROM uploads LIMIT 10;">查询文件 (SELECT uploads)</SelectItem>
                  <SelectItem value="SELECT * FROM task_executions ORDER BY created_at DESC LIMIT 10;">查询任务流水 (SELECT task_executions)</SelectItem>
                  <SelectItem value="SELECT sqlite_version();">SQLite 版本 (SQLite only)</SelectItem>
                  <SelectItem value="SELECT version();">PostgreSQL 版本 (Postgres only)</SelectItem>
                </SelectContent>
              </Select>
            </div>

            {/* 控制按钮 */}
            <div className="flex items-center gap-1.5">
              <Button
                variant="ghost"
                size="icon"
                className="h-7 w-7 text-muted-foreground hover:text-foreground"
                onClick={() => setSqlQuery("")}
                title="清空编辑器"
              >
                <Trash2 className="size-3.5" />
              </Button>
              <Button
                size="sm"
                className="h-7 px-3 gap-1 text-[11px]"
                onClick={handleExecuteSQL}
                disabled={executingSQL || !sqlQuery.trim()}
              >
                <Play className="size-3" />
                {executingSQL ? "运行中..." : "运行"}
              </Button>
            </div>
          </div>
        </div>

        {/* 上半区：编辑器输入框 */}
        <div
          style={{ height: editorHeight }}
          className="w-full overflow-hidden relative min-h-[100px] bg-background"
        >
          <CodeMirror
            value={sqlQuery}
            height="100%"
            extensions={[sql()]}
            theme={cmTheme}
            onChange={(value) => setSqlQuery(value)}
            className="h-full text-xs font-mono"
            basicSetup={{
              lineNumbers: true,
              foldGutter: true,
              dropCursor: true,
              allowMultipleSelections: false,
              indentOnInput: true,
            }}
          />
        </div>

        {/* 拖动分割线 */}
        <div
          onMouseDown={handleMouseDown}
          className="h-1.5 bg-border/60 hover:bg-primary/50 cursor-row-resize transition-colors flex items-center justify-center shrink-0 select-none z-10"
          title="拖动调整大小"
        >
          <div className="w-8 h-1 rounded bg-muted-foreground/30" />
        </div>

        {/* 下半区：查看结果 (可滑动查看) */}
        <div className="flex-1 min-h-[100px] flex flex-col overflow-hidden bg-muted/10">
          {/* 结果栏工具提示 */}
          <div className="flex items-center justify-between px-4 py-1.5 border-b bg-muted/20 shrink-0 text-[11px] text-muted-foreground font-mono">
            <span className="font-semibold">执行输出</span>
            {sqlResult && (
              <span>
                类型: {sqlResult.type.toUpperCase()} | 耗时: {sqlResult.execution_time_ms} ms
              </span>
            )}
          </div>

          {/* 滚动结果内容 */}
          <div className="flex-1 overflow-auto p-4 min-h-0">
            {executingSQL && (
              <div className="flex flex-col items-center justify-center h-full py-10 space-y-2">
                <RefreshCw className="size-5 text-primary animate-spin" />
                <span className="text-xs text-muted-foreground">正在在数据库执行查询，请稍候...</span>
              </div>
            )}

            {sqlError && (
              <div className="bg-destructive/10 border border-destructive/20 text-destructive font-mono text-xs p-4 rounded-lg overflow-auto h-full max-h-[300px]">
                <p className="font-semibold mb-1">SQL 执行报错 (Error):</p>
                <pre className="whitespace-pre-wrap">{sqlError}</pre>
              </div>
            )}

            {sqlResult && (
              <div className="h-full flex flex-col">
                {sqlResult.type === "select" && sqlResult.columns && sqlResult.columns.length > 0 && (
                  <div className="border rounded-md overflow-auto max-w-full max-h-full bg-background relative flex-1 min-h-0">
                    <Table className="text-xs">
                      <TableHeader className="bg-muted/40 font-semibold sticky top-0 z-10">
                        <TableRow>
                          {sqlResult.columns.map((col) => (
                            <TableHead key={col} className="font-semibold text-foreground py-2">{col}</TableHead>
                          ))}
                        </TableRow>
                      </TableHeader>
                      <TableBody>
                        {sqlResult.results && sqlResult.results.length > 0 ? (
                          sqlResult.results.map((row, rIndex) => (
                            <TableRow key={rIndex} className="hover:bg-muted/10">
                              {sqlResult.columns!.map((col) => (
                                <TableCell key={col} className="font-mono py-1.5">
                                  <span className="truncate max-w-[240px] block" title={formatCellValue(row[col])}>
                                    {formatCellValue(row[col])}
                                  </span>
                                </TableCell>
                              ))}
                            </TableRow>
                          ))
                        ) : (
                          <TableRow>
                            <TableCell colSpan={sqlResult.columns.length} className="text-center py-10 text-muted-foreground">
                              查询结果为空
                            </TableCell>
                          </TableRow>
                        )}
                      </TableBody>
                    </Table>
                  </div>
                )}

                {sqlResult.type === "exec" && (
                  <div className="bg-primary/5 border border-primary/10 text-primary-foreground font-mono text-xs p-6 rounded-lg text-center my-auto">
                    <p className="text-foreground text-sm font-semibold">SQL 执行成功</p>
                    <p className="text-muted-foreground mt-2">
                      受影响行数: {sqlResult.affected_rows} 行，耗时 {sqlResult.execution_time_ms} 毫秒。
                    </p>
                  </div>
                )}
              </div>
            )}

            {!executingSQL && !sqlResult && !sqlError && (
              <div className="flex flex-col items-center justify-center h-full py-10 text-muted-foreground opacity-60">
                <Terminal className="size-8 mb-2" />
                <span className="text-xs">编辑器就绪，请在上方编写 SQL 并运行查看结果</span>
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}
