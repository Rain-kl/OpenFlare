'use client';

import { type DragEvent, useCallback, useEffect, useRef } from 'react';
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
import { Trash2 } from 'lucide-react';
import '@xyflow/react/dist/style.css';

import { Button } from '@/components/ui/button';
import type { WAFRuleEdge, WAFRuleGraph } from '@/lib/services/openflare';

import {
  acceptedNodeChanges,
  filterRemovableNodeIds,
  type GraphErrorTarget,
  isConnectionAllowed,
  isPersistentEdgeChange,
} from './editor-behavior';
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

  const selectedNode = graph.nodes.find((node) => node.id === selectedId);
  const canDeleteNode = Boolean(
    selectedNode && !['start', 'allow'].includes(selectedNode.type),
  );
  const canDeleteEdge = Boolean(
    selectedEdgeId && graph.edges.some((edge) => edge.id === selectedEdgeId),
  );
  const canDelete = canDeleteNode || canDeleteEdge;
  const deleteLabel = canDeleteEdge
    ? '删除连线'
    : canDeleteNode
      ? '删除节点'
      : selectedNode
        ? '系统节点不可删除'
        : '请选择可删除项';

  const deleteSelection = useCallback(() => {
    if (canDeleteEdge && selectedEdgeId) {
      setEdges((current) =>
        current.filter((edge) => edge.id !== selectedEdgeId),
      );
      onGraphChange(removeEdgeFromGraph(graph, selectedEdgeId));
      onSelectEdge(undefined);
      return;
    }
    if (canDeleteNode && selectedId) {
      setNodes((current) => current.filter((node) => node.id !== selectedId));
      setEdges((current) =>
        current.filter(
          (edge) => edge.source !== selectedId && edge.target !== selectedId,
        ),
      );
      onGraphChange(removeNodeFromGraph(graph, selectedId));
      onSelect(undefined);
    }
  }, [
    canDeleteEdge,
    canDeleteNode,
    graph,
    onGraphChange,
    onSelect,
    onSelectEdge,
    selectedEdgeId,
    selectedId,
    setEdges,
    setNodes,
  ]);

  return (
    <section className='relative min-w-0 flex-1 bg-muted/20'>
      <div className='absolute left-4 top-4 z-10 rounded-lg border bg-background/95 p-2 shadow-sm backdrop-blur'>
        <NodeLibrary />
      </div>
      <div className='absolute right-4 top-4 z-10'>
        <Button
          variant='destructive'
          size='sm'
          disabled={!canDelete}
          onClick={deleteSelection}
        >
          <Trash2 data-icon='inline-start' />
          {deleteLabel}
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
        }}
        onEdgeClick={(_, edge) => {
          onSelect(undefined);
          onSelectEdge(edge.id);
        }}
        onPaneClick={() => {
          onSelect(undefined);
          onSelectEdge(undefined);
        }}
        onDragOver={onDragOver}
        onDrop={onDrop}
        fitView
        fitViewOptions={initialFitViewOptions}
        deleteKeyCode={['Backspace', 'Delete']}
      >
        <Background gap={20} size={1} />
        <MiniMap pannable zoomable />
        <Controls />
      </ReactFlow>
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
