import { LitElement, html } from 'lit';
import * as React from 'react';
import { createRoot, type Root } from 'react-dom/client';
import {
  applyNodeChanges,
  Background,
  BackgroundVariant,
  Controls,
  Handle,
  MiniMap,
  Position,
  ReactFlow,
  ReactFlowProvider,
  useReactFlow,
  useViewport,
  type Edge,
  type NodeChange,
  type Node,
  type NodeProps,
  type NodeTypes,
} from '@xyflow/react';

type GraphNodeResponse = {
  id: string;
  label: string;
  subtitle?: string;
  x: number;
  y: number;
  tone?: string;
};

type GraphEdgeResponse = {
  id: string;
  source: string;
  target: string;
  label?: string;
};

type GraphResponse = {
  title: string;
  nodes: GraphNodeResponse[];
  edges: GraphEdgeResponse[];
};

type MetricNodeData = {
  id: string;
  label: string;
  fullLabel: string;
  subtitle?: string;
  tone?: string;
  mode: 'default' | 'overview';
  emphasis: 'default' | 'active' | 'related' | 'muted';
  onSelect: (nodeID: string) => void;
};

const compactLabel = (label: string): string => {
  const parts = label.split('/');
  if (parts.length <= 2) {
    return label;
  }
  return parts.slice(-2).join('/');
};

const MetricNode = ({ data, selected }: NodeProps<Node<MetricNodeData>>) => {
  const { zoom } = useViewport();
  const isOverview = data.mode === 'overview';
  const isHighlighted = data.emphasis === 'active' || data.emphasis === 'related';
  const showLabel = !isOverview || zoom >= 0.2 || selected || isHighlighted;
  const showSubtitle = !isOverview
    ? Boolean(data.subtitle)
    : (zoom >= 0.64 || data.emphasis === 'active') && Boolean(data.subtitle);
  const title = data.subtitle ? `${data.fullLabel} • ${data.subtitle}` : data.fullLabel;

  let className = 'governance-node';
  if (isOverview) {
    className += ' is-overview';
  }
  if (isOverview && !showLabel) {
    className += ' is-collapsed';
  }
  if (data.emphasis === 'active') {
    className += ' is-active';
  } else if (data.emphasis === 'related') {
    className += ' is-related';
  } else if (data.emphasis === 'muted') {
    className += ' is-muted';
  }

  return (
    <div
      className={className}
      data-tone={data.tone ?? ''}
      onClick={(event) => {
        event.stopPropagation();
        data.onSelect(data.id);
      }}
      onKeyDown={(event) => {
        if (event.key === 'Enter' || event.key === ' ') {
          event.preventDefault();
          event.stopPropagation();
          data.onSelect(data.id);
        }
      }}
      role="button"
      tabIndex={0}
      title={title}
    >
      <Handle className="governance-node-handle" position={Position.Left} type="target" />
      {showLabel ? <p className="governance-node-title">{isOverview ? compactLabel(data.label) : data.label}</p> : null}
      {showSubtitle ? <p className="governance-node-subtitle">{data.subtitle}</p> : null}
      <Handle className="governance-node-handle" position={Position.Right} type="source" />
    </div>
  );
};

const nodeTypes: NodeTypes = {
  metric: MetricNode,
};

type GraphCanvasProps = {
  graph: GraphResponse;
  graphMode: 'default' | 'overview';
};

type LineageSelection = {
  highlightedNodes: Set<string>;
  highlightedEdges: Set<string>;
};

function buildGraphNodes(
  graph: GraphResponse,
  graphMode: 'default' | 'overview',
  selectedNodeId: string | null,
  lineageSelection: LineageSelection,
  onSelect: (nodeID: string) => void,
  previousNodes?: Node<MetricNodeData>[],
): Node<MetricNodeData>[] {
  const previousByID = new Map(previousNodes?.map((node) => [node.id, node]) ?? []);

  return graph.nodes.map((node) => {
    let emphasis: MetricNodeData['emphasis'] = 'default';
    if (selectedNodeId) {
      if (node.id === selectedNodeId) {
        emphasis = 'active';
      } else if (lineageSelection.highlightedNodes.has(node.id)) {
        emphasis = 'related';
      } else {
        emphasis = 'muted';
      }
    }

    const previous = previousByID.get(node.id);

    return {
      id: node.id,
      type: 'metric',
      position: previous?.position ?? { x: node.x, y: node.y },
      sourcePosition: Position.Right,
      targetPosition: Position.Left,
      data: {
        id: node.id,
        label: node.label,
        fullLabel: node.label,
        subtitle: node.subtitle,
        tone: node.tone,
        mode: graphMode,
        emphasis,
        onSelect,
      },
      draggable: true,
      selectable: true,
      selected: node.id === selectedNodeId,
    };
  });
}

function buildLineageSelection(graph: GraphResponse, selectedNodeId: string | null): LineageSelection {
  if (!selectedNodeId) {
    return {
      highlightedNodes: new Set(),
      highlightedEdges: new Set(),
    };
  }

  const outgoing = new Map<string, GraphEdgeResponse[]>();
  const incoming = new Map<string, GraphEdgeResponse[]>();

  for (const edge of graph.edges) {
    const outbound = outgoing.get(edge.source) ?? [];
    outbound.push(edge);
    outgoing.set(edge.source, outbound);

    const inbound = incoming.get(edge.target) ?? [];
    inbound.push(edge);
    incoming.set(edge.target, inbound);
  }

  const highlightedNodes = new Set<string>([selectedNodeId]);
  const highlightedEdges = new Set<string>();

  const walk = (startID: string, edgesByNode: Map<string, GraphEdgeResponse[]>, nextNodeForEdge: (edge: GraphEdgeResponse) => string) => {
    const stack = [startID];
    const visited = new Set<string>([startID]);

    while (stack.length > 0) {
      const current = stack.pop();
      if (!current) {
        continue;
      }

      for (const edge of edgesByNode.get(current) ?? []) {
        highlightedEdges.add(edge.id);
        const nextNodeID = nextNodeForEdge(edge);
        if (!highlightedNodes.has(nextNodeID)) {
          highlightedNodes.add(nextNodeID);
        }
        if (!visited.has(nextNodeID)) {
          visited.add(nextNodeID);
          stack.push(nextNodeID);
        }
      }
    }
  };

  walk(selectedNodeId, outgoing, (edge) => edge.target);
  walk(selectedNodeId, incoming, (edge) => edge.source);

  return { highlightedNodes, highlightedEdges };
}

function GraphCanvasInner({ graph, graphMode }: GraphCanvasProps) {
  const { fitView } = useReactFlow();
  const [selectedNodeId, setSelectedNodeId] = React.useState<string | null>(null);
  const [nodes, setNodes] = React.useState<Node<MetricNodeData>[]>([]);
  const lastFitKeyRef = React.useRef<string>('');

  const handleSelectNode = React.useCallback((nodeID: string) => {
    setSelectedNodeId((current) => (current === nodeID ? null : nodeID));
  }, []);

  React.useEffect(() => {
    setSelectedNodeId(null);
  }, [graph, graphMode]);

  const lineageSelection = React.useMemo(
    () => buildLineageSelection(graph, selectedNodeId),
    [graph, selectedNodeId],
  );

  React.useEffect(() => {
    setNodes((current) =>
      buildGraphNodes(graph, graphMode, selectedNodeId, lineageSelection, handleSelectNode, current),
    );
  }, [graph, graphMode, handleSelectNode, lineageSelection, selectedNodeId]);

  React.useEffect(() => {
    if (nodes.length === 0) {
      return;
    }

    const fitKey = `${graphMode}:${graph.title}:${graph.nodes.length}:${graph.edges.length}`;
    if (lastFitKeyRef.current === fitKey) {
      return;
    }
    lastFitKeyRef.current = fitKey;

    const frame = requestAnimationFrame(() => {
      void fitView({
        padding: graphMode === 'overview' ? 0.16 : 0.12,
        duration: 280,
        minZoom: 0.2,
        maxZoom: 1.2,
      });
    });

    return () => cancelAnimationFrame(frame);
  }, [fitView, graph, graphMode, nodes.length]);

  const handleNodesChange = React.useCallback(
    (changes: NodeChange<Node<MetricNodeData>>[]) => {
      setNodes((current) => applyNodeChanges(changes, current));
    },
    [],
  );

  const edges: Edge[] = React.useMemo(
    () =>
      graph.edges.map((edge) => {
        const isHighlighted = selectedNodeId ? lineageSelection.highlightedEdges.has(edge.id) : false;
        const isMuted = Boolean(selectedNodeId) && !isHighlighted;

        let className = graphMode === 'overview' ? 'governance-edge governance-edge-overview' : 'governance-edge';
        if (selectedNodeId) {
          className += isHighlighted ? ' is-lineage' : ' is-muted';
        }

        return {
          id: edge.id,
          source: edge.source,
          target: edge.target,
          label: edge.label,
          animated: false,
          className,
          selectable: true,
          style: selectedNodeId
            ? {
                stroke: isHighlighted ? 'oklch(52% 0.02 255)' : 'oklch(84% 0.006 255 / 0.45)',
                strokeWidth: isHighlighted ? 1.9 : 1,
              }
            : undefined,
        };
      }),
    [graph.edges, graphMode, lineageSelection.highlightedEdges, selectedNodeId],
  );

  return (
    <ReactFlow
      nodes={nodes}
      edges={edges}
      nodeTypes={nodeTypes}
      onNodesChange={handleNodesChange}
      defaultEdgeOptions={{
        style: {
          stroke: graphMode === 'overview' ? 'oklch(76% 0.008 255 / 0.55)' : 'oklch(71% 0.01 255)',
          strokeWidth: graphMode === 'overview' ? 1 : 1.4,
        },
      }}
      nodesDraggable={true}
      nodesConnectable={false}
      elementsSelectable={true}
      onPaneClick={() => {
        setSelectedNodeId(null);
      }}
      proOptions={{ hideAttribution: true }}
      minZoom={0.2}
      maxZoom={1.4}
    >
      <Background
        color={
          graphMode === 'overview'
            ? 'color-mix(in oklab, oklch(82% 0.006 85) 52%, white)'
            : 'color-mix(in oklab, oklch(82% 0.006 85) 68%, white)'
        }
        gap={graphMode === 'overview' ? 22 : 20}
        size={1}
        variant={BackgroundVariant.Dots}
      />
      <MiniMap />
      <Controls />
    </ReactFlow>
  );
}

function GraphCanvas({ graph, graphMode }: GraphCanvasProps) {
  return (
    <ReactFlowProvider>
      <GraphCanvasInner graph={graph} graphMode={graphMode} />
    </ReactFlowProvider>
  );
}

export class GovernanceGraphView extends LitElement {
  static properties = {
    graphTitle: { attribute: 'graph-title' },
    graphMode: { attribute: 'graph-mode' },
    graph: { type: Object },
  };

  declare graphTitle: string;

  declare graphMode: 'default' | 'overview';

  declare graph?: GraphResponse;

  private reactRoot?: Root;

  constructor() {
    super();
    this.graphTitle = 'Graph';
    this.graphMode = 'default';
    this.graph = undefined;
  }

  createRenderRoot() {
    return this;
  }

  disconnectedCallback(): void {
    this.reactRoot?.unmount();
    this.reactRoot = undefined;
    super.disconnectedCallback();
  }

  protected updated(changed: Map<string, unknown>): void {
    if (changed.has('graph') || changed.has('graphMode')) {
      this.mountReactFlow();
    }
  }

  render() {
    if (!this.graph || this.graph.nodes.length === 0) {
      return html`<div class="governance-graph-empty">No graph data available for this view.</div>`;
    }

    return html`
      <section class="governance-graph-shell" aria-label=${this.graph.title || this.graphTitle}>
        <div class="governance-graph-canvas" data-graph-root></div>
      </section>
    `;
  }

  private mountReactFlow(): void {
    const container = this.querySelector<HTMLElement>('[data-graph-root]');
    if (!container || !this.graph || this.graph.nodes.length === 0) {
      this.reactRoot?.unmount();
      this.reactRoot = undefined;
      return;
    }

    if (!this.reactRoot) {
      this.reactRoot = createRoot(container);
    }

    this.reactRoot.render(
      <GraphCanvas graph={this.graph} graphMode={this.graphMode} />,
    );
  }
}

customElements.define('governance-graph-view', GovernanceGraphView);
