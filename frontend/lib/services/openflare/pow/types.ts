export type PoWPathMatch = {
  type: 'exact' | 'prefix' | 'wildcard' | 'regex';
  value: string;
};

export interface PoWPathRules {
  mode: 'all' | 'blacklist';
  whitelist?: PoWPathMatch[];
  blacklist?: PoWPathMatch[];
}

export interface PoWPolicyConfig {
  algorithm: 'fast' | 'slow';
  difficulty: number;
  session_ttl: number;
  challenge_ttl: number;
  path_rules?: PoWPathRules;
}

export interface PoWPolicy {
  route_id: number;
  enabled: boolean;
  config: PoWPolicyConfig;
}

export interface PoWPageConfig {
  template: 'classic' | 'shield';
  widget: 'progress' | 'turnstile';
  html?: string;
}

export const defaultPoWConfig: PoWPolicyConfig = {
  algorithm: 'fast',
  difficulty: 4,
  session_ttl: 600,
  challenge_ttl: 300,
  path_rules: { mode: 'all', whitelist: [], blacklist: [] },
};
