import { LitElement, html } from 'lit';
import * as React from 'react';
import { createRoot, type Root } from 'react-dom/client';
import {
  Background,
  BackgroundVariant,
  Controls,
  Handle,
  MiniMap,
  Position,
  ReactFlow,
  ReactFlowProvider,
  useViewport,
  type Edge,
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
  label: string;
  fullLabel: string;
  subtitle?: string;
  tone?: string;
  mode: 'default' | 'overview';
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
  const showLabel = !isOverview || zoom >= 0.55 || selected;
  const showSubtitle = !isOverview ? Boolean(data.subtitle) : (zoom >= 0.82 || selected) && Boolean(data.subtitle);
  const title = data.subtitle ? `${data.fullLabel} • ${data.subtitle}` : data.fullLabel;

  let className = 'governance-node';
  if (isOverview) {
    className += ' is-overview';
  }
  if (isOverview && !showLabel) {
    className += ' is-collapsed';
  }

  return (
    <div className={className} data-tone={data.tone ?? ''} title={title}>
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
    if (changed.has('graph')) {
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

    const nodes: Node<MetricNodeData>[] = this.graph.nodes.map((node) => ({
      id: node.id,
      type: 'metric',
      position: { x: node.x, y: node.y },
      sourcePosition: Position.Right,
      targetPosition: Position.Left,
      data: {
        label: node.label,
        fullLabel: node.label,
        subtitle: node.subtitle,
        tone: node.tone,
        mode: this.graphMode,
      },
      draggable: false,
      selectable: true,
    }));

    const edges: Edge[] = this.graph.edges.map((edge) => ({
      id: edge.id,
      source: edge.source,
      target: edge.target,
      label: edge.label,
      animated: false,
      className: this.graphMode === 'overview' ? 'governance-edge governance-edge-overview' : 'governance-edge',
      selectable: true,
    }));

    this.reactRoot.render(
      <ReactFlowProvider>
        <ReactFlow
          fitView
          fitViewOptions={this.graphMode === 'overview' ? { padding: 0.18 } : { padding: 0.12 }}
          nodes={nodes}
          edges={edges}
          nodeTypes={nodeTypes}
          defaultEdgeOptions={{
            style: {
              stroke: this.graphMode === 'overview' ? 'oklch(76% 0.008 255 / 0.55)' : 'oklch(71% 0.01 255)',
              strokeWidth: this.graphMode === 'overview' ? 1 : 1.4,
            },
          }}
          nodesDraggable={false}
          nodesConnectable={false}
          elementsSelectable={true}
          proOptions={{ hideAttribution: true }}
          minZoom={0.2}
          maxZoom={1.4}
        >
          <Background
            color={
              this.graphMode === 'overview'
                ? 'color-mix(in oklab, oklch(82% 0.006 85) 52%, white)'
                : 'color-mix(in oklab, oklch(82% 0.006 85) 68%, white)'
            }
            gap={this.graphMode === 'overview' ? 22 : 20}
            size={1}
            variant={BackgroundVariant.Dots}
          />
          <MiniMap />
          <Controls />
        </ReactFlow>
      </ReactFlowProvider>,
    );
  }
}

customElements.define('governance-graph-view', GovernanceGraphView);
