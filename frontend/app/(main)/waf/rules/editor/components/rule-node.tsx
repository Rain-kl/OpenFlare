import { Handle, Position, type NodeProps } from '@xyflow/react';
import {
  Ban,
  Fingerprint,
  Flag,
  Globe2,
  Play,
  ScanSearch,
  Shield,
  ShieldCheck,
} from 'lucide-react';

import { Badge } from '@/components/ui/badge';
import { cn } from '@/lib/utils';
import type { WAFRuleNode } from '@/lib/services/openflare';

import { displayNodeTitle } from './node-factory';

export interface RuleFlowNodeData extends Record<string, unknown> {
  rule: WAFRuleNode;
  issues: number;
}

const meta = {
  start: { icon: Play },
  ip_match: { icon: Fingerprint },
  geo_match: { icon: Globe2 },
  ua_check: { icon: ScanSearch },
  security_check: { icon: Shield },
  pow: { icon: ShieldCheck },
  allow: { icon: Flag },
  block: { icon: Ban },
} as const;

const outputHandles: Partial<Record<WAFRuleNode['type'], string[]>> = {
  start: ['next'],
  ip_match: ['true', 'false'],
  geo_match: ['true', 'false'],
  ua_check: ['true', 'false'],
  security_check: ['true', 'false'],
  pow: ['next'],
};

export function RuleNode({ data, selected }: NodeProps) {
  const value = data as RuleFlowNodeData;
  const { rule, issues } = value;
  const { icon: Icon } = meta[rule.type];
  const title = displayNodeTitle(rule);
  return (
    <div
      className={cn(
        'min-w-44 rounded-lg border bg-card shadow-sm transition-shadow',
        selected && 'ring-2 ring-ring',
        issues > 0 && 'border-destructive',
      )}
    >
      {rule.type !== 'start' && (
        <Handle type='target' position={Position.Left} />
      )}
      <div className='flex items-center gap-3 px-4 py-3'>
        <Icon className='size-5 text-primary' />
        <div className='flex min-w-0 flex-1 flex-col gap-0.5'>
          <span className='text-sm font-medium'>{title}</span>
          <span className='font-mono text-[10px] text-muted-foreground'>
            {rule.id}
          </span>
        </div>
        {issues > 0 && <Badge variant='destructive'>{issues}</Badge>}
      </div>
      {(outputHandles[rule.type] ?? []).map((handle, index, all) => (
        <Handle
          key={handle}
          id={handle}
          type='source'
          position={Position.Right}
          style={{ top: `${((index + 1) / (all.length + 1)) * 100}%` }}
        >
          <span className='absolute right-3 -translate-y-1/2 text-[9px] text-muted-foreground'>
            {handle}
          </span>
        </Handle>
      ))}
    </div>
  );
}
