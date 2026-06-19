'use client';

import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import {Button} from '@/components/ui/button';
import type {WAFIPGroupAutoTestResult} from '@/lib/services/openflare';

interface IPGroupTestDialogProps {
  open: boolean;
  loading: boolean;
  result: WAFIPGroupAutoTestResult | null;
  onOpenChange: (open: boolean) => void;
}

export function IPGroupTestDialog({
  open,
  loading,
  result,
  onOpenChange,
}: IPGroupTestDialogProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-2xl">
        <DialogHeader>
          <DialogTitle>自动规则测试结果</DialogTitle>
          <DialogDescription>
            基于当前自动配置 JSON 对请求日志进行回看测试。
          </DialogDescription>
        </DialogHeader>

        {loading ? (
          <p className="text-sm text-muted-foreground py-6 text-center">测试中...</p>
        ) : result ? (
          <div className="space-y-4">
            <div className="rounded-lg border border-dashed p-4 text-sm">
              <p>
                回看 {result.lookback_minutes} 分钟 · 规则 {result.rule_count} 条 · 命中{' '}
                {result.matched_count} 个 IP
              </p>
              <p className="text-xs text-muted-foreground mt-1">
                测试时间：{new Date(result.tested_at).toLocaleString()}
              </p>
            </div>
            {result.matched_count > 0 ? (
              <pre className="max-h-64 overflow-auto rounded-lg border bg-muted/40 p-3 text-xs whitespace-pre-wrap break-all">
                {result.matched_ips.join('\n')}
              </pre>
            ) : (
              <p className="text-sm text-muted-foreground">当前没有匹配到任何 IP。</p>
            )}
          </div>
        ) : (
          <p className="text-sm text-muted-foreground py-6 text-center">暂无测试结果。</p>
        )}

        <DialogFooter>
          <Button type="button" onClick={() => onOpenChange(false)}>
            关闭
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
