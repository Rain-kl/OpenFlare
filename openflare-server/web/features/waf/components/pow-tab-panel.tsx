import {useEffect, useState} from 'react';
import type {ProxyRoutePoWConfig} from '@/features/proxy-routes/types';
import {
    ResourceField,
    ResourceInput,
    ResourceTextarea,
    ToggleField,
} from '@/features/shared/components/resource-primitives';
import {listToText, parseTextareaList} from './helpers';

export function PowTabPanel({
  enabled,
  config,
  onChange,
}: {
  enabled: boolean;
  config: ProxyRoutePoWConfig;
  onChange: (enabled: boolean, config: ProxyRoutePoWConfig) => void;
}) {
  const [draft, setDraft] = useState(() => ({
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
  }));

  useEffect(() => {
    setDraft({
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
    });
  }, [config]);

  const updateConfig = (
    newEnabled: boolean,
    newConfig: Partial<ProxyRoutePoWConfig>,
    newDraft?: typeof draft,
  ) => {
    const nextConfig = { ...config, ...newConfig };
    if (newDraft) {
      setDraft(newDraft);
      nextConfig.whitelist = {
        ips: parseTextareaList(newDraft.whitelist.ips),
        ip_cidrs: parseTextareaList(newDraft.whitelist.ip_cidrs),
        paths: parseTextareaList(newDraft.whitelist.paths),
        path_regexes: parseTextareaList(newDraft.whitelist.path_regexes),
        user_agents: parseTextareaList(newDraft.whitelist.user_agents),
      };
      nextConfig.blacklist = {
        ips: parseTextareaList(newDraft.blacklist.ips),
        ip_cidrs: parseTextareaList(newDraft.blacklist.ip_cidrs),
        paths: parseTextareaList(newDraft.blacklist.paths),
        path_regexes: parseTextareaList(newDraft.blacklist.path_regexes),
        user_agents: parseTextareaList(newDraft.blacklist.user_agents),
      };
    }
    onChange(newEnabled, nextConfig);
  };

  const updateList = (
    scope: 'whitelist' | 'blacklist',
    key: keyof ProxyRoutePoWConfig['whitelist'],
    value: string,
  ) => {
    const nextDraft = {
      ...draft,
      [scope]: {
        ...draft[scope],
        [key]: value,
      },
    };
    updateConfig(enabled, {}, nextDraft);
  };

  return (
    <div className="space-y-6">
      <ToggleField
        label="启用 PoW 防护"
        description="启用后，命中该规则组的请求需要完成浏览器计算挑战。"
        checked={enabled}
        onChange={(newEnabled) => updateConfig(newEnabled, {})}
      />

      <div className="grid gap-5 md:grid-cols-2 xl:grid-cols-4">
        <ResourceField label="算法">
          <select
            value={config.algorithm}
            onChange={(event) =>
              updateConfig(enabled, {
                algorithm: event.target.value as 'fast' | 'slow',
              })
            }
            className="w-full rounded-2xl border border-[var(--border-default)] bg-[var(--control-background)] px-4 py-3 text-sm text-[var(--foreground-primary)] transition outline-none focus:border-[var(--border-strong)]"
          >
            <option value="fast">Fast</option>
            <option value="slow">Slow</option>
          </select>
        </ResourceField>
        <ResourceField label="难度">
          <ResourceInput
            type="number"
            min={1}
            max={16}
            value={config.difficulty}
            onChange={(event) =>
              updateConfig(enabled, { difficulty: Number(event.target.value) })
            }
          />
        </ResourceField>
        <ResourceField label="会话 TTL">
          <ResourceInput
            type="number"
            min={60}
            value={config.session_ttl}
            onChange={(event) =>
              updateConfig(enabled, { session_ttl: Number(event.target.value) })
            }
          />
        </ResourceField>
        <ResourceField label="挑战 TTL">
          <ResourceInput
            type="number"
            min={30}
            value={config.challenge_ttl}
            onChange={(event) =>
              updateConfig(enabled, {
                challenge_ttl: Number(event.target.value),
              })
            }
          />
        </ResourceField>
      </div>

      <div className="grid gap-5 xl:grid-cols-2">
        {(['whitelist', 'blacklist'] as const).map((scope) => (
          <div
            key={scope}
            className="rounded-[26px] border border-[var(--border-default)] bg-[var(--surface-elevated)] p-5"
          >
            <h3 className="text-sm font-semibold text-[var(--foreground-primary)]">
              {scope === 'whitelist'
                ? '白名单（跳过 PoW）'
                : '黑名单（必须 PoW）'}
            </h3>
            <div className="mt-4 space-y-4">
              <ResourceField label="IP">
                <ResourceTextarea
                  className="min-h-20"
                  value={draft[scope].ips}
                  onChange={(event) =>
                    updateList(scope, 'ips', event.target.value)
                  }
                />
              </ResourceField>
              <ResourceField label="IP CIDR">
                <ResourceTextarea
                  className="min-h-20"
                  value={draft[scope].ip_cidrs}
                  onChange={(event) =>
                    updateList(scope, 'ip_cidrs', event.target.value)
                  }
                />
              </ResourceField>
              <ResourceField label="路径">
                <ResourceTextarea
                  className="min-h-20"
                  value={draft[scope].paths}
                  onChange={(event) =>
                    updateList(scope, 'paths', event.target.value)
                  }
                />
              </ResourceField>
              <ResourceField label="路径正则">
                <ResourceTextarea
                  className="min-h-20"
                  value={draft[scope].path_regexes}
                  onChange={(event) =>
                    updateList(scope, 'path_regexes', event.target.value)
                  }
                />
              </ResourceField>
              <ResourceField label="User-Agent">
                <ResourceTextarea
                  className="min-h-20"
                  value={draft[scope].user_agents}
                  onChange={(event) =>
                    updateList(scope, 'user_agents', event.target.value)
                  }
                />
              </ResourceField>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
