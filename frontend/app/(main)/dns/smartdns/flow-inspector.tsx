import { ChevronDown, ChevronUp, Plus, Trash2 } from 'lucide-react';
import { useTranslations } from 'next-intl';
import { useState } from 'react';

import { Button } from '@/components/ui/button';
import {
  Field,
  FieldDescription,
  FieldGroup,
  FieldLabel,
} from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import type {
  SmartDNSFlowEdge,
  SmartDNSFlowGraph,
  SmartDNSFlowNode,
  SmartDNSFlowSwitchCase,
} from '@/lib/services/openflare';
import { FlowActionFields } from './flow-action-fields';
import { canConnectFlow, flowId, flowNodeType } from './flow-graph';

interface FlowInspectorProps {
  graph: SmartDNSFlowGraph;
  node?: SmartDNSFlowNode;
  edge?: SmartDNSFlowEdge;
  onGraphChange: (
    update: (current: SmartDNSFlowGraph) => SmartDNSFlowGraph,
  ) => void;
  onConnect: (source: string, target: string, handle?: string) => void;
  onSelectEdge: (id: string) => void;
  onClearSelection: () => void;
}

export function FlowInspector({
  graph,
  node,
  edge,
  onGraphChange,
  onConnect,
  onSelectEdge,
  onClearSelection,
}: FlowInspectorProps) {
  const t = useTranslations('smartdns.flow');
  const type = node ? flowNodeType(node) : undefined;
  const nodeName = (id: string) => {
    const item = graph.nodes.find((candidate) => candidate.id === id);
    return item ? item.label || t(`nodeTypes.${flowNodeType(item)}`) : id;
  };
  const rename = (label: string) => updateNode((item) => ({ ...item, label }));
  const updateNode = (update: (item: SmartDNSFlowNode) => SmartDNSFlowNode) =>
    onGraphChange((current) => ({
      ...current,
      nodes: current.nodes.map((item) =>
        item.id === node?.id ? update(item) : item,
      ),
    }));
  const updateBranches = (cases: SmartDNSFlowSwitchCase[]) => {
    const valid = new Set(cases.map((branch) => branch.id));
    onGraphChange((current) => ({
      nodes: current.nodes.map((item) =>
        item.id === node?.id
          ? { ...item, data: { ...item.data, cases } }
          : item,
      ),
      edges: current.edges.filter(
        (item) =>
          item.source !== node?.id ||
          !item.source_handle ||
          valid.has(item.source_handle),
      ),
    }));
  };
  const remove = () => {
    if (edge) {
      onGraphChange((current) => ({
        ...current,
        edges: current.edges.filter((item) => item.id !== edge.id),
      }));
    } else if (node && (type === 'condition' || type === 'action')) {
      onGraphChange((current) => ({
        nodes: current.nodes.filter((item) => item.id !== node.id),
        edges: current.edges.filter(
          (item) => item.source !== node.id && item.target !== node.id,
        ),
      }));
    }
    onClearSelection();
  };
  const removable = edge || type === 'condition' || type === 'action';
  const branchName = (connection: SmartDNSFlowEdge) => {
    const source = graph.nodes.find((item) => item.id === connection.source);
    const branch = source?.data.cases?.find(
      (item) => item.id === connection.source_handle,
    );
    return branch?.label || '';
  };

  return (
    <aside
      aria-label={t('properties')}
      className='flex min-w-0 flex-col gap-4 rounded-lg border border-dashed p-4 xl:max-h-[max(32rem,calc(100dvh-21rem))] xl:overflow-y-auto'
    >
      <div className='flex flex-wrap items-center justify-between gap-2'>
        <h2 className='text-base font-semibold'>
          {edge
            ? t('connection')
            : type
              ? t(`nodeTypes.${type}`)
              : t('properties')}
        </h2>
        {removable && (
          <Button size='sm' variant='ghost' onClick={remove}>
            <Trash2 data-icon='inline-start' />
            {edge ? t('removeEdge') : t('removeNode')}
          </Button>
        )}
      </div>
      {edge ? (
        <div className='flex flex-col gap-1 text-sm wrap-anywhere'>
          <p>
            {nodeName(edge.source)} → {nodeName(edge.target)}
          </p>
          {branchName(edge) && (
            <p className='text-muted-foreground'>{branchName(edge)}</p>
          )}
        </div>
      ) : node ? (
        <>
          <Field>
            <FieldLabel htmlFor='sdns-flow-node-name'>
              {t('nodeLabel')}
            </FieldLabel>
            <Input
              id='sdns-flow-node-name'
              value={node.label}
              onChange={(event) => rename(event.target.value)}
            />
          </Field>
          {type === 'condition' && (
            <ConditionFields
              node={node}
              onBranchesChange={updateBranches}
              onHitPolicyChange={(hit_policy) =>
                updateNode((item) => ({
                  ...item,
                  data: { ...item.data, hit_policy },
                }))
              }
            />
          )}
          {type === 'action' && (
            <FlowActionFields
              data={node.data}
              onChange={(patch) =>
                updateNode((item) => ({
                  ...item,
                  data: { ...item.data, ...patch },
                }))
              }
            />
          )}
          {type !== 'output' && (
            <ConnectionPicker
              graph={graph}
              node={node}
              onConnect={(target, handle) => onConnect(node.id, target, handle)}
            />
          )}
          {graph.edges.some(
            (item) => item.source === node.id || item.target === node.id,
          ) && (
            <div className='flex flex-col items-start gap-1'>
              <h3 className='text-sm font-medium'>{t('connections')}</h3>
              {graph.edges
                .filter(
                  (item) => item.source === node.id || item.target === node.id,
                )
                .map((item) => (
                  <Button
                    key={item.id}
                    variant='ghost'
                    size='sm'
                    className='h-auto max-w-full justify-start px-0 text-left whitespace-normal'
                    onClick={() => onSelectEdge(item.id)}
                  >
                    {nodeName(item.source)} → {nodeName(item.target)}
                    {branchName(item) ? ` · ${branchName(item)}` : ''}
                  </Button>
                ))}
            </div>
          )}
        </>
      ) : (
        <p className='text-sm text-muted-foreground'>{t('selectNodeHint')}</p>
      )}
    </aside>
  );
}

function ConditionFields({
  node,
  onBranchesChange,
  onHitPolicyChange,
}: {
  node: SmartDNSFlowNode;
  onBranchesChange: (cases: SmartDNSFlowSwitchCase[]) => void;
  onHitPolicyChange: (value: string) => void;
}) {
  const t = useTranslations('smartdns.flow');
  const branches = node.data.cases ?? [];
  const updateBranch = (id: string, patch: Partial<SmartDNSFlowSwitchCase>) =>
    onBranchesChange(
      branches.map((branch) =>
        branch.id === id ? { ...branch, ...patch } : branch,
      ),
    );
  const addBranch = () => {
    const branch = {
      id: flowId('case'),
      label: t('branchPosition', { position: branches.length + 1 }),
      condition: '',
    };
    const fallback = branches.findIndex((item) => !item.condition.trim());
    const at = fallback === -1 ? branches.length : fallback;
    onBranchesChange([...branches.slice(0, at), branch, ...branches.slice(at)]);
  };
  const moveBranch = (index: number, offset: number) => {
    const next = [...branches];
    const to = index + offset;
    [next[index], next[to]] = [next[to], next[index]];
    onBranchesChange(next);
  };
  const first = (node.data.hit_policy || 'first') === 'first';
  const earlyFallback =
    first && branches.slice(0, -1).some((branch) => !branch.condition.trim());

  return (
    <FieldGroup className='gap-3'>
      <Field>
        <FieldLabel htmlFor='sdns-flow-hit-policy'>{t('hitPolicy')}</FieldLabel>
        <Select
          value={node.data.hit_policy || 'first'}
          onValueChange={onHitPolicyChange}
        >
          <SelectTrigger id='sdns-flow-hit-policy' className='w-full'>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectGroup>
              <SelectItem value='first'>{t('hitFirst')}</SelectItem>
              <SelectItem value='collect'>{t('hitCollect')}</SelectItem>
            </SelectGroup>
          </SelectContent>
        </Select>
      </Field>
      <div className='flex flex-wrap items-center justify-between gap-2'>
        <h3 className='text-sm font-medium'>{t('branches')}</h3>
        <Button variant='outline' size='sm' onClick={addBranch}>
          <Plus data-icon='inline-start' />
          {t('addBranch')}
        </Button>
      </div>
      <p className='text-xs text-muted-foreground'>
        {t(earlyFallback ? 'fallbackBeforeBranch' : 'branchHint')}
      </p>
      {branches.map((branch, index) => (
        <div
          key={branch.id}
          className='flex min-w-0 flex-col gap-3 rounded-lg border border-dashed p-3'
        >
          <div className='flex flex-wrap items-center justify-between gap-1'>
            <p className='text-xs font-medium'>
              {t('branchPosition', { position: index + 1 })}
            </p>
            <div className='flex gap-1'>
              <Button
                variant='ghost'
                size='icon-sm'
                aria-label={t('moveBranchUp')}
                disabled={index === 0}
                onClick={() => moveBranch(index, -1)}
              >
                <ChevronUp />
              </Button>
              <Button
                variant='ghost'
                size='icon-sm'
                aria-label={t('moveBranchDown')}
                disabled={index === branches.length - 1}
                onClick={() => moveBranch(index, 1)}
              >
                <ChevronDown />
              </Button>
              <Button
                variant='ghost'
                size='icon-sm'
                aria-label={t('removeBranch')}
                disabled={branches.length <= 1}
                onClick={() =>
                  onBranchesChange(
                    branches.filter((item) => item.id !== branch.id),
                  )
                }
              >
                <Trash2 />
              </Button>
            </div>
          </div>
          <Field>
            <FieldLabel htmlFor={`sdns-flow-branch-name-${branch.id}`}>
              {t('branchLabel')}
            </FieldLabel>
            <Input
              id={`sdns-flow-branch-name-${branch.id}`}
              value={branch.label}
              onChange={(event) =>
                updateBranch(branch.id, { label: event.target.value })
              }
            />
          </Field>
          <Field>
            <FieldLabel htmlFor={`sdns-flow-branch-expr-${branch.id}`}>
              {t('branchExpression')}
            </FieldLabel>
            <Input
              id={`sdns-flow-branch-expr-${branch.id}`}
              className='font-mono'
              value={branch.condition}
              placeholder="client.country == 'CN'"
              onChange={(event) =>
                updateBranch(branch.id, { condition: event.target.value })
              }
            />
          </Field>
        </div>
      ))}
    </FieldGroup>
  );
}

function ConnectionPicker({
  graph,
  node,
  onConnect,
}: {
  graph: SmartDNSFlowGraph;
  node: SmartDNSFlowNode;
  onConnect: (target: string, handle?: string) => void;
}) {
  const t = useTranslations('smartdns.flow');
  const [branchId, setBranchId] = useState('');
  const branches =
    flowNodeType(node) === 'condition' ? (node.data.cases ?? []) : undefined;
  const handle =
    branches?.find((branch) => branch.id === branchId)?.id ?? branches?.[0]?.id;
  const targets = graph.nodes.filter((target) =>
    canConnectFlow(graph, node.id, target.id, handle),
  );
  return (
    <FieldGroup className='gap-3'>
      {branches && branches.length > 0 && (
        <Field>
          <FieldLabel htmlFor='sdns-flow-port'>
            {t('connectionBranch')}
          </FieldLabel>
          <Select value={handle} onValueChange={setBranchId}>
            <SelectTrigger id='sdns-flow-port' className='w-full'>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectGroup>
                {branches.map((branch, index) => (
                  <SelectItem key={branch.id} value={branch.id}>
                    {branch.label ||
                      t('branchPosition', { position: index + 1 })}
                  </SelectItem>
                ))}
              </SelectGroup>
            </SelectContent>
          </Select>
        </Field>
      )}
      <Field>
        <FieldLabel htmlFor='sdns-flow-target'>{t('connectTo')}</FieldLabel>
        <Select
          value=''
          onValueChange={(target) => onConnect(target, handle)}
          disabled={!targets.length}
        >
          <SelectTrigger id='sdns-flow-target' className='w-full'>
            <SelectValue
              placeholder={t(targets.length ? 'selectTarget' : 'noTargets')}
            />
          </SelectTrigger>
          <SelectContent>
            <SelectGroup>
              {targets.map((target) => (
                <SelectItem key={target.id} value={target.id}>
                  {target.label || t(`nodeTypes.${flowNodeType(target)}`)}
                </SelectItem>
              ))}
            </SelectGroup>
          </SelectContent>
        </Select>
        <FieldDescription>{t('connectHint')}</FieldDescription>
      </Field>
    </FieldGroup>
  );
}
