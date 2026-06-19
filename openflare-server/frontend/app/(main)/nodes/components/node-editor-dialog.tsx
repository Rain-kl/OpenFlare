'use client';

import {useEffect} from 'react';
import {zodResolver} from '@hookform/resolvers/zod';
import {useForm} from 'react-hook-form';
import {z} from 'zod';

import {Button} from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import {Input} from '@/components/ui/input';
import {Label} from '@/components/ui/label';
import {Select, SelectContent, SelectItem, SelectTrigger, SelectValue,} from '@/components/ui/select';
import {Switch} from '@/components/ui/switch';
import type {NodeItem, NodeMutationPayload, NodeType} from '@/lib/services/openflare';

const nodeSchema = z
  .object({
    node_type: z.enum(['edge_node', 'tunnel_relay', 'tunnel_client']),
    name: z.string().trim().min(1, '请输入节点名称').max(255),
    ip: z.string(),
    ip_manual_override: z.boolean(),
    auto_update_enabled: z.boolean(),
    geo_manual_override: z.boolean(),
    geo_name: z.string(),
    geo_latitude: z.string(),
    geo_longitude: z.string(),
    relay_bind_port: z.string(),
    relay_vhost_http_port: z.string(),
  })
  .superRefine((value, context) => {
    if (value.ip_manual_override && !value.ip.trim()) {
      context.addIssue({
        code: z.ZodIssueCode.custom,
        path: ['ip'],
        message: '锁定 IP 时必须填写节点 IP',
      });
    }
    if (value.node_type === 'tunnel_relay') {
      const bindPort = Number(value.relay_bind_port);
      const vhostPort = Number(value.relay_vhost_http_port);
      if (!Number.isFinite(bindPort) || bindPort <= 0) {
        context.addIssue({
          code: z.ZodIssueCode.custom,
          path: ['relay_bind_port'],
          message: '请输入有效的绑定端口',
        });
      }
      if (!Number.isFinite(vhostPort) || vhostPort <= 0) {
        context.addIssue({
          code: z.ZodIssueCode.custom,
          path: ['relay_vhost_http_port'],
          message: '请输入有效的 VHost 端口',
        });
      }
    }
  });

type NodeFormValues = z.infer<typeof nodeSchema>;

const defaultForm: NodeFormValues = {
  node_type: 'edge_node',
  name: '',
  ip: '',
  ip_manual_override: false,
  auto_update_enabled: false,
  geo_manual_override: false,
  geo_name: '',
  geo_latitude: '',
  geo_longitude: '',
  relay_bind_port: '7000',
  relay_vhost_http_port: '8080',
};

function buildFormValues(node?: NodeItem | null): NodeFormValues {
  if (!node) return defaultForm;

  return {
    node_type: node.node_type,
    name: node.name,
    ip: node.ip,
    ip_manual_override: node.ip_manual_override,
    auto_update_enabled: node.auto_update_enabled,
    geo_manual_override: node.geo_manual_override,
    geo_name: node.geo_name,
    geo_latitude:
      node.geo_latitude === undefined || node.geo_latitude === null
        ? ''
        : String(node.geo_latitude),
    geo_longitude:
      node.geo_longitude === undefined || node.geo_longitude === null
        ? ''
        : String(node.geo_longitude),
    relay_bind_port: String(node.relay_bind_port || 7000),
    relay_vhost_http_port: String(node.relay_vhost_http_port || 8080),
  };
}

function toPayload(form: NodeFormValues): NodeMutationPayload {
  const base: NodeMutationPayload = {
    node_type: form.node_type,
    name: form.name.trim(),
    ip: form.ip.trim(),
    ip_manual_override: form.ip_manual_override,
    auto_update_enabled: form.auto_update_enabled,
    geo_manual_override: form.geo_manual_override,
    geo_name: form.geo_manual_override ? form.geo_name.trim() : '',
    geo_latitude: form.geo_manual_override && form.geo_latitude
      ? Number(form.geo_latitude)
      : null,
    geo_longitude: form.geo_manual_override && form.geo_longitude
      ? Number(form.geo_longitude)
      : null,
  };

  if (form.node_type === 'tunnel_relay') {
    return {
      ...base,
      relay_bind_port: Number(form.relay_bind_port),
      relay_vhost_http_port: Number(form.relay_vhost_http_port),
      relay_web_server_enabled: true,
    };
  }

  return base;
}

export function NodeEditorDialog({
  open,
  node,
  submitting,
  onClose,
  onSubmit,
}: {
  open: boolean;
  node: NodeItem | null;
  submitting: boolean;
  onClose: () => void;
  onSubmit: (payload: NodeMutationPayload) => Promise<void>;
}) {
  const form = useForm<NodeFormValues>({
    resolver: zodResolver(nodeSchema),
    defaultValues: defaultForm,
  });

  useEffect(() => {
    if (open) {
      form.reset(buildFormValues(node));
    }
  }, [form, open, node]);

  const nodeType = form.watch('node_type');
  const ipManualOverride = form.watch('ip_manual_override');
  const geoManualOverride = form.watch('geo_manual_override');

  const handleSubmit = form.handleSubmit(async (values) => {
    await onSubmit(toPayload(values));
  });

  return (
    <Dialog open={open} onOpenChange={(next) => !next && onClose()}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{node ? '编辑节点' : '新增节点'}</DialogTitle>
          <DialogDescription>
            预创建节点后可在详情页查看专属 Token 与部署命令。
          </DialogDescription>
        </DialogHeader>

        <form onSubmit={(event) => void handleSubmit(event)} className="space-y-4">
          <div className="space-y-2">
            <Label>节点类型</Label>
            <Select
              value={nodeType}
              disabled={Boolean(node)}
              onValueChange={(value) =>
                form.setValue('node_type', value as NodeType)
              }
            >
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="edge_node">Edge 节点</SelectItem>
                <SelectItem value="tunnel_relay">Relay 节点</SelectItem>
                <SelectItem value="tunnel_client">Tunnel 节点</SelectItem>
              </SelectContent>
            </Select>
          </div>

          <div className="space-y-2">
            <Label htmlFor="node-name">节点名称</Label>
            <Input
              id="node-name"
              placeholder="例如 edge-hk-01"
              {...form.register('name')}
            />
            {form.formState.errors.name ? (
              <p className="text-xs text-destructive">
                {form.formState.errors.name.message}
              </p>
            ) : null}
          </div>

          <div className="space-y-2">
            <Label htmlFor="node-ip">节点 IP</Label>
            <Input
              id="node-ip"
              placeholder="可选，接入后自动上报"
              {...form.register('ip')}
            />
            {form.formState.errors.ip ? (
              <p className="text-xs text-destructive">
                {form.formState.errors.ip.message}
              </p>
            ) : null}
          </div>

          <div className="flex items-center justify-between rounded-lg border px-3 py-2">
            <div>
              <p className="text-sm font-medium">锁定 IP</p>
              <p className="text-xs text-muted-foreground">启用后管理端不再自动覆盖 IP</p>
            </div>
            <Switch
              checked={ipManualOverride}
              onCheckedChange={(checked) =>
                form.setValue('ip_manual_override', checked)
              }
            />
          </div>

          <div className="flex items-center justify-between rounded-lg border px-3 py-2">
            <div>
              <p className="text-sm font-medium">自动更新 Agent</p>
              <p className="text-xs text-muted-foreground">启用后节点将自动拉取正式版更新</p>
            </div>
            <Switch
              checked={form.watch('auto_update_enabled')}
              onCheckedChange={(checked) =>
                form.setValue('auto_update_enabled', checked)
              }
            />
          </div>

          {nodeType === 'tunnel_relay' ? (
            <div className="grid gap-3 sm:grid-cols-2">
              <div className="space-y-2">
                <Label htmlFor="relay-bind-port">绑定端口</Label>
                <Input
                  id="relay-bind-port"
                  {...form.register('relay_bind_port')}
                />
                {form.formState.errors.relay_bind_port ? (
                  <p className="text-xs text-destructive">
                    {form.formState.errors.relay_bind_port.message}
                  </p>
                ) : null}
              </div>
              <div className="space-y-2">
                <Label htmlFor="relay-vhost-port">VHost 端口</Label>
                <Input
                  id="relay-vhost-port"
                  {...form.register('relay_vhost_http_port')}
                />
                {form.formState.errors.relay_vhost_http_port ? (
                  <p className="text-xs text-destructive">
                    {form.formState.errors.relay_vhost_http_port.message}
                  </p>
                ) : null}
              </div>
            </div>
          ) : null}

          <div className="flex items-center justify-between rounded-lg border px-3 py-2">
            <div>
              <p className="text-sm font-medium">手动地图点位</p>
              <p className="text-xs text-muted-foreground">用于总览地图展示</p>
            </div>
            <Switch
              checked={geoManualOverride}
              onCheckedChange={(checked) =>
                form.setValue('geo_manual_override', checked)
              }
            />
          </div>

          {geoManualOverride ? (
            <div className="grid gap-3 sm:grid-cols-3">
              <div className="space-y-2 sm:col-span-3">
                <Label htmlFor="geo-name">位置名称</Label>
                <Input id="geo-name" {...form.register('geo_name')} />
              </div>
              <div className="space-y-2">
                <Label htmlFor="geo-lat">纬度</Label>
                <Input id="geo-lat" {...form.register('geo_latitude')} />
              </div>
              <div className="space-y-2 sm:col-span-2">
                <Label htmlFor="geo-lng">经度</Label>
                <Input id="geo-lng" {...form.register('geo_longitude')} />
              </div>
            </div>
          ) : null}

          <DialogFooter>
            <Button type="button" variant="outline" onClick={onClose} disabled={submitting}>
              取消
            </Button>
            <Button type="submit" disabled={submitting}>
              {submitting ? '保存中...' : node ? '保存修改' : '新增节点'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}