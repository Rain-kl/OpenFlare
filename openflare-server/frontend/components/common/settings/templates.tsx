"use client"

import {useState} from "react"
import {useMutation, useQuery, useQueryClient} from "@tanstack/react-query"
import {FileText, Loader2, Pencil, Plus, Trash2} from "lucide-react"

import {Button} from "@/components/ui/button"
import {Card, CardContent, CardDescription, CardHeader, CardTitle} from "@/components/ui/card"
import {Input} from "@/components/ui/input"
import {Label} from "@/components/ui/label"
import {Textarea} from "@/components/ui/textarea"
import {Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle} from "@/components/ui/dialog"
import services from "@/lib/services"
import type {Template} from "@/lib/services/admin/types"
import {toast} from "sonner"

export function TemplatesManager() {
  const queryClient = useQueryClient()
  const [modalOpen, setModalOpen] = useState(false)
  const [selectedTemplate, setSelectedTemplate] = useState<Template | null>(null)

  // Form states
  const [key, setKey] = useState("")
  const [name, setName] = useState("")
  const [type, setType] = useState("email")
  const [subject, setSubject] = useState("")
  const [content, setContent] = useState("")
  const [description, setDescription] = useState("")

  const templatesQuery = useQuery({
    queryKey: ["admin", "templates"],
    queryFn: () => services.adminTemplate.listTemplates(),
  })

  const createTemplateMutation = useMutation({
    mutationFn: async () => {
      await services.adminTemplate.createTemplate({
        key,
        name,
        type,
        subject,
        content,
        description,
      })
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["admin", "templates"] })
      toast.success("通知模板已成功创建")
      setModalOpen(false)
    },
    onError: (error: Error) => {
      toast.error(error.message || "创建模板失败")
    },
  })

  const updateTemplateMutation = useMutation({
    mutationFn: async (key: string) => {
      await services.adminTemplate.updateTemplate(key, {
        name,
        type,
        subject,
        content,
        description,
      })
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["admin", "templates"] })
      toast.success("通知模板已成功保存")
      setModalOpen(false)
    },
    onError: (error: Error) => {
      toast.error(error.message || "修改模板失败")
    },
  })

  const deleteTemplateMutation = useMutation({
    mutationFn: async (key: string) => {
      await services.adminTemplate.deleteTemplate(key)
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["admin", "templates"] })
      toast.success("通知模板已删除")
    },
    onError: (error: Error) => {
      toast.error(error.message || "删除模板失败")
    },
  })

  const handleOpenCreate = () => {
    setSelectedTemplate(null)
    setKey("")
    setName("")
    setType("email")
    setSubject("")
    setContent("")
    setDescription("")
    setModalOpen(true)
  }

  const handleOpenEdit = (tmpl: Template) => {
    setSelectedTemplate(tmpl)
    setKey(tmpl.key)
    setName(tmpl.name)
    setType(tmpl.type)
    setSubject(tmpl.subject || "")
    setContent(tmpl.content)
    setDescription(tmpl.description || "")
    setModalOpen(true)
  }

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (selectedTemplate) {
      updateTemplateMutation.mutate(selectedTemplate.key)
    } else {
      createTemplateMutation.mutate()
    }
  }

  const isPending = createTemplateMutation.isPending || updateTemplateMutation.isPending

  return (
    <div className="space-y-6">
      <Card className="border border-dashed shadow-sm">
        <CardHeader className="border-b border-dashed pb-4 flex flex-row items-center justify-between gap-4">
          <div className="flex items-center gap-2">
            <div className="p-1.5 rounded-lg bg-indigo-500/10 text-indigo-500">
              <FileText className="size-4" />
            </div>
            <div>
              <CardTitle className="text-base font-semibold">通知模板管理</CardTitle>
              <CardDescription className="text-xs">
                配置与编辑系统各类场景的通知邮件/短信模板，支持动态变量渲染
              </CardDescription>
            </div>
          </div>
          <Button
            type="button"
            size="sm"
            onClick={handleOpenCreate}
            variant="secondary"
          >
            <Plus className="mr-1.5 size-3.5" />
            新增模板
          </Button>
        </CardHeader>
        <CardContent className="pt-6 space-y-4">
          {templatesQuery.isPending ? (
            <div className="flex items-center justify-center p-8">
              <Loader2 className="size-6 animate-spin text-muted-foreground/50" />
            </div>
          ) : (templatesQuery.data ?? []).length > 0 ? (
            <div className="grid grid-cols-1 gap-4">
              {(templatesQuery.data ?? []).map((tmpl) => (
                <div
                  key={tmpl.id}
                  className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 rounded-xl border border-dashed p-4 bg-card hover:bg-muted/10 hover:border-indigo-500/30 transition-all duration-300 shadow-sm"
                >
                  <div className="space-y-1.5">
                    <div className="flex items-center gap-2 flex-wrap">
                      <span className="font-semibold text-sm text-foreground">{tmpl.name}</span>
                      <span className={`text-[10px] px-2 py-0.5 rounded-full border font-medium ${
                        tmpl.is_system
                          ? "bg-indigo-500/10 text-indigo-500 border-indigo-500/20"
                          : "bg-amber-500/10 text-amber-500 border-amber-500/20"
                      }`}>
                        {tmpl.is_system ? "系统内置" : "自定义"}
                      </span>
                      <span className="text-[10px] px-2 py-0.5 rounded-full border border-border/50 bg-muted/50 text-muted-foreground font-mono">
                        {tmpl.type.toUpperCase()}
                      </span>
                    </div>
                    <div className="text-xs text-muted-foreground">
                      标识符: <span className="font-mono text-indigo-500 bg-indigo-500/5 px-1.5 py-0.5 rounded">{tmpl.key}</span>
                      {tmpl.subject && ` · 主题: ${tmpl.subject}`}
                    </div>
                    {tmpl.description && (
                      <p className="text-xs text-muted-foreground/80 leading-relaxed max-w-xl">
                        {tmpl.description}
                      </p>
                    )}
                  </div>
                  <div className="flex items-center justify-end gap-2 shrink-0">
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon"
                      className="size-8 text-muted-foreground hover:text-indigo-500 hover:bg-indigo-500/10 rounded-lg transition-colors"
                      onClick={() => handleOpenEdit(tmpl)}
                    >
                      <Pencil className="size-4" />
                    </Button>
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon"
                      className="size-8 text-muted-foreground hover:text-rose-500 hover:bg-rose-500/10 rounded-lg transition-colors"
                      disabled={tmpl.is_system || deleteTemplateMutation.isPending}
                      onClick={() => {
                        if (window.confirm(`确定删除模板「${tmpl.name}」吗？`)) {
                          deleteTemplateMutation.mutate(tmpl.key)
                        }
                      }}
                    >
                      <Trash2 className="size-4" />
                    </Button>
                  </div>
                </div>
              ))}
            </div>
          ) : (
            <div className="rounded-xl border border-dashed border-border/50 px-4 py-8 text-center text-xs text-muted-foreground bg-muted/5 flex flex-col items-center justify-center gap-3">
              <span>暂无配置的通知模板，点击上方按钮新增</span>
              <Button
                type="button"
                size="sm"
                variant="outline"
                onClick={handleOpenCreate}
                className="border-dashed"
              >
                <Plus className="mr-1.5 size-3.5" />
                新增模板
              </Button>
            </div>
          )}
        </CardContent>
      </Card>

      <Dialog open={modalOpen} onOpenChange={setModalOpen}>
        <DialogContent className="max-w-xl border border-dashed">
          <DialogHeader>
            <DialogTitle className="text-base font-semibold">
              {selectedTemplate ? "编辑通知模板" : "新增通知模板"}
            </DialogTitle>
            <DialogDescription className="text-xs">
              配置模板渲染逻辑，支持 Go `text/template` 语法（例如 `{"{{.Code}}"}` 表示验证码变量）。
            </DialogDescription>
          </DialogHeader>

          <form onSubmit={handleSubmit} className="space-y-4">
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
              <div className="space-y-1.5">
                <Label htmlFor="tmpl_key" className="text-xs font-semibold">
                  模板标识符 (Key)
                </Label>
                <Input
                  id="tmpl_key"
                  type="text"
                  required
                  value={key}
                  onChange={(e) => setKey(e.target.value)}
                  placeholder="例如: login_email"
                  className="bg-card border-dashed text-xs"
                  disabled={!!selectedTemplate}
                />
                <p className="text-[10px] text-muted-foreground leading-normal">
                  模板的唯一标识，在代码中通过此 Key 调用。
                </p>
              </div>

              <div className="space-y-1.5">
                <Label htmlFor="tmpl_name" className="text-xs font-semibold">
                  模板名称
                </Label>
                <Input
                  id="tmpl_name"
                  type="text"
                  required
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  placeholder="例如: 登录验证码邮件"
                  className="bg-card border-dashed text-xs"
                />
                <p className="text-[10px] text-muted-foreground leading-normal">
                  用于后台识别该模板的可读名称。
                </p>
              </div>

              <div className="space-y-1.5">
                <Label htmlFor="tmpl_type" className="text-xs font-semibold">
                  模板类型
                </Label>
                <Input
                  id="tmpl_type"
                  type="text"
                  required
                  value={type}
                  onChange={(e) => setType(e.target.value)}
                  placeholder="email"
                  className="bg-card border-dashed text-xs"
                />
                <p className="text-[10px] text-muted-foreground leading-normal">
                  模板分类标识，目前支持 `email`。
                </p>
              </div>

              <div className="space-y-1.5">
                <Label htmlFor="tmpl_subject" className="text-xs font-semibold">
                  模板主题 (Subject)
                </Label>
                <Input
                  id="tmpl_subject"
                  type="text"
                  value={subject}
                  onChange={(e) => setSubject(e.target.value)}
                  placeholder="例如: OpenFlare 登录验证码"
                  className="bg-card border-dashed text-xs"
                />
                <p className="text-[10px] text-muted-foreground leading-normal">
                  邮件标题（类型为 email 时生效）。
                </p>
              </div>
            </div>

            <div className="space-y-1.5">
              <Label htmlFor="tmpl_description" className="text-xs font-semibold">
                模板说明与变量描述
              </Label>
              <Input
                id="tmpl_description"
                type="text"
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                placeholder="例如: 包含变量：{{.Code}}，5分钟内有效"
                className="bg-card border-dashed text-xs"
              />
            </div>

            <div className="space-y-1.5">
              <Label htmlFor="tmpl_content" className="text-xs font-semibold">
                模板正文内容 (Content)
              </Label>
              <Textarea
                id="tmpl_content"
                required
                value={content}
                onChange={(e) => setContent(e.target.value)}
                placeholder="<h3>正文标题</h3><p>内容段落，可用变量 {{.Code}}</p>"
                rows={8}
                className="bg-card border-dashed text-xs font-mono"
              />
            </div>

            <DialogFooter className="gap-2 sm:gap-0 border-t border-dashed pt-4">
              <Button
                type="button"
                variant="ghost"
                size="sm"
                onClick={() => setModalOpen(false)}
                disabled={isPending}
              >
                取消
              </Button>
              <Button
                type="submit"
                size="sm"
                disabled={isPending}
              >
                {isPending ? (
                  <>
                    <Loader2 className="mr-1.5 size-3.5 animate-spin" />
                    保存中...
                  </>
                ) : (
                  "保存配置"
                )}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </div>
  )
}
