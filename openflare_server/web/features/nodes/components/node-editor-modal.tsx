'use client';

import { zodResolver } from '@hookform/resolvers/zod';
import { useMemo, useEffect } from 'react';
import { useForm, useWatch } from 'react-hook-form';
import { z } from 'zod';

import { AppModal } from '@/components/ui/app-modal';
import worldGeoJson from '@/features/dashboard/data/world-geo.json';
import type { NodeItem, NodeMutationPayload } from '@/features/nodes/types';
import {
  PrimaryButton,
  ResourceField,
  ResourceInput,
  ResourceSelect,
  SecondaryButton,
  ToggleField,
} from '@/features/shared/components/resource-primitives';

type GeoJsonGeometry = {
  type: string;
  coordinates: unknown;
};

type GeoJsonFeature = {
  geometry?: GeoJsonGeometry;
  properties?: {
    name?: string;
  };
};

type RegionOption = {
  label: string;
  latitude: number;
  longitude: number;
};

const nodeEditorSchema = z
  .object({
    type: z.enum(['edge_node', 'tunnel_relay']),
    name: z
      .string()
      .trim()
      .min(1, '请输入节点名')
      .max(128, '节点名不能超过 128 个字符'),
    ip: z.string().trim().max(64, '节点 IP 不能超过 64 个字符'),
    ip_manual_override: z.boolean(),
    auto_update_enabled: z.boolean(),
    relay_bind_port: z.string().trim(),
    relay_client_access_addr: z.string().trim(),
    relay_client_proxy_url: z.string().trim(),
    geo_manual_override: z.boolean(),
    geo_region: z.string(),
    geo_name: z.string().trim().max(128, '位置名不能超过 128 个字符'),
    geo_latitude: z.string().trim(),
    geo_longitude: z.string().trim(),
  })
  .superRefine((values, ctx) => {
    if (values.ip_manual_override && values.ip.trim() === '') {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        path: ['ip'],
        message: '锁定节点 IP 时必须填写节点 IP',
      });
    }

    if (values.type === 'tunnel_relay') {
      const port = Number(values.relay_bind_port);
      if (Number.isNaN(port) || port < 1 || port > 65535) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          path: ['relay_bind_port'],
          message: '请填写正确的端口号 (1-65535)',
        });
      }
    }

    if (!values.geo_manual_override) {
      return;
    }

    const hasLatitude = values.geo_latitude !== '';
    const hasLongitude = values.geo_longitude !== '';

    if (hasLatitude !== hasLongitude) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        path: ['geo_latitude'],
        message: '纬度和经度需要同时填写',
      });
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        path: ['geo_longitude'],
        message: '纬度和经度需要同时填写',
      });
      return;
    }

    if (hasLatitude) {
      const latitude = Number(values.geo_latitude);
      if (Number.isNaN(latitude) || latitude < -90 || latitude > 90) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          path: ['geo_latitude'],
          message: '纬度必须在 -90 到 90 之间',
        });
      }
    }

    if (hasLongitude) {
      const longitude = Number(values.geo_longitude);
      if (Number.isNaN(longitude) || longitude < -180 || longitude > 180) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          path: ['geo_longitude'],
          message: '经度必须在 -180 到 180 之间',
        });
      }
    }
  });

type NodeEditorValues = z.infer<typeof nodeEditorSchema>;

const defaultValues: NodeEditorValues = {
  type: 'edge_node',
  name: '',
  ip: '',
  ip_manual_override: false,
  auto_update_enabled: false,
  relay_bind_port: '7000',
  relay_client_access_addr: '',
  relay_client_proxy_url: '',
  geo_manual_override: false,
  geo_region: '',
  geo_name: '',
  geo_latitude: '',
  geo_longitude: '',
};

function collectPoints(value: unknown, points: Array<[number, number]>) {
  if (!Array.isArray(value)) {
    return;
  }
  if (
    value.length >= 2 &&
    typeof value[0] === 'number' &&
    typeof value[1] === 'number'
  ) {
    points.push([value[0], value[1]]);
    return;
  }
  for (const item of value) {
    collectPoints(item, points);
  }
}

function getRegionCenter(feature: GeoJsonFeature) {
  const points: Array<[number, number]> = [];
  collectPoints(feature.geometry?.coordinates, points);
  if (points.length === 0) {
    return null;
  }

  let minLongitude = Number.POSITIVE_INFINITY;
  let maxLongitude = Number.NEGATIVE_INFINITY;
  let minLatitude = Number.POSITIVE_INFINITY;
  let maxLatitude = Number.NEGATIVE_INFINITY;

  for (const [longitude, latitude] of points) {
    minLongitude = Math.min(minLongitude, longitude);
    maxLongitude = Math.max(maxLongitude, longitude);
    minLatitude = Math.min(minLatitude, latitude);
    maxLatitude = Math.max(maxLatitude, latitude);
  }

  return {
    longitude: Number(((minLongitude + maxLongitude) / 2).toFixed(4)),
    latitude: Number(((minLatitude + maxLatitude) / 2).toFixed(4)),
  };
}

function buildRegionOptions() {
  const features = ((worldGeoJson as { features?: GeoJsonFeature[] })
    .features ?? []) as GeoJsonFeature[];
  const options = new Map<string, RegionOption>();

  for (const feature of features) {
    const label = feature.properties?.name?.trim();
    if (!label || options.has(label)) {
      continue;
    }
    const center = getRegionCenter(feature);
    if (!center) {
      continue;
    }
    options.set(label, {
      label,
      latitude: center.latitude,
      longitude: center.longitude,
    });
  }

  return Array.from(options.values()).sort((a, b) =>
    a.label.localeCompare(b.label),
  );
}

function buildFormValues(node?: Partial<NodeItem> | null): NodeEditorValues {
  if (!node) {
    return defaultValues;
  }

  return {
    type: node.node_type ?? 'edge_node',
    name: node.name ?? '',
    ip: node.ip ?? '',
    ip_manual_override: node.ip_manual_override ?? false,
    auto_update_enabled: node.auto_update_enabled ?? false,
    relay_bind_port: String(node.relay_bind_port ?? 7000),
    relay_client_access_addr: node.relay_client_access_addr ?? '',
    relay_client_proxy_url: node.relay_client_proxy_url ?? '',
    geo_manual_override: node.geo_manual_override ?? false,
    geo_region: node.geo_manual_override ? (node.geo_name ?? '') : '',
    geo_name: node.geo_name ?? '',
    geo_latitude:
      node.geo_latitude === undefined || node.geo_latitude === null
        ? ''
        : String(node.geo_latitude),
    geo_longitude:
      node.geo_longitude === undefined || node.geo_longitude === null
        ? ''
        : String(node.geo_longitude),
  };
}

function toPayload(values: NodeEditorValues): NodeMutationPayload {
  const basePayload = {
    node_type: values.type,
    name: values.name.trim(),
    ip: values.ip.trim(),
    ip_manual_override: values.ip_manual_override,
    auto_update_enabled: values.auto_update_enabled,
    relay_bind_port: values.type === 'tunnel_relay' ? Number(values.relay_bind_port) : undefined,
    relay_client_access_addr: values.type === 'tunnel_relay' ? values.relay_client_access_addr.trim() : undefined,
    relay_client_proxy_url: values.type === 'tunnel_relay' ? values.relay_client_proxy_url.trim() : undefined,
  };

  if (!values.geo_manual_override) {
    return {
      ...basePayload,
      geo_manual_override: false,
      geo_name: '',
      geo_latitude: null,
      geo_longitude: null,
    };
  }

  return {
    ...basePayload,
    geo_manual_override: true,
    geo_name: values.geo_name.trim(),
    geo_latitude:
      values.geo_latitude.trim() === '' ? null : Number(values.geo_latitude),
    geo_longitude:
      values.geo_longitude.trim() === '' ? null : Number(values.geo_longitude),
  };
}

export function NodeEditorModal({
  isOpen,
  node,
  isSubmitting,
  title,
  description,
  submitLabel,
  onClose,
  onSubmit,
}: {
  isOpen: boolean;
  node?: Partial<NodeItem> | null;
  isSubmitting: boolean;
  title: string;
  description: string;
  submitLabel: string;
  onClose: () => void;
  onSubmit: (payload: NodeMutationPayload) => void;
}) {
  const form = useForm<NodeEditorValues>({
    resolver: zodResolver(nodeEditorSchema),
    defaultValues,
  });

  const watchedType = useWatch({
    control: form.control,
    name: 'type',
  });
  const watchedAutoUpdate = useWatch({
    control: form.control,
    name: 'auto_update_enabled',
  });
  const watchedIPManualOverride = useWatch({
    control: form.control,
    name: 'ip_manual_override',
  });
  const watchedGeoManualOverride = useWatch({
    control: form.control,
    name: 'geo_manual_override',
  });
  const regionOptions = useMemo(() => buildRegionOptions(), []);

  useEffect(() => {
    if (!isOpen) {
      return;
    }
    form.reset(buildFormValues(node));
  }, [form, isOpen, node]);

  const ipField = form.register('ip');
  const handleSubmit = form.handleSubmit((values) => {
    onSubmit(toPayload(values));
  });

  return (
    <AppModal
      isOpen={isOpen}
      onClose={onClose}
      title={title}
      description={description}
      footer={
        <div className="flex flex-wrap justify-end gap-3">
          <SecondaryButton
            type="button"
            onClick={onClose}
            disabled={isSubmitting}
          >
            取消
          </SecondaryButton>
          <PrimaryButton
            type="submit"
            form="node-editor-form"
            disabled={isSubmitting}
          >
            {isSubmitting ? '保存中...' : submitLabel}
          </PrimaryButton>
        </div>
      }
    >
      <form id="node-editor-form" className="space-y-5" onSubmit={handleSubmit}>
        <ResourceField
          label="节点类型"
          hint="边缘节点用于常规的网站代理和缓存；隧道中继节点用于内网穿透的流量中转服务器。"
        >
          <ResourceSelect {...form.register('type')} disabled={!!node}>
            <option value="edge_node">边缘节点 (Edge Node)</option>
            <option value="tunnel_relay">隧道中继节点 (Tunnel Relay)</option>
          </ResourceSelect>
        </ResourceField>

        <ResourceField
          label="节点名"
          hint="示例：shanghai-edge-1"
          error={form.formState.errors.name?.message}
        >
          <ResourceInput
            placeholder="shanghai-edge-1"
            {...form.register('name')}
          />
        </ResourceField>

        {watchedType === 'tunnel_relay' && (
          <div className="space-y-5 rounded-lg border border-border p-4 bg-muted/50">
            <h4 className="text-sm font-medium">中继配置 (Relay Config)</h4>
            <ResourceField
              label="中继绑定端口 (Bind Port)"
              hint="中继服务端在此端口监听并接受 Flared 客户端连接。"
              error={form.formState.errors.relay_bind_port?.message}
            >
              <ResourceInput placeholder="7000" {...form.register('relay_bind_port')} />
            </ResourceField>
            
            <ResourceField
              label="客户端接入地址 (Client Access Addr)"
              hint="可选，如果不填默认使用节点的 IP + 绑定端口。"
              error={form.formState.errors.relay_client_access_addr?.message}
            >
              <ResourceInput placeholder="例如: relay.example.com:7000" {...form.register('relay_client_access_addr')} />
            </ResourceField>

            <ResourceField
              label="代理地址 (Proxy URL)"
              hint="可选，下发给客户端。当客户端连接中继需要经过 HTTP 代理时使用。"
              error={form.formState.errors.relay_client_proxy_url?.message}
            >
              <ResourceInput placeholder="例如: http://10.0.0.1:3128" {...form.register('relay_client_proxy_url')} />
            </ResourceField>
          </div>
        )}

        <ResourceField
          label="节点 IP"
          hint={
            watchedIPManualOverride
              ? '已锁定为指定值，不会被更新。'
              : '留空或关闭锁定时，等待 Agent 自动更新。'
          }
          error={form.formState.errors.ip?.message}
        >
          <ResourceInput
            placeholder="203.0.113.10"
            {...ipField}
            onChange={(event) => {
              ipField.onChange(event);
              const nextIP = event.target.value.trim();
              const previousIP = (node?.ip ?? '').trim();
              if (nextIP !== '' && nextIP !== previousIP) {
                form.setValue('ip_manual_override', true, {
                  shouldDirty: true,
                  shouldValidate: true,
                });
              }
            }}
          />
        </ResourceField>

        <ToggleField
          label="锁定节点 IP"
          description="开启后以管理端填写的 IP 为准，Agent 后续上报不会覆盖该字段。"
          checked={watchedIPManualOverride}
          onChange={(checked) =>
            form.setValue('ip_manual_override', checked, {
              shouldDirty: true,
              shouldValidate: true,
            })
          }
        />

        {watchedType === 'edge_node' && (
          <ToggleField
            label="启用自动更新"
            description="开启后 Agent 心跳返回会提示节点自动执行自更新。"
            checked={watchedAutoUpdate}
            onChange={(checked) =>
              form.setValue('auto_update_enabled', checked, {
                shouldDirty: true,
                shouldValidate: true,
              })
            }
          />
        )}

        <ToggleField
          label="手动指定地图地区"
          description="关闭时，节点会根据当前 IP 自动解析归属地；开启后使用你手动选择的地区与坐标。"
          checked={watchedGeoManualOverride}
          onChange={(checked) => {
            form.setValue('geo_manual_override', checked, {
              shouldDirty: true,
              shouldValidate: true,
            });
            if (!checked) {
              form.setValue('geo_region', '', { shouldDirty: true });
              form.setValue('geo_name', '', { shouldDirty: true });
              form.setValue('geo_latitude', '', { shouldDirty: true });
              form.setValue('geo_longitude', '', { shouldDirty: true });
            }
          }}
        />

        {watchedGeoManualOverride ? (
          <ResourceField
            label="地区选择"
            hint="选择后会自动填充位置名与地图坐标，你也可以继续微调。"
          >
            <ResourceSelect
              value={form.watch('geo_region')}
              onChange={(event) => {
                const regionName = event.target.value;
                form.setValue('geo_region', regionName, {
                  shouldDirty: true,
                });
                const selectedRegion = regionOptions.find(
                  (item) => item.label === regionName,
                );
                if (!selectedRegion) {
                  return;
                }
                form.setValue('geo_name', selectedRegion.label, {
                  shouldDirty: true,
                  shouldValidate: true,
                });
                form.setValue('geo_latitude', String(selectedRegion.latitude), {
                  shouldDirty: true,
                  shouldValidate: true,
                });
                form.setValue(
                  'geo_longitude',
                  String(selectedRegion.longitude),
                  {
                    shouldDirty: true,
                    shouldValidate: true,
                  },
                );
              }}
            >
              <option value="">请选择地区</option>
              {regionOptions.map((option) => (
                <option key={option.label} value={option.label}>
                  {option.label}
                </option>
              ))}
            </ResourceSelect>
          </ResourceField>
        ) : null}

        <ResourceField
          label="地图位置名"
          hint={
            watchedGeoManualOverride
              ? '可在自动填充后继续修改，例如使用更贴近业务的展示名称。'
              : '自动解析模式下，该字段会由系统根据节点 IP 回填。'
          }
          error={form.formState.errors.geo_name?.message}
        >
          <ResourceInput
            placeholder="Shanghai"
            disabled={!watchedGeoManualOverride}
            {...form.register('geo_name')}
          />
        </ResourceField>

        <div className="grid gap-5 md:grid-cols-2">
          <ResourceField
            label="纬度"
            hint="范围 -90 到 90，例如上海约为 31.2304"
            error={form.formState.errors.geo_latitude?.message}
          >
            <ResourceInput
              placeholder="31.2304"
              disabled={!watchedGeoManualOverride}
              {...form.register('geo_latitude')}
            />
          </ResourceField>

          <ResourceField
            label="经度"
            hint="范围 -180 到 180，例如上海约为 121.4737"
            error={form.formState.errors.geo_longitude?.message}
          >
            <ResourceInput
              placeholder="121.4737"
              disabled={!watchedGeoManualOverride}
              {...form.register('geo_longitude')}
            />
          </ResourceField>
        </div>
      </form>
    </AppModal>
  );
}
