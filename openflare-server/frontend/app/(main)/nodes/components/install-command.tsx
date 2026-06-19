'use client';

import {useEffect, useMemo, useState} from 'react';
import {Copy} from 'lucide-react';
import {toast} from 'sonner';

import {Button} from '@/components/ui/button';
import {Card, CardContent, CardDescription, CardHeader, CardTitle} from '@/components/ui/card';
import {Input} from '@/components/ui/input';
import {Label} from '@/components/ui/label';
import type {NodeItem} from '@/lib/services/openflare';

import {
  buildRelayDockerInstallCommand,
  buildRelayInstallCommand,
  buildTunnelDockerInstallCommand,
  buildTunnelInstallCommand,
  getServerUrl,
} from './node-utils';

type InstallVariant = 'relay' | 'tunnel';

const variantMeta: Record<
  InstallVariant,
  {
    title: string;
    description: string;
    tokenLabel: string;
    scriptLabel: string;
    dockerLabel: string;
  }
> = {
  relay: {
    title: '中继部署命令',
    description: '使用 Discovery Token 将 frps 中继节点接入控制端。',
    tokenLabel: 'Discovery Token',
    scriptLabel: '一键脚本部署 (Linux / macOS)',
    dockerLabel: 'Docker 容器部署',
  },
  tunnel: {
    title: '隧道部署命令',
    description: '使用 Tunnel Token 将 openflared 客户端接入控制端。',
    tokenLabel: 'Tunnel Token',
    scriptLabel: '一键脚本部署 (Linux / macOS)',
    dockerLabel: 'Docker 容器部署',
  },
};

export function InstallCommand({
  node,
  variant,
}: {
  node: NodeItem;
  variant: InstallVariant;
}) {
  const [serverUrl, setServerUrl] = useState('');
  const meta = variantMeta[variant];

  useEffect(() => {
    if (typeof window !== 'undefined' && !serverUrl) {
      setServerUrl(window.location.origin);
    }
  }, [serverUrl]);

  const normalizedServerUrl = getServerUrl(serverUrl);
  const scriptCommand = useMemo(() => {
    if (!normalizedServerUrl || !node.access_token) {
      return '';
    }
    return variant === 'relay'
      ? buildRelayInstallCommand(normalizedServerUrl, node.access_token)
      : buildTunnelInstallCommand(normalizedServerUrl, node.access_token);
  }, [normalizedServerUrl, node.access_token, variant]);

  const dockerCommand = useMemo(() => {
    if (!normalizedServerUrl || !node.access_token) {
      return '';
    }
    return variant === 'relay'
      ? buildRelayDockerInstallCommand(normalizedServerUrl, node.access_token)
      : buildTunnelDockerInstallCommand(normalizedServerUrl, node.access_token);
  }, [normalizedServerUrl, node.access_token, variant]);

  const handleCopy = async (value: string, message: string) => {
    try {
      await navigator.clipboard.writeText(value);
      toast.success(message);
    } catch {
      toast.error('复制失败，请手动选择命令文本。');
    }
  };

  return (
    <Card className="border-dashed shadow-none">
      <CardHeader className="flex-row items-start justify-between space-y-0 gap-4">
        <div>
          <CardTitle className="text-base font-semibold">{meta.title}</CardTitle>
          <CardDescription>{meta.description}</CardDescription>
        </div>
        <div className="flex flex-wrap gap-2">
          {scriptCommand ? (
            <Button
              variant="secondary"
              size="sm"
              className="h-7 text-xs"
              onClick={() => void handleCopy(scriptCommand, '部署脚本命令已复制')}
            >
              <Copy className="size-3.5 mr-1" />
              复制脚本
            </Button>
          ) : null}
          {dockerCommand ? (
            <Button
              variant="secondary"
              size="sm"
              className="h-7 text-xs"
              onClick={() => void handleCopy(dockerCommand, 'Docker 部署命令已复制')}
            >
              <Copy className="size-3.5 mr-1" />
              复制 Docker
            </Button>
          ) : null}
        </div>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="grid gap-4 md:grid-cols-2">
          <div className="rounded-lg border px-3 py-3">
            <p className="text-xs text-muted-foreground">Node ID</p>
            <p className="mt-1 text-sm break-all font-medium">{node.node_id}</p>
          </div>
          <div className="rounded-lg border px-3 py-3">
            <p className="text-xs text-muted-foreground">{meta.tokenLabel}</p>
            <p className="mt-1 text-sm break-all font-medium">{node.access_token || '暂无'}</p>
          </div>
        </div>

        <div className="space-y-2">
          <Label htmlFor={`server-url-${variant}`}>Server URL（控制端地址）</Label>
          <Input
            id={`server-url-${variant}`}
            value={serverUrl}
            onChange={(event) => setServerUrl(event.target.value)}
            placeholder="https://openflare.example.com"
          />
          <p className="text-xs text-muted-foreground">
            部署脚本和容器将使用此地址连接 OpenFlare 控制端。
          </p>
        </div>

        {!node.access_token ? (
          <p className="text-sm text-muted-foreground">节点尚未生成接入 Token，请稍后刷新。</p>
        ) : !normalizedServerUrl ? (
          <p className="text-sm text-muted-foreground">请填写有效的 Server URL 以生成部署命令。</p>
        ) : (
          <>
            <div className="space-y-2">
              <p className="text-sm font-medium">{meta.scriptLabel}</p>
              <pre className="overflow-x-auto rounded-lg border bg-muted/40 p-3 text-xs whitespace-pre-wrap">
                {scriptCommand}
              </pre>
            </div>
            <div className="space-y-2">
              <p className="text-sm font-medium">{meta.dockerLabel}</p>
              <pre className="overflow-x-auto rounded-lg border bg-muted/40 p-3 text-xs whitespace-pre-wrap">
                {dockerCommand}
              </pre>
            </div>
          </>
        )}
      </CardContent>
    </Card>
  );
}