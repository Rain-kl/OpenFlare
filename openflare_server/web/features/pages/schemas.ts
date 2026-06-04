import { z } from 'zod';

export const pagesProjectSchema = z
  .object({
    name: z
      .string()
      .trim()
      .min(1, '请输入项目名称')
      .max(255, '项目名称不能超过 255 个字符'),
    slug: z
      .string()
      .trim()
      .max(255, '项目标识不能超过 255 个字符')
      .optional()
      .or(z.literal('')),
    description: z
      .string()
      .trim()
      .max(1000, '描述不能超过 1000 个字符')
      .optional()
      .or(z.literal('')),
    spa_fallback_enabled: z.boolean(),
    spa_fallback_path: z.string().trim(),
    api_proxy_enabled: z.boolean(),
    api_proxy_path: z.string().trim(),
    api_proxy_pass: z.string().trim(),
    api_proxy_rewrite: z.string().trim(),
    root_dir: z
      .string()
      .trim()
      .max(512, '根目录不能超过 512 个字符')
      .optional()
      .or(z.literal('')),
    entry_file: z
      .string()
      .trim()
      .min(1, '请输入入口文件')
      .max(512, '入口文件不能超过 512 个字符'),
  })
  .superRefine((data, ctx) => {
    if (data.spa_fallback_enabled) {
      if (!data.spa_fallback_path) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          path: ['spa_fallback_path'],
          message: '启用 SPA fallback 时必须输入回退路径',
        });
      } else if (!data.spa_fallback_path.startsWith('/')) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          path: ['spa_fallback_path'],
          message: '回退路径必须以 / 开头，例如 /index.html',
        });
      }
    }

    if (data.api_proxy_enabled) {
      if (!data.api_proxy_path) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          path: ['api_proxy_path'],
          message: '启用 API 反向代理时必须输入匹配路径',
        });
      } else if (!data.api_proxy_path.startsWith('/')) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          path: ['api_proxy_path'],
          message: '匹配路径必须以 / 开头，例如 /api',
        });
      }

      if (!data.api_proxy_pass) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          path: ['api_proxy_pass'],
          message: '启用 API 反向代理时必须输入后端服务地址',
        });
      } else if (!/^https?:\/\//i.test(data.api_proxy_pass)) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          path: ['api_proxy_pass'],
          message: '后端服务地址必须以 http:// 或 https:// 开头',
        });
      }
    }
  });

export type PagesProjectFormValues = z.infer<typeof pagesProjectSchema>;
