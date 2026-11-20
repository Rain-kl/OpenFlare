export type PerformanceValue = boolean | number | string;
export type PerformanceSettings = Record<string, PerformanceValue>;

export interface PerformanceParameter {
  key: string;
  group: string;
  default: PerformanceValue;
  min?: number;
  max?: number;
  choices?: string[];
}

export interface PerformancePolicy {
  id: number;
  name: string;
  is_default: boolean;
  settings: PerformanceSettings;
}

export interface PerformanceNode {
  node_id: string;
  name: string;
  status: string;
  policy_id: number | null;
}

export interface PerformancePolicies {
  policies: PerformancePolicy[];
  nodes: PerformanceNode[];
  parameters: PerformanceParameter[];
}
