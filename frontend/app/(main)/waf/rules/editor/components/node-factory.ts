import type { WAFRuleNode } from '@/lib/services/openflare';

export const WAF_NODE_DRAG_MIME = 'application/openflare-waf-node';

export type AddableNodeType = Extract<
  WAFRuleNode['type'],
  'ip_match' | 'geo_match' | 'ua_check' | 'security_check' | 'pow' | 'block'
>;

export const NODE_TYPE_LABEL_KEYS = {
  start: 'start_d2bb025a',
  ip_match: 'ip_match_43f4b6d4',
  geo_match: 'geography_match_9acb08c4',
  ua_check: 'ua_check_b56dac23',
  security_check: 'security_checks_e09822e6',
  pow: 'pow_challenge_3c043088',
  allow: 'allow_1e9f2561',
  block: 'block_2a366d09',
} as const satisfies Record<WAFRuleNode['type'], string>;

export function displayNodeTitle(
  node: Pick<WAFRuleNode, 'type' | 'label'>,
  translate: (
    key: (typeof NODE_TYPE_LABEL_KEYS)[WAFRuleNode['type']],
  ) => string,
): string {
  const custom = node.label?.trim();
  return custom || translate(NODE_TYPE_LABEL_KEYS[node.type]);
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
  if (type === 'ua_check')
    return {
      id,
      type,
      position,
      config: {
        require_ua: false,
        browsers: [],
        operating_systems: [],
        match_mode: 'or',
        block_common_bots: false,
        block_abnormal_ua: false,
        block_custom_ua: false,
        custom_ua_patterns: [],
      },
    };
  if (type === 'security_check')
    return {
      id,
      type,
      position,
      config: {
        sql_injection: false,
        path_traversal: true,
        command_injection: false,
        xss: false,
        ssrf: false,
        file_inclusion: true,
        malicious_upload: false,
        xxe: false,
        crlf_injection: false,
      },
    };
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
    value === 'ua_check' ||
    value === 'security_check' ||
    value === 'pow' ||
    value === 'block'
  )
    return value;
  return null;
}
