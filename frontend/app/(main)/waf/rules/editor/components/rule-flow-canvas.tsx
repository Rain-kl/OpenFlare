'use client';

import {
  type DragEvent,
  type MouseEvent as ReactMouseEvent,
  useCallback,
  useEffect,
  useRef,
  useState,
} from 'react';
import {
  addEdge,
  applyEdgeChanges,
  Background,
  Controls,
  MiniMap,
  ReactFlow,
  useEdgesState,
  useNodesState,
  type Connection,
  type Edge,
  type EdgeChange,
  type FitViewOptions,
  type Node,
  type NodeChange,
  type ReactFlowInstance,
} from '@xyflow/react';
import { AlignHorizontalSpaceAround, Trash2 } from 'lucide-react';
import '@xyflow/react/dist/style.css';

import { Button } from '@/components/ui/button';
import type { WAFRuleEdge, WAFRuleGraph } from '@/lib/services/openflare';
import { cn } from '@/lib/utils';

import {
  acceptedNodeChanges,
  filterRemovableNodeIds,
  type GraphErrorTarget,
  isConnectionAllowed,
  isPersistentEdgeChange,
} from './editor-behavior';
import { layoutRuleGraph } from './graph-layout';
import {
  type GraphIssue,
  removeEdgeFromGraph,
  removeNodeFromGraph,
} from './graph-validation';
import {
  createRuleNode,
  parseAddableNodeType,
  WAF_NODE_DRAG_MIME,
  type AddableNodeType,
} from './node-factory';
import { NodeLibrary } from './node-library';
import { RuleNode, type RuleFlowNodeData } from './rule-node';

const nodeTypes = { rule: RuleNode };
const initialFitViewOptions = {
  padding: 0.3,
  maxZoom: 0.85,
} satisfies FitViewOptions;

type FlowNode = Node<RuleFlowNodeData>;
type FlowEdge = Edge;

interface RuleFlowCanvasProps {
  graph: WAFRuleGraph;
  issues: GraphIssue[];
  selectedId?: string;
  selectedEdgeId?: string;
  focusTarget?: GraphErrorTarget;
  onGraphChange: (graph: WAFRuleGraph, persistent?: boolean) => void;
  onSelect: (id?: string) => void;
  onSelectEdge: (id?: string) => void;
}

export function RuleFlowCanvas({
  graph,
  issues,
  selectedId,
  selectedEdgeId,
  focusTarget,
  onGraphChange,
  onSelect,
  onSelectEdge,
}: RuleFlowCanvasProps) {
  const instance = useRef<ReactFlowInstance<FlowNode, FlowEdge> | null>(null);
  const [nodes, setNodes, applyFlowNodeChanges] = useNodesState<FlowNode>(
    buildFlowNodes(graph, issues, selectedId),
  );
  const [edges, setEdges, applyFlowEdgeChanges] = useEdgesState<FlowEdge>(
    buildFlowEdges(graph, selectedId, selectedEdgeId),
  );
  const [contextMenu, setContextMenu] = useState<
    | { kind: 'node'; id: string; x: number; y: number }
    | { kind: 'edge'; id: string; x: number; y: number }
    | null
  >(null);

  useEffect(() => {
    setNodes((current) => buildFlowNodes(graph, issues, selectedId, current));
  }, [graph, issues, selectedId, setNodes]);

  useEffect(() => {
    setEdges((current) =>
      buildFlowEdges(graph, selectedId, selectedEdgeId, current),
    );
  }, [graph, selectedEdgeId, selectedId, setEdges]);

  useEffect(() => {
    if (!focusTarget || !instance.current) return;
    if (focusTarget.kind === 'node') {
      void instance.current.fitView({
        nodes: [{ id: focusTarget.id }],
        duration: 350,
        maxZoom: 1.4,
      });
      return;
    }
    const edge = graph.edges.find((item) => item.id === focusTarget.id);
    if (edge)
      void instance.current.fitView({
        nodes: [{ id: edge.source }, { id: edge.target }],
        duration: 350,
        maxZoom: 1.4,
      });
  }, [focusTarget, graph.edges]);

  const onNodesChange = useCallback(
    (changes: NodeChange<FlowNode>[]) => {
      const accepted = acceptedNodeChanges(graph.nodes, changes);
      if (accepted.changes.length === 0) return;

      applyFlowNodeChanges(accepted.changes);
      if (!accepted.persistent) return;

      const removals = filterRemovableNodeIds(
        graph.nodes,
        accepted.changes
          .filter((change) => change.type === 'remove')
          .map((change) => change.id),
      );
      let next = graph;
      for (const id of removals) next = removeNodeFromGraph(next, id);

      const positions = new Map<string, { x: number; y: number }>();
      for (const change of accepted.changes) {
        if (
          change.type === 'position' &&
          change.dragging === false &&
          change.position
        ) {
          positions.set(change.id, change.position);
        }
      }
      next = {
        ...next,
        nodes: next.nodes.map((node) => ({
          ...node,
          position: positions.get(node.id) ?? node.position,
        })),
      };
      onGraphChange(next);

      if (selectedId && removals.includes(selectedId)) onSelect(undefined);
    },
    [applyFlowNodeChanges, graph, onGraphChange, onSelect, selectedId],
  );

  const onEdgesChange = useCallback(
    (changes: EdgeChange<FlowEdge>[]) => {
      applyFlowEdgeChanges(changes);
      const persistent = changes.filter(isPersistentEdgeChange);
      if (persistent.length === 0) return;

      const next = applyEdgeChanges(persistent, edges);
      onGraphChange({ ...graph, edges: next.map(toRuleEdge) });
      if (
        selectedEdgeId &&
        persistent.some(
          (change) => change.type === 'remove' && change.id === selectedEdgeId,
        )
      ) {
        onSelectEdge(undefined);
      }
    },
    [
      applyFlowEdgeChanges,
      edges,
      graph,
      onGraphChange,
      onSelectEdge,
      selectedEdgeId,
    ],
  );

  const isValidConnection = useCallback(
    (connection: Edge | Connection) => {
      return isConnectionAllowed(graph, connection);
    },
    [graph],
  );

  const onConnect = useCallback(
    (connection: Connection) => {
      if (!isValidConnection(connection)) return;
      const next = addEdge(
        {
          ...connection,
          id: `${connection.source}-${connection.sourceHandle}-${connection.target}`,
        },
        edges,
      );
      setEdges(next);
      onGraphChange({ ...graph, edges: next.map(toRuleEdge) });
    },
    [edges, graph, isValidConnection, onGraphChange, setEdges],
  );

  const addNodeAt = useCallback(
    (type: AddableNodeType, position: { x: number; y: number }) => {
      const node = createRuleNode(type, position);
      onGraphChange({
        ...graph,
        nodes: [...graph.nodes, node],
      });
      onSelectEdge(undefined);
      onSelect(node.id);
    },
    [graph, onGraphChange, onSelect, onSelectEdge],
  );

  const onDragOver = useCallback((event: DragEvent) => {
    event.preventDefault();
    event.dataTransfer.dropEffect = 'copy';
  }, []);

  const onDrop = useCallback(
    (event: DragEvent) => {
      event.preventDefault();
      const raw =
        event.dataTransfer.getData(WAF_NODE_DRAG_MIME) ||
        event.dataTransfer.getData('text/plain');
      const type = parseAddableNodeType(raw);
      if (!type || !instance.current) return;
      const position = instance.current.screenToFlowPosition({
        x: event.clientX,
        y: event.clientY,
      });
      addNodeAt(type, position);
    },
    [addNodeAt],
  );

  const deleteNodeById = useCallback(
    (nodeId: string) => {
      const target = graph.nodes.find((node) => node.id === nodeId);
      if (!target || ['start', 'allow'].includes(target.type)) return;
      setNodes((current) => current.filter((node) => node.id !== nodeId));
      setEdges((current) =>
        current.filter(
          (edge) => edge.source !== nodeId && edge.target !== nodeId,
        ),
      );
      onGraphChange(removeNodeFromGraph(graph, nodeId));
      if (selectedId === nodeId) onSelect(undefined);
      if (selectedEdgeId) onSelectEdge(undefined);
    },
    [
      graph,
      onGraphChange,
      onSelect,
      onSelectEdge,
      selectedEdgeId,
      selectedId,
      setEdges,
      setNodes,
    ],
  );

  const deleteEdgeById = useCallback(
    (edgeId: string) => {
      if (!graph.edges.some((edge) => edge.id === edgeId)) return;
      setEdges((current) => current.filter((edge) => edge.id !== edgeId));
      onGraphChange(removeEdgeFromGraph(graph, edgeId));
      if (selectedEdgeId === edgeId) onSelectEdge(undefined);
    },
    [graph, onGraphChange, onSelectEdge, selectedEdgeId, setEdges],
  );

  const openNodeContextMenu = useCallback(
    (event: ReactMouseEvent, node: FlowNode) => {
      event.preventDefault();
      event.stopPropagation();
      onSelectEdge(undefined);
      onSelect(node.id);
      setContextMenu({
        kind: 'node',
        id: node.id,
        x: event.clientX,
        y: event.clientY,
      });
    },
    [onSelect, onSelectEdge],
  );

  const openEdgeContextMenu = useCallback(
    (event: ReactMouseEvent, edge: FlowEdge) => {
      event.preventDefault();
      event.stopPropagation();
      onSelect(undefined);
      onSelectEdge(edge.id);
      setContextMenu({
        kind: 'edge',
        id: edge.id,
        x: event.clientX,
        y: event.clientY,
      });
    },
    [onSelect, onSelectEdge],
  );

  const suppressBrowserContextMenu = useCallback(
    (event: ReactMouseEvent | MouseEvent) => {
      event.preventDefault();
      setContextMenu(null);
    },
    [],
  );

  const formatLayout = useCallback(() => {
    const next = layoutRuleGraph(graph);
    onGraphChange(next);
    setContextMenu(null);
    requestAnimationFrame(() => {
      void instance.current?.fitView({
        ...initialFitViewOptions,
        duration: 280,
      });
    });
  }, [graph, onGraphChange]);

  const contextNode =
    contextMenu?.kind === 'node'
      ? graph.nodes.find((node) => node.id === contextMenu.id)
      : undefined;
  const contextCanDeleteNode = Boolean(
    contextNode && !['start', 'allow'].includes(contextNode.type),
  );
  const contextCanDeleteEdge = Boolean(
    contextMenu?.kind === 'edge' &&
    graph.edges.some((edge) => edge.id === contextMenu.id),
  );

  return (
    <section
      className='relative min-w-0 flex-1 bg-muted/20'
      onContextMenu={suppressBrowserContextMenu}
    >
      <div className='absolute left-4 top-4 z-10 flex flex-wrap items-center gap-2 rounded-lg border bg-background/95 p-2 shadow-sm backdrop-blur'>
        <NodeLibrary />
        <Button
          type='button'
          variant='outline'
          size='sm'
          title='自动整理节点与连线布局'
          onClick={formatLayout}
        >
          <AlignHorizontalSpaceAround data-icon='inline-start' />
          格式化布局
        </Button>
      </div>
      <ReactFlow
        nodes={nodes}
        edges={edges}
        nodeTypes={nodeTypes}
        onInit={(value) => {
          instance.current = value;
        }}
        onNodesChange={onNodesChange}
        onEdgesChange={onEdgesChange}
        onConnect={onConnect}
        isValidConnection={isValidConnection}
        onBeforeDelete={async ({ nodes: deletingNodes }) =>
          !deletingNodes.some((node) =>
            ['start', 'allow'].includes(node.data.rule.type),
          )
        }
        onNodeClick={(_, node) => {
          onSelectEdge(undefined);
          onSelect(node.id);
          setContextMenu(null);
        }}
        onEdgeClick={(_, edge) => {
          onSelect(undefined);
          onSelectEdge(edge.id);
          setContextMenu(null);
        }}
        onPaneClick={() => {
          onSelect(undefined);
          onSelectEdge(undefined);
          setContextMenu(null);
        }}
        onMoveStart={() => setContextMenu(null)}
        onNodeContextMenu={openNodeContextMenu}
        onEdgeContextMenu={openEdgeContextMenu}
        onPaneContextMenu={suppressBrowserContextMenu}
        onDragOver={onDragOver}
        onDrop={onDrop}
        fitView
        fitViewOptions={initialFitViewOptions}
        deleteKeyCode={['Backspace', 'Delete']}
        className='h-full w-full'
      >
        <Background gap={20} size={1} />
        <MiniMap pannable zoomable />
        <Controls />
      </ReactFlow>
      {contextMenu && (
        <div
          className='fixed z-50 min-w-40 overflow-hidden rounded-md border bg-popover p-1 text-popover-foreground shadow-md'
          style={{ left: contextMenu.x, top: contextMenu.y }}
          role='menu'
        >
          {contextMenu.kind === 'node' && (
            <button
              type='button'
              role='menuitem'
              disabled={!contextCanDeleteNode}
              className={cn(
                'flex w-full items-center gap-2 rounded-sm px-2 py-1.5 text-sm outline-none',
                contextCanDeleteNode
                  ? 'text-destructive hover:bg-destructive/10'
                  : 'cursor-not-allowed text-muted-foreground opacity-60',
              )}
              onClick={() => {
                deleteNodeById(contextMenu.id);
                setContextMenu(null);
              }}
            >
              <Trash2 className='size-4' />
              {contextCanDeleteNode ? '删除节点' : '系统节点不可删除'}
            </button>
          )}
          {contextMenu.kind === 'edge' && (
            <button
              type='button'
              role='menuitem'
              disabled={!contextCanDeleteEdge}
              className={cn(
                'flex w-full items-center gap-2 rounded-sm px-2 py-1.5 text-sm outline-none',
                contextCanDeleteEdge
                  ? 'text-destructive hover:bg-destructive/10'
                  : 'cursor-not-allowed text-muted-foreground opacity-60',
              )}
              onClick={() => {
                deleteEdgeById(contextMenu.id);
                setContextMenu(null);
              }}
            >
              <Trash2 className='size-4' />
              删除连线
            </button>
          )}
        </div>
      )}
    </section>
  );
}

function buildFlowNodes(
  graph: WAFRuleGraph,
  issues: GraphIssue[],
  selectedId?: string,
  current: FlowNode[] = [],
): FlowNode[] {
  const existing = new Map(current.map((node) => [node.id, node]));
  const issueCounts = new Map<string, number>();
  for (const issue of issues) {
    if (issue.nodeId)
      issueCounts.set(issue.nodeId, (issueCounts.get(issue.nodeId) ?? 0) + 1);
  }
  return graph.nodes.map((rule) => ({
    ...existing.get(rule.id),
    id: rule.id,
    type: 'rule',
    position: rule.position,
    selected: rule.id === selectedId,
    data: { rule, issues: issueCounts.get(rule.id) ?? 0 },
  }));
}

function buildFlowEdges(
  graph: WAFRuleGraph,
  selectedId?: string,
  selectedEdgeId?: string,
  current: FlowEdge[] = [],
): FlowEdge[] {
  const existing = new Map(current.map((edge) => [edge.id, edge]));
  return graph.edges.map((edge) => ({
    ...existing.get(edge.id),
    id: edge.id,
    source: edge.source,
    sourceHandle: edge.source_handle,
    target: edge.target,
    selected: edge.id === selectedEdgeId,
    animated: selectedId === edge.source || selectedId === edge.target,
  }));
}

function toRuleEdge(edge: Edge): WAFRuleEdge {
  return {
    id: edge.id,
    source: edge.source,
    source_handle: edge.sourceHandle ?? '',
    target: edge.target,
  };
}
