'use client';

import {useCallback, useEffect, useMemo, useRef} from 'react';
import {addEdge, Background, Controls, MiniMap, ReactFlow, type Connection, type Edge, type Node, type NodeChange, applyNodeChanges, type EdgeChange, applyEdgeChanges, type ReactFlowInstance} from '@xyflow/react';
import '@xyflow/react/dist/style.css';

import type {WAFRuleEdge, WAFRuleGraph, WAFRuleNode} from '@/lib/services/openflare';

import {type GraphIssue, removeNodeFromGraph} from './graph-validation';
import {acceptedNodeChanges, filterRemovableNodeIds, type GraphErrorTarget, isConnectionAllowed, isPersistentEdgeChange} from './editor-behavior';
import {NodeLibrary} from './node-library';
import {RuleNode, type RuleFlowNodeData} from './rule-node';

const nodeTypes = {rule: RuleNode};

export function RuleFlowCanvas({graph, issues, selectedId, selectedEdgeId, focusTarget, onGraphChange, onSelect, onSelectEdge}: {graph: WAFRuleGraph; issues: GraphIssue[]; selectedId?: string; selectedEdgeId?: string; focusTarget?: GraphErrorTarget; onGraphChange: (graph: WAFRuleGraph, persistent?: boolean) => void; onSelect: (id?: string) => void; onSelectEdge: (id?: string) => void}) {
  const instance = useRef<ReactFlowInstance<Node<RuleFlowNodeData>, Edge> | null>(null);
  const nodes = useMemo<Node<RuleFlowNodeData>[]>(() => graph.nodes.map((rule) => ({id: rule.id, type: 'rule', position: rule.position, selected: rule.id === selectedId, data: {rule, issues: issues.filter((issue) => issue.nodeId === rule.id).length}})), [graph.nodes, issues, selectedId]);
  const edges = useMemo<Edge[]>(() => graph.edges.map((edge) => ({id: edge.id, source: edge.source, sourceHandle: edge.source_handle, target: edge.target, selected: edge.id === selectedEdgeId, animated: selectedId === edge.source || selectedId === edge.target})), [graph.edges, selectedEdgeId, selectedId]);

  useEffect(() => {
    if (!focusTarget || !instance.current) return;
    if (focusTarget.kind === 'node') void instance.current.fitView({nodes: [{id: focusTarget.id}], duration: 350, maxZoom: 1.4});
    else {
      const edge = graph.edges.find((item) => item.id === focusTarget.id);
      if (edge) void instance.current.fitView({nodes: [{id: edge.source}, {id: edge.target}], duration: 350, maxZoom: 1.4});
    }
  }, [focusTarget, graph.edges]);

  const onNodesChange = useCallback((changes: NodeChange[]) => {
    const accepted = acceptedNodeChanges(graph.nodes, changes);
    if (accepted.changes.length === 0) return;
    const removals = filterRemovableNodeIds(graph.nodes, accepted.changes.filter((change) => change.type === 'remove').map((change) => change.id));
    let next = graph;
    for (const id of removals) next = removeNodeFromGraph(next, id);
    const positioned = applyNodeChanges(accepted.changes.filter((change) => change.type !== 'remove'), nodes);
    const positions = new Map(positioned.map((node) => [node.id, node.position]));
    onGraphChange({...next, nodes: next.nodes.map((node) => ({...node, position: positions.get(node.id) ?? node.position}))}, accepted.persistent);
  }, [graph, nodes, onGraphChange]);

  const onEdgesChange = useCallback((changes: EdgeChange[]) => {
    const persistent = changes.filter(isPersistentEdgeChange);
    if (persistent.length === 0) return;
    const next = applyEdgeChanges(persistent, edges);
    onGraphChange({...graph, edges: next.map(toRuleEdge)});
  }, [edges, graph, onGraphChange]);

  const isValidConnection = useCallback((connection: Edge | Connection) => {
    return isConnectionAllowed(graph, connection);
  }, [graph]);

  const onConnect = useCallback((connection: Connection) => {
    if (!isValidConnection(connection)) return;
    const next = addEdge({...connection, id: `${connection.source}-${connection.sourceHandle}-${connection.target}`}, edges);
    onGraphChange({...graph, edges: next.map(toRuleEdge)});
  }, [edges, graph, isValidConnection, onGraphChange]);

  const addNode = useCallback((type: 'ip_match' | 'geo_match' | 'pow' | 'block') => {
    const id = `${type}-${crypto.randomUUID().slice(0, 8)}`;
    const config = type === 'ip_match' ? {ips: [], cidrs: [], ip_group_ids: []} : type === 'geo_match' ? {countries: [], regions: []} : type === 'pow' ? {algorithm: 'fast' as const, difficulty: 4, session_ttl: 3600, challenge_ttl: 300} : {status_code: 403, response_body: ''};
    onGraphChange({...graph, nodes: [...graph.nodes, {id, type, position: {x: 240, y: 140 + graph.nodes.length * 24}, config} as WAFRuleNode]});
    onSelect(id);
  }, [graph, onGraphChange, onSelect]);

  return <section className="relative min-w-0 flex-1 bg-muted/20"><div className="absolute left-4 top-4 z-10 rounded-lg border bg-background/95 p-2 shadow-sm backdrop-blur"><NodeLibrary onAdd={addNode}/></div><ReactFlow nodes={nodes} edges={edges} nodeTypes={nodeTypes} onInit={(value) => { instance.current = value; }} onNodesChange={onNodesChange} onEdgesChange={onEdgesChange} onConnect={onConnect} isValidConnection={isValidConnection} onNodeClick={(_, node) => { onSelectEdge(undefined); onSelect(node.id); }} onEdgeClick={(_, edge) => { onSelect(undefined); onSelectEdge(edge.id); }} onPaneClick={() => { onSelect(undefined); onSelectEdge(undefined); }} fitView deleteKeyCode={['Backspace', 'Delete']}><Background gap={20} size={1}/><MiniMap pannable zoomable/><Controls/></ReactFlow></section>;
}

function toRuleEdge(edge: Edge): WAFRuleEdge { return {id: edge.id, source: edge.source, source_handle: edge.sourceHandle ?? '', target: edge.target}; }
