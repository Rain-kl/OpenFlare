import type { WAFRuleNode } from '@/lib/services/openflare';

export const WAF_NODE_DRAG_MIME = 'application/openflare-waf-node';

export type AddableNodeType = Extract<
  WAFRuleNode['type'],
  'ip_match' | 'geo_match' | 'pow' | 'block'
>;

export const NODE_TYPE_LABELS: Record<WAFRuleNode['type'], string> = {
  start: '开始',
  ip_match: 'IP 匹配',
  geo_match: '地域匹配',
  pow: 'PoW 挑战',
  allow: '通过',
  block: '阻止',
};

export function displayNodeTitle(
  node: Pick<WAFRuleNode, 'type' | 'label'>,
): string {
  const custom = node.label?.trim();
  return custom || NODE_TYPE_LABELS[node.type];
}

export function createRuleNode(
  type: AddableNodeType,
  position: { x: number; y: number },
): WAFRuleNode {
  const id = `${type}-${crypto.randomUUID().slice(0, 8)}`;
  if (type === 'ip_match')
    return {
      id,
      type,
      position,
      config: { ips: [], cidrs: [], ip_group_ids: [] },
    };
  if (type === 'geo_match')
    return { id, type, position, config: { countries: [], regions: [] } };
  if (type === 'pow')
    return {
      id,
      type,
      position,
      config: {
        algorithm: 'fast',
        difficulty: 4,
        session_ttl: 3600,
        challenge_ttl: 300,
      },
    };
  return {
    id,
    type: 'block',
    position,
    config: { status_code: 403, response_body: '' },
  };
}

export function parseAddableNodeType(value: string): AddableNodeType | null {
  if (
    value === 'ip_match' ||
    value === 'geo_match' ||
    value === 'pow' ||
    value === 'block'
  )
    return value;
  return null;
}
