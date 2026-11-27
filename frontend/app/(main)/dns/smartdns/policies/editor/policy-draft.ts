import type {
  SmartDNSFlowGraph,
  SmartDNSPolicy,
  SmartDNSPolicyInput,
} from '@/lib/services/openflare';
import { createDefaultFlowGraph } from '../../flow-graph';
import type { PresetRules } from '../../preset-rule-fields';

export type PolicyEditorMode = 'smart' | 'load_balance' | 'flow' | 'jdm';

export const smartWeights = { latency: '0.55', load: '0.30', geo: '0.15' };
export const balanceWeights = { latency: '0', load: '1', geo: '0' };

export interface PolicyDraft {
  mode: PolicyEditorMode;
  name: string;
  description: string;
  scope: 'all' | 'domains';
  domains: string;
  priority: string;
  ttl: string;
  maxAnswers: string;
  enabled: boolean;
  fallback: string;
  fallbackNodes: string;
  preset: PresetRules;
  graph: SmartDNSFlowGraph;
  jdm: string;
}

function splitList(raw: string) {
  return raw
    .split(/[\n,]+/)
    .map((value) => value.trim())
    .filter(Boolean);
}

export function createPolicyDraft(policy?: SmartDNSPolicy): PolicyDraft {
  const graph = policy?.config?.graph;
  const strategy = policy?.strategy_type;
  const preset = strategy === 'load_balance' ? balanceWeights : smartWeights;
  return {
    mode:
      strategy === 'custom'
        ? graph?.nodes?.length
          ? 'flow'
          : 'jdm'
        : strategy === 'load_balance'
          ? 'load_balance'
          : 'smart',
    name: policy?.name ?? '',
    description: policy?.description ?? '',
    scope: policy?.scope_type === 'domains' ? 'domains' : 'all',
    domains: (policy?.scope_domains ?? []).join(', '),
    priority: String(policy?.priority ?? 100),
    ttl: String(policy?.ttl ?? 60),
    maxAnswers: String(policy?.max_answers ?? 1),
    enabled: policy?.enabled ?? true,
    fallback:
      policy?.fallback_strategy && policy.fallback_strategy !== 'lowest_load'
        ? policy.fallback_strategy
        : 'smart',
    fallbackNodes: (policy?.fallback_node_ids ?? []).join(', '),
    preset: {
      latency: String(policy?.config?.weights?.latency ?? preset.latency),
      load: String(policy?.config?.weights?.load ?? preset.load),
      geo: String(policy?.config?.weights?.geo ?? preset.geo),
      useTagWeights: policy?.config?.use_tag_weights ?? true,
      countries: (policy?.config?.filters?.countries ?? []).join(', '),
      tags: (policy?.config?.filters?.tags ?? []).join(', '),
      groups: (policy?.config?.filters?.network_groups ?? []).join(', '),
    },
    graph: graph?.nodes?.length ? graph : createDefaultFlowGraph(strategy),
    jdm:
      strategy === 'custom' && !graph?.nodes?.length
        ? (policy?.jdm_json ?? '')
        : '',
  };
}

export function buildPolicyPayload(
  draft: PolicyDraft,
  policy?: SmartDNSPolicy,
): SmartDNSPolicyInput {
  const mode = draft.mode === 'load_balance' ? 'load_balance' : 'smart';
  const priority = Number(draft.priority);
  const ttl = Number(draft.ttl);
  const maxAnswers = Number(draft.maxAnswers);
  return {
    name: draft.name.trim(),
    description: draft.description.trim(),
    scope_type: draft.scope,
    scope_domains: draft.scope === 'domains' ? splitList(draft.domains) : [],
    strategy_type:
      draft.mode === 'flow' || draft.mode === 'jdm' ? 'custom' : draft.mode,
    priority,
    ttl,
    max_answers: maxAnswers,
    enabled: draft.enabled,
    is_default: policy?.is_default ?? false,
    fallback_strategy: draft.fallback,
    fallback_node_ids:
      draft.fallback === 'static_nodes' ? splitList(draft.fallbackNodes) : [],
    config: {
      mode,
      weights: {
        latency: Number(draft.preset.latency) || 0,
        load: Number(draft.preset.load) || 0,
        geo: Number(draft.preset.geo) || 0,
      },
      use_tag_weights: draft.preset.useTagWeights,
      filters: {
        node_ids: policy?.config?.filters?.node_ids ?? [],
        countries: splitList(draft.preset.countries),
        tags: splitList(draft.preset.tags),
        network_groups: splitList(draft.preset.groups),
      },
      ttl,
      max_answers: maxAnswers,
      graph: draft.mode === 'flow' ? draft.graph : null,
    },
    jdm_json: draft.mode === 'jdm' ? draft.jdm.trim() : '',
  };
}
