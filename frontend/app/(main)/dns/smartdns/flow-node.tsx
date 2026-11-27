'use client';

import {
  Handle,
  Position,
  useUpdateNodeInternals,
  type Node,
  type NodeProps,
} from '@xyflow/react';
import { GitBranch, LogIn, LogOut, Sliders } from 'lucide-react';
import { useTranslations } from 'next-intl';
import { useEffect } from 'react';

import type { SmartDNSFlowNode } from '@/lib/services/openflare';
import { cn } from '@/lib/utils';
import { flowNodeType } from './flow-graph';

export type CanvasNode = Node<{ node: SmartDNSFlowNode }, 'smartdnsNode'>;

const icons = {
  input: LogIn,
  condition: GitBranch,
  action: Sliders,
  output: LogOut,
};

export function FlowNode({
  id,
  data: { node },
  selected,
}: NodeProps<CanvasNode>) {
  const t = useTranslations('smartdns.flow');
  const type = flowNodeType(node);
  const Icon = icons[type];
  const branches = type === 'condition' ? (node.data.cases ?? []) : [];
  const handles = branches.map((branch) => branch.id).join('\n');
  const updateInternals = useUpdateNodeInternals();
  useEffect(() => updateInternals(id), [id, handles, updateInternals]);

  const handleStyle = 'size-3!';
  return (
    <div
      className={cn(
        'w-52 rounded-lg border border-dashed bg-card text-card-foreground',
        selected && 'ring-2 ring-ring',
      )}
    >
      {type !== 'input' && (
        <Handle
          className={handleStyle}
          type='target'
          position={Position.Left}
        />
      )}
      <div className='flex items-center gap-2 px-3 py-2.5'>
        <Icon className='size-4 shrink-0 text-primary' />
        <div className='min-w-0'>
          <p className='truncate text-sm font-medium' title={node.label}>
            {node.label || t(`nodeTypes.${type}`)}
          </p>
          <p className='text-xs text-muted-foreground'>
            {t(`nodeTypes.${type}`)}
          </p>
        </div>
      </div>
      {branches.map((branch, index) => (
        <div
          key={branch.id}
          className='relative border-t border-dashed px-3 py-1.5 pr-4 text-right text-xs'
        >
          <p className='truncate' title={branch.label}>
            {branch.label || t('branchPosition', { position: index + 1 })}
          </p>
          <Handle
            id={branch.id}
            className={handleStyle}
            type='source'
            position={Position.Right}
          />
        </div>
      ))}
      {(type === 'input' || type === 'action') && (
        <Handle
          className={handleStyle}
          type='source'
          position={Position.Right}
        />
      )}
    </div>
  );
}
