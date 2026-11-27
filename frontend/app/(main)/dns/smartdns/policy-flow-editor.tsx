'use client';

import '@xyflow/react/dist/style.css';

import {
  addEdge,
  applyEdgeChanges,
  applyNodeChanges,
  Background,
  Controls,
  Handle,
  Position,
  ReactFlow,
  type Connection,
  type Edge,
  type EdgeChange,
  type Node,
  type NodeChange,
  type NodeProps,
} from '@xyflow/react';
import {
  GitBranch,
  Play,
  Plus,
  ShieldCheck,
  Sliders,
  Trash2,
} from 'lucide-react';
import { useTranslations } from 'next-intl';
import { useCallback, useMemo, useState } from 'react';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { Switch } from '@/components/ui/switch';
import type {
  SmartDNSFlowGraph,
  SmartDNSFlowNode,
  SmartDNSFlowSwitchCase,
} from '@/lib/services/openflare';
import { cn } from '@/lib/utils';

export function createDefaultFlowGraph(strategy = 'smart'): SmartDNSFlowGraph {
  const isLB = strategy === 'load_balance';
  return {
    nodes: [
      {
        id: 'input',
        type: 'input',
        label: 'DNS Query',
        position: { x: 40, y: 120 },
        data: {},
      },
      {
        id: 'cond-1',
        type: 'condition',
        label: 'Geo / Network Match',
        position: { x: 260, y: 110 },
        data: {
          hit_policy: 'first',
          cases: [
            {
              id: 'case-cn',
              label: 'Mainland China',
              condition: "client.country == 'CN'",
            },
            {
              id: 'case-default',
              label: 'Default Fallback',
              condition: '',
            },
          ],
        },
      },
      {
        id: 'action-cn',
        type: 'action',
        label: 'CN Low Latency',
        position: { x: 540, y: 40 },
        data: {
          mode: isLB ? 'load_balance' : 'smart',
          weights: isLB
            ? { latency: 0, load: 1, geo: 0 }
            : { latency: 0.65, load: 0.25, geo: 0.1 },
          use_tag_weights: true,
          filters: {
            node_ids: [],
            countries: [],
            tags: [],
            network_groups: [],
          },
        },
      },
      {
        id: 'action-default',
        type: 'action',
        label: 'Global Selection',
        position: { x: 540, y: 200 },
        data: {
          mode: isLB ? 'load_balance' : 'smart',
          weights: isLB
            ? { latency: 0, load: 1, geo: 0 }
            : { latency: 0.55, load: 0.3, geo: 0.15 },
          use_tag_weights: true,
          filters: {
            node_ids: [],
            countries: [],
            tags: [],
            network_groups: [],
          },
        },
      },
      {
        id: 'output',
        type: 'output',
        label: 'DNS Answer',
        position: { x: 820, y: 120 },
        data: {},
      },
    ],
    edges: [
      { id: 'e-in', source: 'input', target: 'cond-1' },
      {
        id: 'e-cn',
        source: 'cond-1',
        source_handle: 'case-cn',
        target: 'action-cn',
      },
      {
        id: 'e-def',
        source: 'cond-1',
        source_handle: 'case-default',
        target: 'action-default',
      },
      { id: 'e-out-cn', source: 'action-cn', target: 'output' },
      { id: 'e-out-def', source: 'action-default', target: 'output' },
    ],
  };
}

interface FlowNodeData extends Record<string, unknown> {
  node: SmartDNSFlowNode;
}

function SmartDNSFlowNodeCard({ data, selected }: NodeProps) {
  const { node } = data as FlowNodeData;
  const cases = node.data.cases ?? [];
  const icon =
    node.type === 'input' ? (
      <Play className='size-4 text-primary' />
    ) : node.type === 'condition' ? (
      <GitBranch className='size-4 text-primary' />
    ) : node.type === 'action' ? (
      <Sliders className='size-4 text-primary' />
    ) : (
      <ShieldCheck className='size-4 text-primary' />
    );

  return (
    <div
      className={cn(
        'min-w-44 rounded-lg border border-dashed bg-card px-3 py-2.5 text-card-foreground',
        selected && 'ring-2 ring-ring',
      )}
    >
      {node.type !== 'input' ? (
        <Handle type='target' position={Position.Left} />
      ) : null}

      <div className='flex items-center gap-2'>
        {icon}
        <div className='min-w-0 flex-1'>
          <div className='truncate text-xs font-medium'>{node.label}</div>
          <div className='font-mono text-[10px] text-muted-foreground'>
            {node.type}
          </div>
        </div>
      </div>

      {node.type === 'condition' ? (
        cases.map((c, idx) => (
          <Handle
            key={c.id}
            id={c.id}
            type='source'
            position={Position.Right}
            style={{ top: `${((idx + 1) / (cases.length + 1)) * 100}%` }}
          />
        ))
      ) : node.type !== 'output' ? (
        <Handle type='source' position={Position.Right} />
      ) : null}
    </div>
  );
}

const nodeTypes = { smartdnsNode: SmartDNSFlowNodeCard };

function splitCSV(raw: string): string[] {
  return raw
    .split(',')
    .map((s) => s.trim())
    .filter(Boolean);
}

export function PolicyFlowEditor({
  graph,
  onChange,
  onValidate,
  validating,
}: {
  graph: SmartDNSFlowGraph;
  onChange: (next: SmartDNSFlowGraph) => void;
  onValidate: () => void;
  validating: boolean;
}) {
  const t = useTranslations('smartdns.flow');
  const [selectedNodeId, setSelectedNodeId] = useState<string | null>(
    graph.nodes[1]?.id ?? graph.nodes[0]?.id ?? null,
  );
  const [selectedEdgeId, setSelectedEdgeId] = useState<string | null>(null);

  const rfNodes = useMemo<Node[]>(
    () =>
      graph.nodes.map((node) => ({
        id: node.id,
        type: 'smartdnsNode',
        position: node.position,
        selected: node.id === selectedNodeId,
        data: { node },
      })),
    [graph.nodes, selectedNodeId],
  );

  const rfEdges = useMemo<Edge[]>(
    () =>
      graph.edges.map((edge) => ({
        id: edge.id,
        source: edge.source,
        sourceHandle: edge.source_handle || undefined,
        target: edge.target,
        label:
          graph.nodes
            .find((n) => n.id === edge.source)
            ?.data.cases?.find((c) => c.id === edge.source_handle)?.label ??
          undefined,
      })),
    [graph.edges, graph.nodes],
  );

  const onNodesChange = useCallback(
    (changes: NodeChange[]) => {
      const updated = applyNodeChanges(changes, rfNodes);
      const nextNodes = graph.nodes.map((orig) => {
        const match = updated.find((u) => u.id === orig.id);
        return match ? { ...orig, position: match.position } : orig;
      });
      onChange({ ...graph, nodes: nextNodes });
    },
    [graph, onChange, rfNodes],
  );

  const onEdgesChange = useCallback(
    (changes: EdgeChange[]) => {
      const updated = applyEdgeChanges(changes, rfEdges);
      const nextEdges = updated.map((e) => ({
        id: e.id,
        source: e.source,
        source_handle: e.sourceHandle ?? undefined,
        target: e.target,
      }));
      onChange({ ...graph, edges: nextEdges });
    },
    [graph, onChange, rfEdges],
  );

  const onConnect = useCallback(
    (conn: Connection) => {
      if (!conn.source || !conn.target || conn.source === conn.target) return;
      const updated = addEdge(
        {
          ...conn,
          id: `e-${Date.now().toString(36)}`,
        },
        rfEdges,
      );
      onChange({
        ...graph,
        edges: updated.map((e) => ({
          id: e.id,
          source: e.source,
          source_handle: e.sourceHandle ?? undefined,
          target: e.target,
        })),
      });
    },
    [graph, onChange, rfEdges],
  );

  const selectedNode = useMemo(
    () => graph.nodes.find((n) => n.id === selectedNodeId) ?? null,
    [graph.nodes, selectedNodeId],
  );

  const updateSelectedNode = (
    updater: (n: SmartDNSFlowNode) => SmartDNSFlowNode,
  ) => {
    if (!selectedNode) return;
    onChange({
      ...graph,
      nodes: graph.nodes.map((n) =>
        n.id === selectedNode.id ? updater(n) : n,
      ),
    });
  };

  const addConditionNode = () => {
    const id = `cond-${Date.now().toString(36)}`;
    const nextNode: SmartDNSFlowNode = {
      id,
      type: 'condition',
      label: t('newConditionLabel'),
      position: { x: 280, y: 240 },
      data: {
        hit_policy: 'first',
        cases: [
          {
            id: `case-${Date.now().toString(36)}`,
            label: 'Branch 1',
            condition: "client.country == 'US'",
          },
          {
            id: `case-def-${Date.now().toString(36)}`,
            label: 'Default',
            condition: '',
          },
        ],
      },
    };
    onChange({ ...graph, nodes: [...graph.nodes, nextNode] });
    setSelectedNodeId(id);
  };

  const addActionNode = () => {
    const id = `action-${Date.now().toString(36)}`;
    const nextNode: SmartDNSFlowNode = {
      id,
      type: 'action',
      label: t('newActionLabel'),
      position: { x: 540, y: 280 },
      data: {
        mode: 'smart',
        weights: { latency: 0.55, load: 0.3, geo: 0.15 },
        use_tag_weights: true,
        filters: { node_ids: [], countries: [], tags: [], network_groups: [] },
      },
    };
    onChange({ ...graph, nodes: [...graph.nodes, nextNode] });
    setSelectedNodeId(id);
  };

  const deleteSelectedNode = () => {
    if (
      !selectedNode ||
      selectedNode.type === 'input' ||
      selectedNode.type === 'output'
    ) {
      return;
    }
    onChange({
      nodes: graph.nodes.filter((n) => n.id !== selectedNode.id),
      edges: graph.edges.filter(
        (e) => e.source !== selectedNode.id && e.target !== selectedNode.id,
      ),
    });
    setSelectedNodeId(null);
  };

  const deleteSelectedEdge = () => {
    if (!selectedEdgeId) return;
    onChange({
      ...graph,
      edges: graph.edges.filter((e) => e.id !== selectedEdgeId),
    });
    setSelectedEdgeId(null);
  };

  return (
    <div className='space-y-3'>
      <div className='flex flex-wrap items-center justify-between gap-2'>
        <div className='flex flex-wrap items-center gap-2'>
          <Button
            type='button'
            variant='outline'
            size='sm'
            onClick={addConditionNode}
          >
            <Plus data-icon='inline-start' />
            {t('addCondition')}
          </Button>
          <Button
            type='button'
            variant='outline'
            size='sm'
            onClick={addActionNode}
          >
            <Plus data-icon='inline-start' />
            {t('addAction')}
          </Button>
          {selectedEdgeId ? (
            <Button
              type='button'
              variant='outline'
              size='sm'
              onClick={deleteSelectedEdge}
            >
              <Trash2 data-icon='inline-start' />
              {t('removeEdge')}
            </Button>
          ) : null}
        </div>
        <Button
          type='button'
          variant='outline'
          size='sm'
          disabled={validating}
          onClick={onValidate}
        >
          <ShieldCheck data-icon='inline-start' />
          {t('validateFlow')}
        </Button>
      </div>

      <div className='grid gap-4 lg:grid-cols-[minmax(0,1fr)_18rem]'>
        <div className='h-80 w-full overflow-hidden rounded-lg border border-dashed bg-background'>
          <ReactFlow
            nodes={rfNodes}
            edges={rfEdges}
            nodeTypes={nodeTypes}
            onNodesChange={onNodesChange}
            onEdgesChange={onEdgesChange}
            onConnect={onConnect}
            onNodeClick={(_, n) => {
              setSelectedNodeId(n.id);
              setSelectedEdgeId(null);
            }}
            onEdgeClick={(_, e) => setSelectedEdgeId(e.id)}
            fitView
          >
            <Background />
            <Controls />
          </ReactFlow>
        </div>

        <div className='space-y-3 rounded-lg border border-dashed p-3 text-xs'>
          {selectedNode ? (
            <>
              <div className='flex items-center justify-between gap-2'>
                <Badge variant='outline'>{selectedNode.type}</Badge>
                {selectedNode.type !== 'input' &&
                selectedNode.type !== 'output' ? (
                  <Button
                    type='button'
                    variant='ghost'
                    size='icon-sm'
                    aria-label={t('removeNode')}
                    onClick={deleteSelectedNode}
                  >
                    <Trash2 className='size-3.5 text-destructive' />
                  </Button>
                ) : null}
              </div>

              <div className='space-y-1'>
                <Label>{t('nodeLabel')}</Label>
                <Input
                  value={selectedNode.label}
                  onChange={(e) =>
                    updateSelectedNode((n) => ({
                      ...n,
                      label: e.target.value,
                    }))
                  }
                />
              </div>

              {selectedNode.type === 'condition' ? (
                <div className='space-y-2'>
                  <div className='space-y-1'>
                    <Label>{t('hitPolicy')}</Label>
                    <Select
                      value={selectedNode.data.hit_policy || 'first'}
                      onValueChange={(val) =>
                        updateSelectedNode((n) => ({
                          ...n,
                          data: { ...n.data, hit_policy: val },
                        }))
                      }
                    >
                      <SelectTrigger className='w-full'>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value='first'>first</SelectItem>
                        <SelectItem value='collect'>collect</SelectItem>
                      </SelectContent>
                    </Select>
                  </div>

                  <div className='flex items-center justify-between'>
                    <span className='font-medium'>{t('branches')}</span>
                    <Button
                      type='button'
                      variant='ghost'
                      size='sm'
                      onClick={() => {
                        const newCase: SmartDNSFlowSwitchCase = {
                          id: `case-${Date.now().toString(36)}`,
                          label: `Case ${(selectedNode.data.cases?.length ?? 0) + 1}`,
                          condition: '',
                        };
                        updateSelectedNode((n) => ({
                          ...n,
                          data: {
                            ...n.data,
                            cases: [...(n.data.cases ?? []), newCase],
                          },
                        }));
                      }}
                    >
                      <Plus className='size-3.5' />
                    </Button>
                  </div>

                  {(selectedNode.data.cases ?? []).map((c, idx) => (
                    <div
                      key={c.id}
                      className='space-y-1 rounded border border-dashed p-2'
                    >
                      <Input
                        placeholder={t('branchLabel')}
                        value={c.label}
                        onChange={(e) => {
                          const nextCases = [
                            ...(selectedNode.data.cases ?? []),
                          ];
                          nextCases[idx] = { ...c, label: e.target.value };
                          updateSelectedNode((n) => ({
                            ...n,
                            data: { ...n.data, cases: nextCases },
                          }));
                        }}
                      />
                      <Input
                        className='font-mono'
                        placeholder="client.country == 'CN'"
                        value={c.condition}
                        onChange={(e) => {
                          const nextCases = [
                            ...(selectedNode.data.cases ?? []),
                          ];
                          nextCases[idx] = { ...c, condition: e.target.value };
                          updateSelectedNode((n) => ({
                            ...n,
                            data: { ...n.data, cases: nextCases },
                          }));
                        }}
                      />
                    </div>
                  ))}
                </div>
              ) : null}

              {selectedNode.type === 'action' ? (
                <div className='space-y-2'>
                  <div className='space-y-1'>
                    <Label>{t('actionMode')}</Label>
                    <Select
                      value={selectedNode.data.mode || 'smart'}
                      onValueChange={(val) =>
                        updateSelectedNode((n) => ({
                          ...n,
                          data: { ...n.data, mode: val },
                        }))
                      }
                    >
                      <SelectTrigger className='w-full'>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value='smart'>smart</SelectItem>
                        <SelectItem value='load_balance'>
                          load_balance
                        </SelectItem>
                      </SelectContent>
                    </Select>
                  </div>

                  <div className='grid grid-cols-3 gap-1.5'>
                    <div className='space-y-1'>
                      <Label>{t('wLatency')}</Label>
                      <Input
                        type='number'
                        step='0.05'
                        value={selectedNode.data.weights?.latency ?? 0.55}
                        onChange={(e) =>
                          updateSelectedNode((n) => ({
                            ...n,
                            data: {
                              ...n.data,
                              weights: {
                                latency: Number(e.target.value) || 0,
                                load: n.data.weights?.load ?? 0.3,
                                geo: n.data.weights?.geo ?? 0.15,
                              },
                            },
                          }))
                        }
                      />
                    </div>
                    <div className='space-y-1'>
                      <Label>{t('wLoad')}</Label>
                      <Input
                        type='number'
                        step='0.05'
                        value={selectedNode.data.weights?.load ?? 0.3}
                        onChange={(e) =>
                          updateSelectedNode((n) => ({
                            ...n,
                            data: {
                              ...n.data,
                              weights: {
                                latency: n.data.weights?.latency ?? 0.55,
                                load: Number(e.target.value) || 0,
                                geo: n.data.weights?.geo ?? 0.15,
                              },
                            },
                          }))
                        }
                      />
                    </div>
                    <div className='space-y-1'>
                      <Label>{t('wGeo')}</Label>
                      <Input
                        type='number'
                        step='0.05'
                        value={selectedNode.data.weights?.geo ?? 0.15}
                        onChange={(e) =>
                          updateSelectedNode((n) => ({
                            ...n,
                            data: {
                              ...n.data,
                              weights: {
                                latency: n.data.weights?.latency ?? 0.55,
                                load: n.data.weights?.load ?? 0.3,
                                geo: Number(e.target.value) || 0,
                              },
                            },
                          }))
                        }
                      />
                    </div>
                  </div>

                  <div className='flex items-center justify-between pt-1'>
                    <Label>{t('useTagWeights')}</Label>
                    <Switch
                      checked={selectedNode.data.use_tag_weights ?? true}
                      onCheckedChange={(checked) =>
                        updateSelectedNode((n) => ({
                          ...n,
                          data: { ...n.data, use_tag_weights: checked },
                        }))
                      }
                    />
                  </div>

                  <div className='space-y-1'>
                    <Label>{t('filterCountries')}</Label>
                    <Input
                      placeholder='CN, SG'
                      value={(selectedNode.data.filters?.countries ?? []).join(
                        ', ',
                      )}
                      onChange={(e) =>
                        updateSelectedNode((n) => ({
                          ...n,
                          data: {
                            ...n.data,
                            filters: {
                              node_ids: n.data.filters?.node_ids ?? [],
                              countries: splitCSV(e.target.value),
                              tags: n.data.filters?.tags ?? [],
                              network_groups:
                                n.data.filters?.network_groups ?? [],
                            },
                          },
                        }))
                      }
                    />
                  </div>

                  <div className='space-y-1'>
                    <Label>{t('filterTags')}</Label>
                    <Input
                      placeholder='optimized, high-bw'
                      value={(selectedNode.data.filters?.tags ?? []).join(', ')}
                      onChange={(e) =>
                        updateSelectedNode((n) => ({
                          ...n,
                          data: {
                            ...n.data,
                            filters: {
                              node_ids: n.data.filters?.node_ids ?? [],
                              countries: n.data.filters?.countries ?? [],
                              tags: splitCSV(e.target.value),
                              network_groups:
                                n.data.filters?.network_groups ?? [],
                            },
                          },
                        }))
                      }
                    />
                  </div>

                  <div className='space-y-1'>
                    <Label>{t('filterGroups')}</Label>
                    <Input
                      placeholder='cn2-gia, bgp'
                      value={(
                        selectedNode.data.filters?.network_groups ?? []
                      ).join(', ')}
                      onChange={(e) =>
                        updateSelectedNode((n) => ({
                          ...n,
                          data: {
                            ...n.data,
                            filters: {
                              node_ids: n.data.filters?.node_ids ?? [],
                              countries: n.data.filters?.countries ?? [],
                              tags: n.data.filters?.tags ?? [],
                              network_groups: splitCSV(e.target.value),
                            },
                          },
                        }))
                      }
                    />
                  </div>
                </div>
              ) : null}
            </>
          ) : (
            <p className='text-muted-foreground'>{t('selectNodeHint')}</p>
          )}
        </div>
      </div>
    </div>
  );
}
