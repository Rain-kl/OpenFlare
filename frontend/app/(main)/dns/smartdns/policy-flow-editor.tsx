'use client';

import '@xyflow/react/dist/style.css';

import {
  Background,
  Controls,
  MarkerType,
  ReactFlow,
  useNodesState,
  type Connection,
  type Edge,
  type NodeChange,
  type ReactFlowInstance,
} from '@xyflow/react';
import { Focus, Plus } from 'lucide-react';
import { useTranslations } from 'next-intl';
import { useCallback, useEffect, useRef, useState } from 'react';

import { Button } from '@/components/ui/button';
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import type {
  SmartDNSFlowGraph,
  SmartDNSFlowNode,
} from '@/lib/services/openflare';
import {
  availableNodePosition,
  canConnectFlow,
  flowId,
  flowNodeType,
} from './flow-graph';
import { FlowInspector } from './flow-inspector';
import { FlowNode, type CanvasNode } from './flow-node';

const nodeTypes = { smartdnsNode: FlowNode };
const fitViewOptions = { padding: 0.16, maxZoom: 1 };
const defaultEdgeOptions = {
  markerEnd: { type: MarkerType.ArrowClosed, width: 15, height: 15 },
};
type Selection = { type: 'node' | 'edge'; id: string } | null;

function canvasNodes(
  graph: SmartDNSFlowGraph,
  selectedID?: string,
  current: CanvasNode[] = [],
): CanvasNode[] {
  const previous = new Map(current.map((node) => [node.id, node]));
  return graph.nodes.map((node) => {
    const cached = previous.get(node.id);
    return {
      ...cached,
      id: node.id,
      type: 'smartdnsNode',
      position: cached?.dragging ? cached.position : node.position,
      selected: node.id === selectedID,
      data: { node },
    };
  });
}

export function PolicyFlowEditor({
  graph,
  onChange,
}: {
  graph: SmartDNSFlowGraph;
  onChange: (update: (current: SmartDNSFlowGraph) => SmartDNSFlowGraph) => void;
}) {
  const t = useTranslations('smartdns.flow');
  const [selection, setSelection] = useState<Selection>(null);
  const [nodes, setNodes, updateCanvasNodes] = useNodesState<CanvasNode>(
    canvasNodes(graph),
  );
  const instance = useRef<ReactFlowInstance<CanvasNode, Edge> | null>(null);
  const canvas = useRef<HTMLDivElement>(null);
  const fitAfterAdd = useRef(false);
  const nodeID = selection?.type === 'node' ? selection.id : undefined;
  const edgeID = selection?.type === 'edge' ? selection.id : undefined;
  const selectedNode = graph.nodes.find((node) => node.id === nodeID);
  const selectedEdge = graph.edges.find((edge) => edge.id === edgeID);
  const edges: Edge[] = graph.edges.map(
    ({ id, source, target, source_handle }) => ({
      id,
      source,
      target,
      sourceHandle: source_handle,
      selected: id === edgeID,
    }),
  );

  useEffect(() => {
    setNodes((current) => canvasNodes(graph, nodeID, current));
  }, [graph, nodeID, setNodes]);

  const fit = useCallback(
    () => void instance.current?.fitView({ ...fitViewOptions, duration: 180 }),
    [],
  );
  useEffect(() => {
    if (
      fitAfterAdd.current &&
      nodes.every((node) => node.measured?.width && node.measured?.height)
    ) {
      fitAfterAdd.current = false;
      fit();
    }
  }, [nodes, fit]);

  const handleNodeChanges = useCallback(
    (changes: NodeChange<CanvasNode>[]) => {
      updateCanvasNodes(changes);
      const positions = changes.flatMap((change) =>
        change.type === 'position' && change.position && !change.dragging
          ? [[change.id, change.position] as const]
          : [],
      );
      if (!positions.length) return;
      const byID = new Map(positions);
      onChange((current) => ({
        ...current,
        nodes: current.nodes.map((node) => {
          const position = byID.get(node.id);
          return position ? { ...node, position } : node;
        }),
      }));
    },
    [onChange, updateCanvasNodes],
  );

  const connect = (source: string, target: string, handle?: string | null) => {
    if (!canConnectFlow(graph, source, target, handle)) return;
    const edge = {
      id: flowId('edge'),
      source,
      source_handle: handle || undefined,
      target,
    };
    onChange((current) => ({ ...current, edges: [...current.edges, edge] }));
  };

  const addNode = (type: 'condition' | 'action') => {
    const rect = canvas.current?.getBoundingClientRect();
    const center =
      rect &&
      instance.current?.screenToFlowPosition({
        x: rect.x + rect.width / 2,
        y: rect.y + rect.height / 2,
      });
    const id = flowId(type);
    const node: SmartDNSFlowNode = {
      id,
      type,
      label: t(type === 'condition' ? 'newConditionLabel' : 'newActionLabel'),
      position: availableNodePosition(graph, center ?? { x: 150, y: 150 }),
      data:
        type === 'condition'
          ? {
              hit_policy: 'first',
              cases: [
                {
                  id: flowId('case'),
                  label: t('fallbackBranch'),
                  condition: '',
                },
              ],
            }
          : {
              mode: 'smart',
              weights: { latency: 0.55, load: 0.3, geo: 0.15 },
              use_tag_weights: true,
            },
    };
    fitAfterAdd.current = true;
    onChange((current) => ({ ...current, nodes: [...current.nodes, node] }));
    setSelection({ type: 'node', id });
  };

  const focusNode = (id: string) => {
    setSelection({ type: 'node', id });
    void instance.current?.fitView({
      nodes: [{ id }],
      maxZoom: 1,
      duration: 180,
    });
  };

  return (
    <div className='flex min-w-0 flex-col gap-3'>
      <div className='flex flex-wrap items-center justify-between gap-2'>
        <div className='flex flex-wrap gap-2'>
          <Button
            variant='outline'
            size='sm'
            onClick={() => addNode('condition')}
          >
            <Plus data-icon='inline-start' />
            {t('addCondition')}
          </Button>
          <Button variant='outline' size='sm' onClick={() => addNode('action')}>
            <Plus data-icon='inline-start' />
            {t('addAction')}
          </Button>
        </div>
        <div className='flex w-full min-w-0 flex-wrap items-center gap-2 sm:w-auto'>
          <Select value={nodeID ?? ''} onValueChange={focusNode}>
            <SelectTrigger
              size='sm'
              className='max-w-full sm:w-48'
              aria-label={t('jumpToNode')}
            >
              <SelectValue placeholder={t('jumpToNode')} />
            </SelectTrigger>
            <SelectContent>
              <SelectGroup>
                {graph.nodes.map((node) => (
                  <SelectItem key={node.id} value={node.id}>
                    {node.label || t(`nodeTypes.${flowNodeType(node)}`)}
                  </SelectItem>
                ))}
              </SelectGroup>
            </SelectContent>
          </Select>
          <Button variant='ghost' size='sm' onClick={fit}>
            <Focus data-icon='inline-start' />
            {t('fitFlow')}
          </Button>
        </div>
      </div>
      <div className='grid min-w-0 items-start gap-4 lg:grid-cols-[minmax(0,1fr)_minmax(17rem,29%)]'>
        <div
          ref={canvas}
          className='h-[24rem] min-w-0 overflow-hidden rounded-lg border border-dashed xl:h-[max(32rem,calc(100dvh-21rem))]'
        >
          <ReactFlow<CanvasNode, Edge>
            nodes={nodes}
            edges={edges}
            nodeTypes={nodeTypes}
            defaultEdgeOptions={defaultEdgeOptions}
            className='[--xy-handle-background-color:var(--primary)] [--xy-handle-border-color:var(--background)] [--xy-edge-stroke:var(--muted-foreground)] [--xy-edge-stroke-selected:var(--primary)] [--xy-edge-stroke-width:1.5] [--xy-controls-button-background-color:var(--background)] [--xy-controls-button-background-color-hover:var(--muted)] [--xy-controls-button-color:var(--foreground)] [--xy-controls-button-border-color:var(--border)]'
            onInit={(flow) => {
              instance.current = flow;
            }}
            onNodesChange={handleNodeChanges}
            isValidConnection={({ source, target, sourceHandle }) =>
              canConnectFlow(graph, source, target, sourceHandle)
            }
            onConnect={({ source, target, sourceHandle }: Connection) =>
              connect(source, target, sourceHandle)
            }
            onNodeClick={(_, node) =>
              setSelection({ type: 'node', id: node.id })
            }
            onNodeDragStart={(_, node) =>
              setSelection({ type: 'node', id: node.id })
            }
            onEdgeClick={(_, edge) =>
              setSelection({ type: 'edge', id: edge.id })
            }
            onPaneClick={() => setSelection(null)}
            deleteKeyCode={null}
            minZoom={0.15}
            maxZoom={1.8}
            fitView
            fitViewOptions={fitViewOptions}
          >
            <Background />
            <Controls showInteractive={false} />
          </ReactFlow>
        </div>
        <FlowInspector
          key={selection?.id ?? 'empty'}
          graph={graph}
          node={selectedNode}
          edge={selectedEdge}
          onGraphChange={onChange}
          onConnect={connect}
          onSelectEdge={(id) => setSelection({ type: 'edge', id })}
          onClearSelection={() => setSelection(null)}
        />
      </div>
    </div>
  );
}
