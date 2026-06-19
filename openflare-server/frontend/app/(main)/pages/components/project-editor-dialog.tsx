"use client"

import {useEffect} from "react"
import {useMutation, useQueryClient} from "@tanstack/react-query"
import {zodResolver} from "@hookform/resolvers/zod"
import {useForm} from "react-hook-form"
import {Loader2} from "lucide-react"
import {z} from "zod"
import {toast} from "sonner"

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
import {Switch} from "@/components/ui/switch"
import {Textarea} from "@/components/ui/textarea"
import {type PagesProject, PagesService} from "@/lib/services/openflare"

import {projectQueryKey, projectsQueryKey} from "./pages-utils"

const pagesProjectSchema = z
  .object({
    name: z.string().trim().min(1, "请输入项目名称").max(255),
    slug: z.string().trim().max(255).optional().or(z.literal("")),
    description: z.string().trim().max(1000).optional().or(z.literal("")),
    spa_fallback_enabled: z.boolean(),
    spa_fallback_path: z.string().trim(),
    api_proxy_enabled: z.boolean(),
    api_proxy_path: z.string().trim(),
    api_proxy_pass: z.string().trim(),
    api_proxy_rewrite: z.string().trim(),
    root_dir: z.string().trim().max(512).optional().or(z.literal("")),
    entry_file: z.string().trim().min(1, "请输入入口文件").max(512),
  })
  .superRefine((data, ctx) => {
    if (data.spa_fallback_enabled && !data.spa_fallback_path.startsWith("/")) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        path: ["spa_fallback_path"],
        message: "回退路径必须以 / 开头",
      })
    }
    if (data.api_proxy_enabled) {
      if (!data.api_proxy_path.startsWith("/")) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          path: ["api_proxy_path"],
          message: "匹配路径必须以 / 开头",
        })
      }
      if (!/^https?:\/\//i.test(data.api_proxy_pass)) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          path: ["api_proxy_pass"],
          message: "后端地址必须以 http:// 或 https:// 开头",
        })
      }
    }
  })

type PagesProjectFormValues = z.infer<typeof pagesProjectSchema>

function toFormValues(project?: PagesProject | null): PagesProjectFormValues {
  if (!project) {
    return {
      name: "",
      slug: "",
      description: "",
      spa_fallback_enabled: false,
      spa_fallback_path: "/index.html",
      api_proxy_enabled: false,
      api_proxy_path: "",
      api_proxy_pass: "",
      api_proxy_rewrite: "",
      root_dir: "",
      entry_file: "index.html",
    }
  }
  return {
    name: project.name,
    slug: project.slug,
    description: project.description || "",
    spa_fallback_enabled: project.spa_fallback_enabled,
    spa_fallback_path: project.spa_fallback_path,
    api_proxy_enabled: project.api_proxy_enabled || false,
    api_proxy_path: project.api_proxy_path || "",
    api_proxy_pass: project.api_proxy_pass || "",
    api_proxy_rewrite: project.api_proxy_rewrite || "",
    root_dir: project.root_dir || "",
    entry_file: project.entry_file || "index.html",
  }
}

interface ProjectEditorDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  project?: PagesProject | null
}

export function ProjectEditorDialog({
  open,
  onOpenChange,
  project,
}: ProjectEditorDialogProps) {
  const queryClient = useQueryClient()
  const form = useForm<PagesProjectFormValues>({
    resolver: zodResolver(pagesProjectSchema),
    defaultValues: toFormValues(project),
  })

  useEffect(() => {
    if (open) form.reset(toFormValues(project))
  }, [form, project, open])

  const mutation = useMutation({
    mutationFn: async (values: PagesProjectFormValues) => {
      const payload = {
        name: values.name.trim(),
        slug: values.slug?.trim() || "",
        description: values.description?.trim() || "",
        enabled: project ? project.enabled : true,
        spa_fallback_enabled: values.spa_fallback_enabled,
        spa_fallback_path: values.spa_fallback_enabled
          ? values.spa_fallback_path.trim()
          : project?.spa_fallback_path || "/index.html",
        api_proxy_enabled: values.api_proxy_enabled,
        api_proxy_path: values.api_proxy_enabled ? values.api_proxy_path.trim() : "",
        api_proxy_pass: values.api_proxy_enabled ? values.api_proxy_pass.trim() : "",
        api_proxy_rewrite: values.api_proxy_enabled
          ? values.api_proxy_rewrite.trim()
          : "",
        root_dir: values.root_dir?.trim() || "",
        entry_file: values.entry_file.trim(),
      }
      return project
        ? PagesService.updateProject(project.id, payload)
        : PagesService.createProject(payload)
    },
    onSuccess: async () => {
      toast.success(project ? "项目已更新" : "项目已创建")
      await queryClient.invalidateQueries({ queryKey: projectsQueryKey })
      if (project) {
        await queryClient.invalidateQueries({
          queryKey: projectQueryKey(project.id),
        })
      }
      onOpenChange(false)
    },
    onError: (error) => {
      toast.error(error instanceof Error ? error.message : "保存失败")
    },
  })

  const spaEnabled = form.watch("spa_fallback_enabled")
  const apiEnabled = form.watch("api_proxy_enabled")

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-2xl max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>{project ? "编辑 Pages 项目" : "新建 Pages 项目"}</DialogTitle>
          <DialogDescription>
            配置静态站点托管参数，上传部署包后在代理规则中选择 Pages 上游。
          </DialogDescription>
        </DialogHeader>

        <form
          id="pages-project-form"
          className="space-y-4"
          onSubmit={form.handleSubmit((values) => mutation.mutate(values))}
        >
          <div className="grid gap-4 md:grid-cols-2">
            <div className="space-y-1.5">
              <Label htmlFor="name">项目名称</Label>
              <Input id="name" {...form.register("name")} />
              {form.formState.errors.name ? (
                <p className="text-xs text-destructive">{form.formState.errors.name.message}</p>
              ) : null}
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="slug">项目标识</Label>
              <Input id="slug" placeholder="留空自动生成" {...form.register("slug")} />
            </div>
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="description">描述</Label>
            <Textarea id="description" rows={2} {...form.register("description")} />
          </div>

          <div className="grid gap-4 md:grid-cols-2">
            <div className="space-y-1.5">
              <Label htmlFor="entry_file">入口文件</Label>
              <Input id="entry_file" {...form.register("entry_file")} />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="root_dir">根目录</Label>
              <Input id="root_dir" placeholder="可选" {...form.register("root_dir")} />
            </div>
          </div>

          <div className="flex items-center justify-between rounded-lg border border-dashed px-4 py-3">
            <div>
              <p className="text-sm font-medium">SPA fallback</p>
              <p className="text-xs text-muted-foreground">未命中静态文件时回退到指定路径</p>
            </div>
            <Switch
              checked={spaEnabled}
              onCheckedChange={(checked) =>
                form.setValue("spa_fallback_enabled", checked)
              }
            />
          </div>
          {spaEnabled ? (
            <div className="space-y-1.5">
              <Label htmlFor="spa_fallback_path">回退路径</Label>
              <Input id="spa_fallback_path" {...form.register("spa_fallback_path")} />
            </div>
          ) : null}

          <div className="flex items-center justify-between rounded-lg border border-dashed px-4 py-3">
            <div>
              <p className="text-sm font-medium">API 反向代理</p>
              <p className="text-xs text-muted-foreground">为静态站点附加 API 反代规则</p>
            </div>
            <Switch
              checked={apiEnabled}
              onCheckedChange={(checked) =>
                form.setValue("api_proxy_enabled", checked)
              }
            />
          </div>
          {apiEnabled ? (
            <div className="grid gap-4 md:grid-cols-2">
              <div className="space-y-1.5">
                <Label htmlFor="api_proxy_path">匹配路径</Label>
                <Input id="api_proxy_path" {...form.register("api_proxy_path")} />
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="api_proxy_pass">后端地址</Label>
                <Input id="api_proxy_pass" {...form.register("api_proxy_pass")} />
              </div>
              <div className="space-y-1.5 md:col-span-2">
                <Label htmlFor="api_proxy_rewrite">重写规则</Label>
                <Input id="api_proxy_rewrite" {...form.register("api_proxy_rewrite")} />
              </div>
            </div>
          ) : null}
        </form>

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            取消
          </Button>
          <Button type="submit" form="pages-project-form" disabled={mutation.isPending}>
            {mutation.isPending ? (
              <>
                <Loader2 className="size-4 animate-spin mr-1" />
                保存中...
              </>
            ) : project ? (
              "保存修改"
            ) : (
              "创建项目"
            )}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
