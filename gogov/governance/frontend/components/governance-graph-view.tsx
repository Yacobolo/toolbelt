import { LitElement, html } from 'lit';
import * as React from 'react';
import { createRoot, type Root } from 'react-dom/client';
import {
  Background,
  Controls,
  MiniMap,
  Position,
  ReactFlow,
  ReactFlowProvider,
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
  subtitle?: string;
  tone?: string;
};

const MetricNode = ({ data }: NodeProps<Node<MetricNodeData>>) => (
  <div className="governance-node" data-tone={data.tone ?? ''}>
    <p className="governance-node-title">{data.label}</p>
    {data.subtitle ? <p className="governance-node-subtitle">{data.subtitle}</p> : null}
  </div>
);

const nodeTypes: NodeTypes = {
  metric: MetricNode,
};

export class GovernanceGraphView extends LitElement {
  static properties = {
    graphTitle: { attribute: 'graph-title' },
    graph: { type: Object },
  };

  declare graphTitle: string;

  declare graph?: GraphResponse;

  private reactRoot?: Root;

  constructor() {
    super();
    this.graphTitle = 'Graph';
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
      <section class="governance-graph-shell">
        <div class="governance-graph-title">
          <div>
            <h4>${this.graph.title || this.graphTitle}</h4>
            <p>${this.graph.nodes.length} nodes · ${this.graph.edges.length} edges</p>
          </div>
        </div>
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
        subtitle: node.subtitle,
        tone: node.tone,
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
      selectable: true,
    }));

    this.reactRoot.render(
      <ReactFlowProvider>
        <ReactFlow
          fitView
          nodes={nodes}
          edges={edges}
          nodeTypes={nodeTypes}
          nodesDraggable={false}
          nodesConnectable={false}
          elementsSelectable={true}
          proOptions={{ hideAttribution: true }}
          minZoom={0.2}
          maxZoom={1.4}
        >
          <MiniMap pannable zoomable />
          <Controls showInteractive={false} />
          <Background gap={18} size={1} />
        </ReactFlow>
      </ReactFlowProvider>,
    );
  }
}

customElements.define('governance-graph-view', GovernanceGraphView);
