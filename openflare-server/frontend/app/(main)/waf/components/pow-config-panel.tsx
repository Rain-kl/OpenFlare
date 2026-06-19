'use client';

import {useEffect, useState} from 'react';

import {Input} from '@/components/ui/input';
import {Label} from '@/components/ui/label';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import {Switch} from '@/components/ui/switch';
import {Textarea} from '@/components/ui/textarea';
import type {ProxyRoutePoWConfig} from '@/lib/services/openflare';

import {listToText, parseTextareaList} from './helpers';

interface PowConfigPanelProps {
  enabled: boolean;
  config: ProxyRoutePoWConfig;
  onChange: (enabled: boolean, config: ProxyRoutePoWConfig) => void;
}

type PowListDraft = {
  whitelist: Record<keyof ProxyRoutePoWConfig['whitelist'], string>;
  blacklist: Record<keyof ProxyRoutePoWConfig['blacklist'], string>;
};

function buildDraft(config: ProxyRoutePoWConfig): PowListDraft {
  return {
    whitelist: {
      ips: listToText(config.whitelist?.ips),
      ip_cidrs: listToText(config.whitelist?.ip_cidrs),
      paths: listToText(config.whitelist?.paths),
      path_regexes: listToText(config.whitelist?.path_regexes),
      user_agents: listToText(config.whitelist?.user_agents),
    },
    blacklist: {
      ips: listToText(config.blacklist?.ips),
      ip_cidrs: listToText(config.blacklist?.ip_cidrs),
      paths: listToText(config.blacklist?.paths),
      path_regexes: listToText(config.blacklist?.path_regexes),
      user_agents: listToText(config.blacklist?.user_agents),
    },
  };
}

function applyDraft(
  config: ProxyRoutePoWConfig,
  draft: PowListDraft,
): ProxyRoutePoWConfig {
  return {
    ...config,
    whitelist: {
      ips: parseTextareaList(draft.whitelist.ips),
      ip_cidrs: parseTextareaList(draft.whitelist.ip_cidrs),
      paths: parseTextareaList(draft.whitelist.paths),
      path_regexes: parseTextareaList(draft.whitelist.path_regexes),
      user_agents: parseTextareaList(draft.whitelist.user_agents),
    },
    blacklist: {
      ips: parseTextareaList(draft.blacklist.ips),
      ip_cidrs: parseTextareaList(draft.blacklist.ip_cidrs),
      paths: parseTextareaList(draft.blacklist.paths),
      path_regexes: parseTextareaList(draft.blacklist.path_regexes),
      user_agents: parseTextareaList(draft.blacklist.user_agents),
    },
  };
}

function PowListCard({
  title,
  scope,
  draft,
  onUpdate,
}: {
  title: string;
  scope: keyof PowListDraft;
  draft: PowListDraft;
  onUpdate: (
    scope: keyof PowListDraft,
    key: keyof ProxyRoutePoWConfig['whitelist'],
    value: string,
  ) => void;
}) {
  const fields: Array<{
    key: keyof ProxyRoutePoWConfig['whitelist'];
    label: string;
    placeholder: string;
  }> = [
    {key: 'ips', label: 'IP', placeholder: '每行一个 IP'},
    {key: 'ip_cidrs', label: 'IP CIDR', placeholder: '每行一个网段，如 10.0.0.0/8'},
    {key: 'paths', label: '路径', placeholder: '每行一个路径前缀'},
    {key: 'path_regexes', label: '路径正则', placeholder: '每行一个正则表达式'},
    {key: 'user_agents', label: 'User-Agent', placeholder: '每行一个 UA 关键词或正则'},
  ];

  return (
    <div className="space-y-3 rounded-lg border border-dashed p-4">
      <p className="text-sm font-medium">{title}</p>
      <div className="space-y-3">
        {fields.map((field) => (
          <div key={field.key} className="space-y-1.5">
            <Label className="text-xs text-muted-foreground">{field.label}</Label>
            <Textarea
              value={draft[scope][field.key]}
              onChange={(event) => onUpdate(scope, field.key, event.target.value)}
              placeholder={field.placeholder}
              className="min-h-16 resize-y font-mono text-xs"
            />
          </div>
        ))}
      </div>
    </div>
  );
}

export function PowConfigPanel({enabled, config, onChange}: PowConfigPanelProps) {
  const [draft, setDraft] = useState(() => buildDraft(config));

  useEffect(() => {
    setDraft(buildDraft(config));
  }, [config]);

  const emitChange = (
    nextEnabled: boolean,
    nextConfig: Partial<ProxyRoutePoWConfig>,
    nextDraft?: PowListDraft,
  ) => {
    const merged = {
      ...config,
      ...nextConfig,
    };
    onChange(nextEnabled, nextDraft ? applyDraft(merged, nextDraft) : merged);
  };

  const updateList = (
    scope: keyof PowListDraft,
    key: keyof ProxyRoutePoWConfig['whitelist'],
    value: string,
  ) => {
    const nextDraft: PowListDraft = {
      ...draft,
      [scope]: {
        ...draft[scope],
        [key]: value,
      },
    };
    setDraft(nextDraft);
    emitChange(enabled, {}, nextDraft);
  };

  return (
    <div className="space-y-5">
      <div className="flex items-center justify-between rounded-lg border border-dashed p-4">
        <div className="space-y-1 pr-4">
          <Label htmlFor="pow-enabled">启用 PoW 防护</Label>
          <p className="text-xs text-muted-foreground">
            启用后，命中该规则组的请求需要完成浏览器计算挑战。
          </p>
        </div>
        <Switch
          id="pow-enabled"
          checked={enabled}
          onCheckedChange={(checked) => emitChange(checked, {})}
        />
      </div>

      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <div className="space-y-1.5">
          <Label className="text-xs text-muted-foreground">算法</Label>
          <Select
            value={config.algorithm}
            onValueChange={(value: 'fast' | 'slow') =>
              emitChange(enabled, {algorithm: value})
            }
          >
            <SelectTrigger className="h-9">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="fast">Fast</SelectItem>
              <SelectItem value="slow">Slow</SelectItem>
            </SelectContent>
          </Select>
        </div>

        <div className="space-y-1.5">
          <Label className="text-xs text-muted-foreground">难度</Label>
          <Input
            type="number"
            min={1}
            max={16}
            value={config.difficulty}
            onChange={(event) =>
              emitChange(enabled, {difficulty: Number(event.target.value) || 1})
            }
          />
        </div>

        <div className="space-y-1.5">
          <Label className="text-xs text-muted-foreground">会话 TTL (秒)</Label>
          <Input
            type="number"
            min={60}
            value={config.session_ttl}
            onChange={(event) =>
              emitChange(enabled, {session_ttl: Number(event.target.value) || 60})
            }
          />
        </div>

        <div className="space-y-1.5">
          <Label className="text-xs text-muted-foreground">挑战 TTL (秒)</Label>
          <Input
            type="number"
            min={30}
            value={config.challenge_ttl}
            onChange={(event) =>
              emitChange(enabled, {challenge_ttl: Number(event.target.value) || 30})
            }
          />
        </div>
      </div>

      <div className="grid gap-4 xl:grid-cols-2">
        <PowListCard
          title="白名单（跳过 PoW）"
          scope="whitelist"
          draft={draft}
          onUpdate={updateList}
        />
        <PowListCard
          title="黑名单（必须 PoW）"
          scope="blacklist"
          draft={draft}
          onUpdate={updateList}
        />
      </div>
    </div>
  );
}