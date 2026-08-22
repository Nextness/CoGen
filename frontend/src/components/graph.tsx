// D3 force layout and canvas rendering for the bounded relationship explorer.
import { forceCenter, forceCollide, forceLink, forceManyBody, forceSimulation } from "../../vendor/d3-force.js";
import type { SimulationNode, SimulationLink } from "../../vendor/d3-force.js";
import { currentDetailOrigin, graphFilters, humanLabel, link, list, value } from "../state.tsx";
import { h, Fragment, render as renderTree } from "../jsx/jsx-runtime.ts";
import { Pagination } from "./pagination.tsx";

/** One graph node with its resolved layout and cluster state. */
export interface GraphNode extends SimulationNode {
  label?: string;
  doi?: string;
  orcid?: string;
  author?: string;
  revision_id?: any;
  author_id?: any;
  reference_id?: any;
  cluster?: number;
  degree?: number;
}

/** One graph edge between resolved node objects. */
export interface GraphEdge extends SimulationLink {
  id?: string;
  type?: string;
  author_order?: any;
  affiliation?: string;
  shared_reference_count?: number;
}

/** One connected-cluster summary. */
export interface ClusterSummary {
  id: number;
  size: number;
}

/** The full interactive graph state owned by one mounted viewport. */
interface GraphState {
  cancelled: boolean;
  canvas: HTMLCanvasElement;
  context: CanvasRenderingContext2D | null;
  nodes: GraphNode[];
  edges: GraphEdge[];
  neighbours: Map<string | number, Set<string | number>>;
  selection: string | number | null;
  hovered: string | number | null;
  searchQuery: string;
  colors: ReturnType<typeof palette>;
  spatialIndex: { cellSize: number; cells: Map<string, GraphNode[]> } | null;
  clusterSummaries: ClusterSummary[];
  overviewOffset: { x: number; y: number };
  overviewMode: boolean;
  layoutRunning: boolean;
  view: { x: number; y: number; scale: number };
  frame: number;
  edgePage: number;
  simulation: ReturnType<typeof forceSimulation>;
  resizeObserver?: ResizeObserver;
  fullscreenHandler?: () => void;
  expandCleanup?: () => void;
  themeObserver?: MutationObserver;
  themeMedia?: MediaQueryList;
  themeHandler?: () => void;
  overviewLayout?: Array<{ id: number; x: number; y: number; radius: number }>;
}

var activeGraph: GraphState | undefined;

/** Renders an escaped graph-filter input with its current URL value. */
export function GraphField(props: { name: string; label: string; type?: string }): JSX.Element {
  const type = props.type || "text";
  return (
    <label>
      {props.label}
      <input name={props.name} type={type} value={value(props.name)} />
    </label>
  );
}

/** Returns the current graph-filter values keyed by query parameter. */
export function graphQuery(): Record<string, string> {
  const result: Record<string, string> = {};
  graphFilters.forEach((name) => {
    result[name] = value(name);
  });
  return result;
}

/** Returns a context-preserving detail link for a graph node when one exists. */
export function graphLink(node: GraphNode): string {
  if (node.type === "article") {
    return link({
      view: "article",
      article_id: node.revision_id,
      origin: currentDetailOrigin(),
    });
  }
  if (node.type === "author") {
    return link({
      view: "author",
      author_id: node.author_id,
      origin: currentDetailOrigin(),
    });
  }
  if (node.type === "reference") {
    return link({
      view: "reference",
      reference_id: node.reference_id,
      origin: currentDetailOrigin(),
    });
  }
  return "";
}

/** Returns an edge endpoint identifier from either an identifier or resolved node object. */
function endpointID(endpoint: string | number | GraphNode): string | number {
  if (endpoint && typeof endpoint === "object") return endpoint.id;
  return endpoint as string | number;
}

/** Finds deterministic connected components and maps graph nodes to cluster identifiers. */
export function graphClusters(sourceNodes: GraphNode[], sourceEdges: GraphEdge[]): { byID: Map<string | number, number>; components: ClusterSummary[] } {
  const adjacency = new Map<string | number, Set<string | number>>(sourceNodes.map((node) => {
    return [node.id, new Set()];
  }));
  sourceEdges.forEach((edge) => {
    const source = endpointID(edge.source);
    const target = endpointID(edge.target);
    if (adjacency.has(source) && adjacency.has(target)) {
      adjacency.get(source)!.add(target);
      adjacency.get(target)!.add(source);
    }
  });

  const byID = new Map<string | number, number>();
  const components: ClusterSummary[] = [];
  const ordered = Array.from(adjacency.keys()).sort();
  ordered.forEach((start) => {
    if (byID.has(start)) return;
    const cluster = components.length;
    const pending: Array<string | number> = [start];
    var size = 0;
    byID.set(start, cluster);
    while (pending.length) {
      const current = pending.pop()!;
      size += 1;
      adjacency.get(current)!.forEach((neighbour) => {
        if (!byID.has(neighbour)) {
          byID.set(neighbour, cluster);
          pending.push(neighbour);
        }
      });
    }
    components.push({ id: cluster, size: size });
  });
  return {
    byID: byID,
    components: components,
  };
}

/** Renders one legend entry with a colored mark and its label. */
function legendEntry(markClass: string, label: string): JSX.Element {
  return (
    <span>
      <i className={markClass}></i>
      {label}
    </span>
  );
}

/** Renders the interactive graph viewport, legend, and relationship table. */
export function GraphResult(props: { data: any }): JSX.Element {
  const nodes = list(props.data, ["nodes"]);
  const edges = list(props.data, ["edges"]);
  const counts = props.data.counts || {};

  if (!nodes.length) {
    return (
      <div className="rw-graph-empty">
        <h4>No relationships match these filters</h4>
        <p>Broaden the filters or choose another graph model. The selected run context has been preserved.</p>
      </div>
    );
  }

  const nodeTypes = counts.node_types || {};
  const edgeTypes = counts.edge_types || {};

  var warning: JSX.Element | null = null;
  if (props.data.truncated) {
    const matchedCount = String(counts.article_matches ?? nodes.length);
    const renderedArticles = String(counts.article_rendered ?? 0);
    const renderedNodes = String(counts.nodes_rendered ?? nodes.length);
    const renderedEdges = String(counts.edges_rendered ?? edges.length);
    warning = (
      <p className="rw-graph__truncation ui warning message" role="status">Graph results truncated. {matchedCount} articles matched; {renderedArticles} articles, {renderedNodes} nodes, and {renderedEdges} edges are rendered. Refine the article filters to inspect relationships outside this bounded result.</p>
    );
  }

  const entityDefinitions = [
    ["article", "Article revision", "article"],
    ["author", "Author occurrence", "author"],
    ["reference", "Reference mention", "reference"],
    ["referenced_author", "Referenced-author string", "referenced-author"],
  ];
  const relationshipDefinitions = [
    ["authorship", "Authorship", ""],
    ["reference", "Reference mention", ""],
    ["reference_author", "Referenced author", "derived"],
    ["citation", "Internal citation", "directed"],
    ["coauthor", "Co-author", "derived"],
    ["shared_reference", "Shared reference", "derived"],
  ];
  const visibleEntityDefinitions = entityDefinitions.filter(([type]) => {
    return Number(nodeTypes[type] || 0) > 0;
  });
  const entityLegend = visibleEntityDefinitions.map(([type, label, markClass]) => {
    const markClassValue = `rw-graph__legend-mark rw-graph__legend-mark--${markClass}`;
    return legendEntry(markClassValue, label);
  });
  const visibleRelationshipDefinitions = relationshipDefinitions.filter(([type]) => {
    return Number(edgeTypes[type] || 0) > 0;
  });
  const relationshipLegend = visibleRelationshipDefinitions.map(([type, label, lineClass]) => {
    var modifier = "";
    if (lineClass) modifier = ` rw-graph__legend-line--${lineClass}`;
    const lineClassValue = `rw-graph__legend-line${modifier}`;
    return legendEntry(lineClassValue, label);
  });

  var largeGraphHint = "";
  if (nodes.length > 1500) largeGraphHint = "Large networks open as a connected-cluster overview where bubble size reflects entity count; zoom in to reveal individual entities. ";

  const toolbarMarkup = (
    <div className="rw-graph__toolbar">
      <div className="rw-graph__search"><input type="text" id="graph-node-search" placeholder={"Search nodes\u2026"} aria-label="Search nodes by name or DOI" /></div>
      <button type="button" id="graph-fit" className="ui button">Fit graph</button>
      <button type="button" id="graph-run-layout" className="ui basic button">Re-run layout</button>
      <button type="button" id="graph-clear-selection" className="ui basic button" disabled>Show full graph</button>
      <span className="rw-graph__zoom" id="graph-zoom-indicator" role="status">100%</span>
      <span id="graph-layout-status" role="status" aria-live="polite">Preparing physics layout</span>
      <button type="button" id="graph-expand" className="ui button">Expand graph</button>
      <button type="button" id="graph-export-png" className="ui button" title="Download graph as PNG">Export PNG</button>
    </div>
  );
  const searchResultsMarkup = (
    <section className="rw-graph__search-results" aria-label="Matching graph nodes">
      <p id="graph-search-summary" role="status">Enter a node name, DOI, ORCID, or identifier to search the rendered graph.</p>
      <ul id="graph-search-results"></ul>
    </section>
  );
  const helpMarkup = (
    <p className="rw-graph__help">
      Shape identifies the entity type, color identifies a connected cluster, and size reflects visible relationship count. {" "}
      {largeGraphHint}
      Select a node to isolate its immediate neighbourhood; select it again or click the background to clear it. {" "}
      Use the mouse wheel to zoom around the pointer, or drag with the secondary mouse button to pan without selecting nodes.
    </p>
  );
  const legendMarkup = (
    <div className="rw-graph__legend" aria-label="Visible graph encodings">
      <div className="rw-graph__legend-group">
        <strong>Entities</strong>
        {entityLegend}
      </div>
      <div className="rw-graph__legend-group">
        <strong>Relationships</strong>
        {relationshipLegend}
      </div>
    </div>
  );

  return (
    <Fragment>
      {warning}
      <div className="rw-graph__viewport" id="graph-viewport">
        {toolbarMarkup}
        {searchResultsMarkup}
        {helpMarkup}
        <div className="rw-graph__wrap">
          {legendMarkup}
          <canvas className="rw-graph__canvas"></canvas>
        </div>
      </div>
      <section className="rw-graph__selection" id="graph-selection" tabindex={-1} aria-live="polite"><p>Select a node to inspect its direct relationships.</p></section>
      <section className="rw-graph__edges">
        <h3>Relationship table</h3>
        <p>The exact, paginated relationship records behind the graph.</p>
        <div id="graph-edge-rows"></div>
      </section>
    </Fragment>
  );
}

/** Calculates a node radius from entity type and visible degree. */
function nodeSize(node: GraphNode, degree: number, maxDegree: number): number {
  if (node.type === "reference" || node.type === "referenced_author") return 7;
  var min = 8;
  var max = 18;
  if (node.type === "article") {
    min = 9;
    max = 22;
  }
  return min + (max - min) * Math.sqrt(degree / Math.max(maxDegree, 1));
}

/** Returns a deterministic unsigned hash for stable graph placement. */
function hash(value: any): number {
  var result = 2166136261;
  for (const character of String(value)) {
    result = Math.imul(result ^ character.charCodeAt(0), 16777619);
  }
  return result >>> 0;
}

/** Reads graph colors from active CSS custom properties with safe fallbacks. */
function palette() {
  const css = getComputedStyle(document.documentElement);
  /** Returns a CSS custom property value with a fallback. */
  function get(name: string, fallback: string): string {
    return css.getPropertyValue(name).trim() || fallback;
  }
  return {
    article: get("--accent", "#0b5e8e"),
    author: get("--warning", "#8f5f00"),
    reference: get("--success", "#2f6f52"),
    edge: get("--border-strong", "#adb9c2"),
    muted: get("--border", "#d2d9de"),
    text: get("--text", "#18232c"),
    surface: get("--surface", "#ffffff"),
    focus: get("--focus", "#d36d00"),
    clusters: [
      get("--graph-cluster-1", "#236b8e"), get("--graph-cluster-2", "#8b6417"),
      get("--graph-cluster-3", "#35765b"), get("--graph-cluster-4", "#6e58a3"),
      get("--graph-cluster-5", "#985468"), get("--graph-cluster-6", "#3b7c83")
    ]
  };
}

/** Draws a diamond path for reference nodes. */
function drawDiamond(context: CanvasRenderingContext2D, x: number, y: number, radius: number): void {
  context.beginPath();
  context.moveTo(x, y - radius);
  context.lineTo(x + radius, y);
  context.lineTo(x, y + radius);
  context.lineTo(x - radius, y);
  context.closePath();
}

/** Draws a triangle path for referenced-author nodes. */
function drawTriangle(context: CanvasRenderingContext2D, x: number, y: number, radius: number): void {
  context.beginPath();
  context.moveTo(x, y - radius);
  context.lineTo(x + radius, y + radius);
  context.lineTo(x - radius, y + radius);
  context.closePath();
}

/** Returns the user-facing label for a graph edge type and its relevant metadata. */
function relationshipLabel(edge: GraphEdge): string {
  if (edge.type === "authorship") {
    var order = "";
    if (edge.author_order) order = `, author ${edge.author_order}`;
    return `Authorship${order}`;
  }
  if (edge.type === "citation") {
    return "Internal citation";
  }
  if (edge.type === "reference") {
    return "Reference mention";
  }
  if (edge.type === "reference_author") {
    return "Referenced-author string";
  }
  if (edge.type === "coauthor") {
    return "Co-author";
  }
  if (edge.type === "shared_reference") {
    return "Shared reference";
  }
  return edge.type || "Relationship";
}

/** Destroys the active graph simulation, observers, and animation frame. */
export function destroyGraph(): void {
  if (!activeGraph) return;
  activeGraph.cancelled = true;
  activeGraph.simulation.stop();
  if (activeGraph.resizeObserver) {
    activeGraph.resizeObserver.disconnect();
  }
  if (activeGraph.fullscreenHandler) {
    document.removeEventListener("fullscreenchange", activeGraph.fullscreenHandler);
  }
  if (activeGraph.expandCleanup) activeGraph.expandCleanup();
  if (activeGraph.themeObserver) activeGraph.themeObserver.disconnect();
  if (activeGraph.themeMedia && activeGraph.themeHandler) {
    activeGraph.themeMedia.removeEventListener("change", activeGraph.themeHandler);
  }
  cancelAnimationFrame(activeGraph.frame);
  activeGraph = undefined;
}

/** Resolves one node's initial layout position and cluster metadata. */
function layoutNode(node: GraphNode, clusters: { byID: Map<string | number, number>; components: ClusterSummary[] }, degree: Map<string | number, number>, maxDegree: number): GraphNode {
  const angle = hash(node.id) / 0xffffffff * Math.PI * 2;
  const cluster = clusters.byID.get(node.id) || 0;
  const clusterAngle = cluster / Math.max(clusters.components.length, 1) * Math.PI * 2;
  var clusterDistance = 0;
  if (clusters.components.length > 1) clusterDistance = 210;
  const distance = 40 + hash(node.id + ":distance") % 150;
  return {
    ...node,
    cluster: cluster,
    degree: degree.get(node.id) || 0,
    radius: nodeSize(node, degree.get(node.id) || 0, maxDegree),
    x: Math.cos(clusterAngle) * clusterDistance + Math.cos(angle) * distance,
    y: Math.sin(clusterAngle) * clusterDistance + Math.sin(angle) * distance,
  };
}

/** Mounts the interactive graph viewport and its force-layout simulation. */
export function mountGraph(data: any): void {
  destroyGraph();
  const canvasElement = document.querySelector<HTMLCanvasElement>(".rw-graph__canvas, .graph-canvas");
  if (!canvasElement) return;
  const canvas = canvasElement;

  const context = canvas.getContext("2d");
  const status = document.querySelector<HTMLElement>("#graph-layout-status");
  const selectionPanel = document.querySelector<HTMLElement>("#graph-selection");
  const zoomIndicator = document.querySelector<HTMLElement>("#graph-zoom-indicator");
  const sourceNodes: GraphNode[] = list(data, ["nodes"]);
  const sourceEdges: GraphEdge[] = list(data, ["edges"]);
  const clusters = graphClusters(sourceNodes, sourceEdges);

  const degree = new Map<string | number, number>(sourceNodes.map((node) => {
    return [node.id, 0];
  }));
  sourceEdges.forEach((edge) => {
    degree.set(endpointID(edge.source), (degree.get(endpointID(edge.source)) || 0) + 1);
    degree.set(endpointID(edge.target), (degree.get(endpointID(edge.target)) || 0) + 1);
  });
  const maxDegree = Math.max.apply(null, Array.from(degree.values()).concat([1]));

  const nodes: GraphNode[] = sourceNodes.map((node) => {
    return layoutNode(node, clusters, degree, maxDegree);
  });

  const nodeByID = new Map<string | number, GraphNode>(nodes.map((node) => {
    return [node.id, node];
  }));

  const resolvableEdges = sourceEdges.filter((edge) => {
    return nodeByID.has(endpointID(edge.source)) && nodeByID.has(endpointID(edge.target));
  });
  const edges: GraphEdge[] = resolvableEdges.map((edge, index) => {
    return {
      ...edge,
      id: edge.id || `${edge.type}:${index}`,
      source: nodeByID.get(endpointID(edge.source))!,
      target: nodeByID.get(endpointID(edge.target))!,
    };
  });

  const neighbours = new Map<string | number, Set<string | number>>(nodes.map((node) => {
    return [node.id, new Set()];
  }));
  edges.forEach((edge) => {
    neighbours.get(endpointID(edge.source))!.add(endpointID(edge.target));
    neighbours.get(endpointID(edge.target))!.add(endpointID(edge.source));
  });

  const graph: GraphState = {
    cancelled: false,
    canvas: canvas,
    context: context,
    nodes: nodes,
    edges: edges,
    neighbours: neighbours,
    selection: null,
    hovered: null,
    searchQuery: "",
    colors: palette(),
    spatialIndex: null,
    clusterSummaries: clusterOverview(nodes),
    overviewOffset: {
      x: 0,
      y: 0,
    },
    overviewMode: false,
    layoutRunning: false,
    view: {
      x: 0,
      y: 0,
      scale: 1,
    },
    frame: 0,
    edgePage: 1,
    // Assigned after construction; the simulation is created over the nodes below.
    simulation: undefined as any,
  };
  activeGraph = graph;

  const largeGraph = nodes.length > 2500;
  var linkStrength = 0.72;
  if (largeGraph) linkStrength = 0.34;
  var chargeDistance = 720;
  if (largeGraph) chargeDistance = 380;
  var alphaDecay = 0.028;
  if (nodes.length > 5000) alphaDecay = 0.16;
  else if (nodes.length > 2500) alphaDecay = 0.11;
  else if (nodes.length > 900) alphaDecay = 0.065;
  const linkForce = forceLink(edges)
    .id((node) => {
      return node.id;
    })
    .distance((edge) => {
      if (edge.type === "citation") {
        return 125;
      }
      if (edge.type === "coauthor" || edge.type === "shared_reference") {
        return 145;
      }
      if (edge.type === "reference_author") {
        return 70;
      }
      return 92;
    })
    .strength(linkStrength);
  const chargeForce = forceManyBody()
    .strength((node) => {
      if (node.type === "article") {
        return -260;
      }
      return -140;
    })
    .distanceMax(chargeDistance);
  const simulation = forceSimulation(nodes)
    .force("link", linkForce)
    .force("charge", chargeForce)
    .force("center", forceCenter(0, 0))
    .alpha(1)
    .alphaDecay(alphaDecay)
    .stop();

  if (!largeGraph) {
    var collisionIterations = 2;
    if (nodes.length > 900) collisionIterations = 1;
    const collisionForce = forceCollide()
      .radius((node) => {
        return node.radius + 7;
      })
      .strength(0.92)
      .iterations(collisionIterations);
    simulation.force("collision", collisionForce);
  }

  graph.simulation = simulation;

  /** Resizes the backing canvas for its layout size and device pixel ratio. */
  function resize(): void {
    const rect = canvas.getBoundingClientRect();
    const ratio = Math.min(window.devicePixelRatio || 1, 2);
    canvas.width = Math.max(1, Math.round(rect.width * ratio));
    canvas.height = Math.max(1, Math.round(rect.height * ratio));
    draw(graph);
  }

  graph.resizeObserver = new ResizeObserver(resize);
  graph.resizeObserver.observe(canvas);
  resize();

  const setSelection = bindInteractions(graph, status, selectionPanel, zoomIndicator);
  bindGraphSearch(graph, setSelection);
  bindGraphExport(graph, data);
  bindGraphExpand(graph);
  bindGraphTheme(graph);
  runLayout(graph, status);
}

/** Returns the radius-aware world-coordinate bounds of graph nodes. */
function graphBounds(nodes: GraphNode[]): { minX: number; maxX: number; minY: number; maxY: number } {
  if (!nodes.length) {
    return {
      minX: -1,
      maxX: 1,
      minY: -1,
      maxY: 1,
    };
  }
  const initialBounds = {
    minX: Infinity,
    maxX: -Infinity,
    minY: Infinity,
    maxY: -Infinity,
  };
  return nodes.reduce((bounds, node) => {
    return {
      minX: Math.min(bounds.minX, node.x! - node.radius!),
      maxX: Math.max(bounds.maxX, node.x! + node.radius!),
      minY: Math.min(bounds.minY, node.y! - node.radius!),
      maxY: Math.max(bounds.maxY, node.y! + node.radius!),
    };
  }, initialBounds);
}

/** Returns connected-cluster sizes ordered from largest to smallest. */
function clusterOverview(nodes: GraphNode[]): ClusterSummary[] {
  const grouped = new Map<number, ClusterSummary>();
  nodes.forEach((node) => {
    if (!grouped.has(node.cluster!)) {
      grouped.set(node.cluster!, {
        id: node.cluster!,
        size: 0,
      });
    }
    const cluster = grouped.get(node.cluster!)!;
    cluster.size += 1;
  });
  return Array.from(grouped.values()).sort((a, b) => {
    return b.size - a.size;
  });
}

/** Draws one overview bubble and its label, returning its layout entry. */
function drawClusterBubble(context: CanvasRenderingContext2D, cluster: ClusterSummary, colors: ReturnType<typeof palette>, x: number, y: number, radius: number): { id: number; x: number; y: number; radius: number } {
  context.globalAlpha = 0.9;
  context.fillStyle = colors.clusters[cluster.id % colors.clusters.length];
  context.strokeStyle = colors.text;
  context.lineWidth = 1.2;
  context.beginPath();
  context.arc(x, y, radius, 0, Math.PI * 2);
  context.fill();
  context.stroke();

  context.globalAlpha = 1;
  context.fillStyle = colors.text;
  context.textAlign = "center";
  context.textBaseline = "top";
  context.font = "600 10px ui-sans-serif, system-ui";
  context.fillText(`Cluster ${cluster.id + 1}`, x, y + radius + 5);
  context.font = "9px ui-sans-serif, system-ui";
  var entityLabel = "entities";
  if (cluster.size === 1) entityLabel = "entity";
  context.fillText(`${cluster.size.toLocaleString()} ${entityLabel}`, x, y + radius + 17);
  return {
    id: cluster.id,
    x: x,
    y: y,
    radius: radius,
  };
}

/** Draws the connected-cluster overview bubbles and returns their layout. */
function drawClusterOverview(context: CanvasRenderingContext2D, clusters: ClusterSummary[], colors: ReturnType<typeof palette>, width: number, height: number, offset: { x: number; y: number }, legendInset: number): Array<{ id: number; x: number; y: number; radius: number }> {
  const top = 28;
  const horizontalPadding = 32;
  const bottomPadding = Math.max(28, legendInset);
  const usableWidth = Math.max(1, width - horizontalPadding * 2);
  const usableHeight = Math.max(1, height - top - bottomPadding);
  const columns = Math.max(1, Math.ceil(Math.sqrt(clusters.length * usableWidth / usableHeight)));
  const rows = Math.max(1, Math.ceil(clusters.length / columns));
  const cellWidth = usableWidth / columns;
  const cellHeight = usableHeight / rows;
  const clusterSizes = clusters.map((cluster) => {
    return cluster.size;
  });
  const maximumSize = Math.max.apply(null, clusterSizes.concat([1]));

  return clusters.map((cluster, index) => {
    const column = index % columns;
    const row = Math.floor(index / columns);
    const x = horizontalPadding + cellWidth * (column + 0.5) + offset.x;
    const y = top + cellHeight * (row + 0.36) + offset.y;
    const scale = 0.5 + 0.5 * Math.sqrt(cluster.size / maximumSize);
    const radius = Math.max(12, Math.min(cellWidth, cellHeight) * 0.34 * scale);
    return drawClusterBubble(context, cluster, colors, x, y, radius);
  });
}

/** Adjusts the graph transform to fit all node bounds in the canvas. */
function fitGraph(graph: GraphState): void {
  const bounds = graphBounds(graph.nodes);
  const width = Math.max(bounds.maxX - bounds.minX, 1);
  const height = Math.max(bounds.maxY - bounds.minY, 1);
  graph.view.scale = Math.min(
    graph.canvas.clientWidth / (width + 100),
    graph.canvas.clientHeight / (height + 100),
    2.5
  );
  graph.view.x = -(bounds.minX + bounds.maxX) / 2 * graph.view.scale;
  graph.view.y = -(bounds.minY + bounds.maxY) / 2 * graph.view.scale;
  graph.overviewOffset.x = 0;
  graph.overviewOffset.y = 0;
  draw(graph);
}

/** Advances the force simulation in animation-frame batches and finalizes spatial state. */
function runLayout(graph: GraphState, status: HTMLElement | null): void {
  graph.simulation.alpha(1).restart().stop();
  graph.spatialIndex = null;
  graph.layoutRunning = true;
  const reduced = matchMedia("(prefers-reduced-motion: reduce)").matches;
  var tickLimit = 220;
  if (reduced) tickLimit = 100;
  if (graph.nodes.length > 5000) tickLimit = 20;
  else if (graph.nodes.length > 2500) tickLimit = 34;
  else if (graph.nodes.length > 900) tickLimit = 64;
  else if (graph.nodes.length > 250) {
    if (reduced) tickLimit = 90;
    else tickLimit = 140;
  }

  var ticksPerFrame = 3;
  if (graph.nodes.length > 5000) ticksPerFrame = 10;
  else if (graph.nodes.length > 2500) ticksPerFrame = 7;
  else if (graph.nodes.length > 900) ticksPerFrame = 5;

  var ticks = 0;

  /** Advances and redraws the next batch of force-layout ticks. */
  function next(): void {
    if (graph.cancelled) {
      return;
    }
    for (var i = 0; i < ticksPerFrame && ticks < tickLimit; i += 1) {
      graph.simulation.tick();
      ticks += 1;
    }
    draw(graph);
    if (ticks < tickLimit) {
      graph.frame = requestAnimationFrame(next);
    } else {
      graph.simulation.stop();
      graph.clusterSummaries = clusterOverview(graph.nodes);
      fitGraph(graph);
      graph.spatialIndex = buildSpatialIndex(graph.nodes);
      graph.layoutRunning = false;
      if (status) {
        if (reduced) {
          status.textContent = "Physics layout placed with reduced motion";
        } else {
          status.textContent = "Physics layout settled";
        }
      }
    }
  }

  if (status) {
    status.textContent = "Running physics layout";
  }
  graph.frame = requestAnimationFrame(next);
}

/** Draws the current graph state to the backing canvas. */
function draw(graph: GraphState): void {
  const canvas = graph.canvas;
  const context = graph.context!;
  const view = graph.view;
  const nodes = graph.nodes;
  const edges = graph.edges;
  const selection = graph.selection;
  const hovered = graph.hovered;
  const searchQuery = graph.searchQuery;

  const ratio = Math.min(window.devicePixelRatio || 1, 2);
  const width = canvas.clientWidth;
  const height = canvas.clientHeight;
  const colors = graph.colors;

  context.setTransform(ratio, 0, 0, ratio, 0, 0);
  context.clearRect(0, 0, width, height);
  context.fillStyle = colors.surface;
  context.fillRect(0, 0, width, height);

  const clusterOverviewVisible = nodes.length > 1500 && !selection && !searchQuery && view.scale < 0.2;
  graph.overviewMode = clusterOverviewVisible;
  if (clusterOverviewVisible) {
    const legend = canvas.parentElement!.querySelector(".rw-graph__legend") as HTMLElement | null;
    var legendInset = 24;
    if (legend) legendInset = legend.offsetHeight + 24;
    graph.overviewLayout = drawClusterOverview(context, graph.clusterSummaries, colors, width, height, graph.overviewOffset, legendInset);
    context.setTransform(ratio, 0, 0, ratio, 0, 0);
    context.globalAlpha = 1;
    return;
  }

  context.translate(width / 2 + view.x, height / 2 + view.y);
  context.scale(view.scale, view.scale);

  var focused: Set<string | number> | null = null;
  if (selection) {
    focused = new Set([selection, ...(graph.neighbours.get(selection) || [])]);
  }

  // Determine search-matched node IDs
  var searchMatchIds: Set<string | number> | null = null;
  if (searchQuery) {
    searchMatchIds = new Set();
    nodes.forEach((node) => {
      const searchableParts = [node.label, node.id, node.doi, node.orcid, node.author].filter(Boolean);
      const searchable = searchableParts.join(" ").toLocaleLowerCase();
      if (searchable.includes(searchQuery)) {
        searchMatchIds!.add(node.id);
      }
    });
  }

  for (const edge of edges) {
    var relevant = true;
    if (focused) {
      relevant = endpointID(edge.source) === selection || endpointID(edge.target) === selection;
    }

    if (focused && !relevant) context.globalAlpha = 0.18;
    else context.globalAlpha = 0.82;

    if (relevant) context.strokeStyle = colors.edge;
    else context.strokeStyle = colors.muted;

    if (relevant) context.lineWidth = 1.45 / view.scale;
    else context.lineWidth = 1 / view.scale;

    if (edge.type === "coauthor" || edge.type === "shared_reference") {
      context.setLineDash([6 / view.scale, 4 / view.scale]);
    } else if (edge.type === "reference_author") {
      context.setLineDash([2 / view.scale, 4 / view.scale]);
    } else {
      context.setLineDash([]);
    }
    context.beginPath();
    context.moveTo((edge.source as GraphNode).x!, (edge.source as GraphNode).y!);
    context.lineTo((edge.target as GraphNode).x!, (edge.target as GraphNode).y!);
    context.stroke();
    context.setLineDash([]);

    if (edge.type === "citation" && relevant) {
      drawArrow(context, edge.source as GraphNode, edge.target as GraphNode, 5 / view.scale, colors.edge);
    }
  }

  for (const node of nodes) {
    var nodeRelevant = true;
    if (focused) {
      nodeRelevant = focused.has(node.id);
    }

    // Search highlight: dim non-matching nodes, highlight matching nodes
    const isSearchMatch = searchMatchIds && searchMatchIds.has(node.id);
    const hasSearch = searchMatchIds !== null;

    if (hasSearch && !isSearchMatch) context.globalAlpha = 0.12;
    else if (nodeRelevant) context.globalAlpha = 1;
    else context.globalAlpha = 0.22;

    context.fillStyle = colors.clusters[(node.cluster || 0) % colors.clusters.length];

    if (node.id === selection) {
      context.strokeStyle = colors.focus;
      context.lineWidth = 2.6 / view.scale;
    } else {
      context.strokeStyle = colors.text;
      context.lineWidth = 1.1 / view.scale;
    }

    if (node.type === "reference") {
      drawDiamond(context, node.x!, node.y!, node.radius!);
    } else if (node.type === "author") {
      context.beginPath();
      context.rect(node.x! - node.radius!, node.y! - node.radius!, node.radius! * 2, node.radius! * 2);
    } else if (node.type === "referenced_author") {
      drawTriangle(context, node.x!, node.y!, node.radius!);
    } else {
      context.beginPath();
      context.arc(node.x!, node.y!, node.radius!, 0, Math.PI * 2);
    }

    context.fill();
    context.stroke();

    // Draw search highlight ring
    if (isSearchMatch) {
      context.strokeStyle = colors.focus;
      context.lineWidth = 2.5 / view.scale;
      if (node.type === "reference") {
        drawDiamond(context, node.x!, node.y!, node.radius! + 4 / view.scale);
      } else if (node.type === "author") {
        context.beginPath();
        const searchRadius = node.radius! + 4 / view.scale;
        context.rect(node.x! - searchRadius, node.y! - searchRadius, searchRadius * 2, searchRadius * 2);
      } else if (node.type === "referenced_author") {
        drawTriangle(context, node.x!, node.y!, node.radius! + 4 / view.scale);
      } else {
        context.beginPath();
        context.arc(node.x!, node.y!, node.radius! + 4 / view.scale, 0, Math.PI * 2);
      }
      context.stroke();
    }
  }

  var labelNode: GraphNode | null = null;
  if (hovered) {
    labelNode = nodes.find((node) => {
      return node.id === hovered;
    }) || null;
  }
  if (!labelNode && selection) {
    labelNode = nodes.find((node) => {
      return node.id === selection;
    }) || null;
  }

  if (labelNode) {
    const label = String(labelNode.label || labelNode.id);
    context.globalAlpha = 1;
    context.font = `${12 / view.scale}px ui-sans-serif, system-ui`;
    context.textBaseline = "middle";
    const labelX = labelNode.x! + labelNode.radius! + 7 / view.scale;
    const labelWidth = context.measureText(label).width;
    context.fillStyle = colors.surface;
    context.fillRect(labelX - 3 / view.scale, labelNode.y! - 10 / view.scale, labelWidth + 6 / view.scale, 20 / view.scale);
    context.fillStyle = colors.text;
    context.fillText(label, labelX, labelNode.y!);
  }

  context.setTransform(ratio, 0, 0, ratio, 0, 0);
  context.globalAlpha = 1;
}

/** Draws a directional arrowhead at the target end of a citation edge. */
function drawArrow(context: CanvasRenderingContext2D, source: GraphNode, target: GraphNode, radius: number, color: string): void {
  const angle = Math.atan2(target.y! - source.y!, target.x! - source.x!);
  const x = target.x! - Math.cos(angle) * (target.radius! + 2);
  const y = target.y! - Math.sin(angle) * (target.radius! + 2);
  context.fillStyle = color;
  context.beginPath();
  context.moveTo(x, y);
  context.lineTo(x - Math.cos(angle - 0.5) * radius, y - Math.sin(angle - 0.5) * radius);
  context.lineTo(x - Math.cos(angle + 0.5) * radius, y - Math.sin(angle + 0.5) * radius);
  context.closePath();
  context.fill();
}

/** Converts a pointer event from canvas coordinates to graph world coordinates. */
function graphCoordinates(graph: GraphState, event: MouseEvent): { x: number; y: number } {
  const rect = graph.canvas.getBoundingClientRect();
  return {
    x: (event.clientX - rect.left - rect.width / 2 - graph.view.x) / graph.view.scale,
    y: (event.clientY - rect.top - rect.height / 2 - graph.view.y) / graph.view.scale,
  };
}

/** Returns a zoom transform that keeps the selected screen point stationary. */
export function zoomViewAt(view: { x: number; y: number; scale: number }, screenPoint: { x: number; y: number }, nextScale: number): { x: number; y: number; scale: number } {
  const worldX = (screenPoint.x - view.x) / view.scale;
  const worldY = (screenPoint.y - view.y) / view.scale;
  return {
    x: screenPoint.x - worldX * nextScale,
    y: screenPoint.y - worldY * nextScale,
    scale: nextScale,
  };
}

/** Returns the overview cluster hit by a pointer event, when any. */
function nearestOverviewCluster(graph: GraphState, event: MouseEvent): { id: number; x: number; y: number; radius: number } | undefined {
  const rect = graph.canvas.getBoundingClientRect();
  const x = event.clientX - rect.left;
  const y = event.clientY - rect.top;
  return (graph.overviewLayout || []).find((cluster) => {
    return Math.hypot(cluster.x - x, cluster.y - y) <= cluster.radius + 8;
  });
}

/** Focuses the viewport on one connected cluster. */
function focusCluster(graph: GraphState, clusterID: number): void {
  const nodes = graph.nodes.filter((node) => {
    return node.cluster === clusterID;
  });
  const bounds = graphBounds(nodes);
  const width = Math.max(bounds.maxX - bounds.minX, 1);
  const height = Math.max(bounds.maxY - bounds.minY, 1);
  graph.view.scale = Math.max(0.22, Math.min(
    graph.canvas.clientWidth / (width + 100),
    graph.canvas.clientHeight / (height + 100),
    2.5
  ));
  graph.view.x = -(bounds.minX + bounds.maxX) / 2 * graph.view.scale;
  graph.view.y = -(bounds.minY + bounds.maxY) / 2 * graph.view.scale;
  draw(graph);
}

/** Builds a spatial index over node positions for fast hit testing. */
function buildSpatialIndex(nodes: GraphNode[]): { cellSize: number; cells: Map<string, GraphNode[]> } {
  const cellSize = 64;
  const cells = new Map<string, GraphNode[]>();
  nodes.forEach((node) => {
    const key = `${Math.floor(node.x! / cellSize)}:${Math.floor(node.y! / cellSize)}`;
    if (!cells.has(key)) cells.set(key, []);
    cells.get(key)!.push(node);
  });
  return {
    cellSize: cellSize,
    cells: cells,
  };
}

/** Returns nodes in the spatial-index cell surrounding a graph point. */
function nearbyNodes(index: { cellSize: number; cells: Map<string, GraphNode[]> }, point: { x: number; y: number }): GraphNode[] {
  const x = Math.floor(point.x / index.cellSize);
  const y = Math.floor(point.y / index.cellSize);
  const candidates: GraphNode[] = [];
  for (var dx = -1; dx <= 1; dx += 1) {
    for (var dy = -1; dy <= 1; dy += 1) {
      const bucket = index.cells.get(`${x + dx}:${y + dy}`);
      if (bucket) candidates.push(...bucket);
    }
  }
  return candidates;
}

/** Returns the closest selectable node within its hit radius. */
function nearestNode(graph: GraphState, point: { x: number; y: number }): GraphNode | null {
  var best: { node: GraphNode; distance: number } | null = null;
  if (graph.overviewMode) {
    return null;
  }
  if (graph.layoutRunning && graph.nodes.length > 2500) {
    return null;
  }
  var candidates = graph.nodes;
  if (graph.spatialIndex) candidates = nearbyNodes(graph.spatialIndex, point);
  for (const node of candidates) {
    const distance = Math.hypot(node.x! - point.x, node.y! - point.y);
    if (distance <= node.radius! + 6) {
      if (!best || distance < best.distance) {
        best = {
          node: node,
          distance: distance,
        };
      }
    }
  }
  if (best) return best.node;
  return null;
}

/** Binds pointer, keyboard, and toolbar interactions for the graph viewport. */
function bindInteractions(graph: GraphState, status: HTMLElement | null, selectionPanel: HTMLElement | null, zoomIndicator: HTMLElement | null): (id: string | number | null) => void {
  var drag: any = null;
  const dragThreshold = 4;

  /** Sets the selected node and refreshes the inspection panel and edge table. */
  function setSelection(id: string | number | null): void {
    graph.selection = id;
    graph.edgePage = 1;
    const clearButton = document.querySelector<HTMLButtonElement>("#graph-clear-selection");
    if (clearButton) {
      clearButton.disabled = !id;
    }
    if (id) {
      const node = graph.nodes.find((n) => {
        return n.id === id;
      });
      const neighbours = graph.neighbours.get(id)!.size;
      if (selectionPanel) {
        const selectionMarkup = <SelectionMarkup node={node} neighbours={neighbours} />;
        renderTree(selectionMarkup, selectionPanel);
      }
    } else if (selectionPanel) {
      const emptySelectionMarkup = <p>Select a node to inspect its direct relationships.</p>;
      renderTree(emptySelectionMarkup, selectionPanel);
    }
    const url = new URL(location.href);
    if (id) {
      url.searchParams.set("node", String(id));
    } else {
      url.searchParams.delete("node");
    }
    history.replaceState({}, "", url.toString());
    draw(graph);
    renderEdgePage(graph);
  }

  /** Updates the visible zoom percentage indicator. */
  function updateZoomDisplay(): void {
    if (zoomIndicator) {
      zoomIndicator.textContent = `${Math.round(graph.view.scale * 100)}%`;
    }
  }

  graph.canvas.addEventListener("pointerdown", (event) => {
    if (event.button === 2) {
      event.preventDefault();
      drag = {
        x: event.clientX,
        y: event.clientY,
        viewX: graph.view.x,
        viewY: graph.view.y,
        overviewX: graph.overviewOffset.x,
        overviewY: graph.overviewOffset.y,
        overview: graph.overviewMode,
        secondary: true,
        moved: false,
      };
      graph.canvas.style.cursor = "grabbing";
      graph.canvas.setPointerCapture(event.pointerId);
      return;
    }
    if (event.button !== 0) {
      return;
    }
    if (graph.overviewMode) {
      const cluster = nearestOverviewCluster(graph, event);
      if (cluster) {
        focusCluster(graph, cluster.id);
        updateZoomDisplay();
        return;
      }
    }
    const node = nearestNode(graph, graphCoordinates(graph, event));
    if (node) {
      drag = {
        node: node,
        x: event.clientX,
        y: event.clientY,
        moved: false,
      };
    } else {
      drag = {
        x: event.clientX,
        y: event.clientY,
        viewX: graph.view.x,
        viewY: graph.view.y,
        overviewX: graph.overviewOffset.x,
        overviewY: graph.overviewOffset.y,
        overview: graph.overviewMode,
        background: true,
        moved: false,
      };
    }
    graph.canvas.setPointerCapture(event.pointerId);
  });

  graph.canvas.addEventListener("pointermove", (event) => {
    if (drag) {
      drag.moved = drag.moved || Math.hypot(event.clientX - drag.x, event.clientY - drag.y) > dragThreshold;
      if (drag.node) {
        return;
      }
      if (drag.overview) {
        graph.overviewOffset.x = drag.overviewX + event.clientX - drag.x;
        graph.overviewOffset.y = drag.overviewY + event.clientY - drag.y;
      } else {
        graph.view.x = drag.viewX + event.clientX - drag.x;
        graph.view.y = drag.viewY + event.clientY - drag.y;
      }
      draw(graph);
      return;
    }
    if (graph.overviewMode) {
      var overviewCursor = "grab";
      if (nearestOverviewCluster(graph, event)) overviewCursor = "pointer";
      graph.canvas.style.cursor = overviewCursor;
      return;
    }
    const node = nearestNode(graph, graphCoordinates(graph, event));
    var hovered: string | number | null = null;
    if (node) hovered = node.id;
    if (hovered !== graph.hovered) {
      graph.hovered = hovered;
      if (hovered) graph.canvas.style.cursor = "pointer";
      else graph.canvas.style.cursor = "grab";
      draw(graph);
    }
  });

  graph.canvas.addEventListener("pointerup", (event) => {
    if (drag && drag.node && !drag.moved) {
      var nextSelection: string | number | null = drag.node.id;
      if (graph.selection === drag.node.id) nextSelection = null;
      setSelection(nextSelection);
    } else if (drag && drag.background && !drag.moved && !drag.secondary) {
      setSelection(null);
    }
    drag = null;
    if (graph.canvas.hasPointerCapture(event.pointerId)) {
      graph.canvas.releasePointerCapture(event.pointerId);
    }
    var pointerCursor = "grab";
    if (graph.hovered) pointerCursor = "pointer";
    graph.canvas.style.cursor = pointerCursor;
  });

  graph.canvas.addEventListener("pointercancel", (event) => {
    drag = null;
    if (graph.canvas.hasPointerCapture(event.pointerId)) {
      graph.canvas.releasePointerCapture(event.pointerId);
    }
    graph.canvas.style.cursor = "grab";
  });

  graph.canvas.addEventListener("contextmenu", (event) => {
    event.preventDefault();
  });

  graph.canvas.addEventListener("wheel", (event) => {
    event.preventDefault();
    const previous = graph.view.scale;
    var factor = 0.86;
    if (event.deltaY < 0) factor = 1.16;
    const next = Math.max(0.08, Math.min(6, previous * factor));
    const rect = graph.canvas.getBoundingClientRect();
    const nextView = zoomViewAt(graph.view, {
      x: event.clientX - rect.left - rect.width / 2,
      y: event.clientY - rect.top - rect.height / 2,
    }, next);
    graph.view.x = nextView.x;
    graph.view.y = nextView.y;
    graph.view.scale = nextView.scale;
    draw(graph);
    updateZoomDisplay();
  }, { passive: false });

  const runLayoutButton = document.querySelector<HTMLButtonElement>("#graph-run-layout");
  if (runLayoutButton) {
    runLayoutButton.addEventListener("click", () => {
      runLayout(graph, status);
    });
  }

  const fitButton = document.querySelector<HTMLButtonElement>("#graph-fit");
  if (fitButton) {
    fitButton.addEventListener("click", () => {
      fitGraph(graph);
      updateZoomDisplay();
    });
  }

  const clearButton = document.querySelector<HTMLButtonElement>("#graph-clear-selection");
  if (clearButton) {
    clearButton.addEventListener("click", () => {
      setSelection(null);
    });
  }

  const requestedSelection = value("node");
  if (requestedSelection && graph.nodes.some((node) => {
    return node.id === requestedSelection;
  })) {
    setSelection(requestedSelection);
  } else {
    renderEdgePage(graph);
  }
  return setSelection;
}

/** Binds graph node search, highlighting matching nodes by name or DOI. */
function bindGraphSearch(graph: GraphState, setSelection: (id: string | number | null) => void): void {
  const searchInput = document.querySelector<HTMLInputElement>("#graph-node-search");
  const summary = document.querySelector<HTMLElement>("#graph-search-summary");
  const results = document.querySelector<HTMLElement>("#graph-search-results");
  if (!searchInput || !summary || !results) return;

  var debounce: number | undefined;

  /** Renders the bounded, keyboard-operable equivalent of canvas search highlighting. */
  function updateResults(): void {
    graph.searchQuery = searchInput!.value.trim().toLocaleLowerCase();
    const matches = graph.nodes.filter((node) => {
      const searchableParts = [node.label, node.id, node.doi, node.orcid, node.author].filter(Boolean);
      return searchableParts.join(" ").toLocaleLowerCase().includes(graph.searchQuery);
    });
    const visible = matches.slice(0, 100);
    if (!graph.searchQuery) {
      summary!.textContent = "Enter a node name, DOI, ORCID, or identifier to search the rendered graph.";
      renderTree(null, results!);
    } else {
      var suffix = "";
      if (matches.length > visible.length) suffix = "; showing the first 100";
      summary!.textContent = `${matches.length.toLocaleString()} matching nodes${suffix}.`;
      const items = visible.map((node) => {
        const label = node.label || node.id;
        return <li><button type="button" data-graph-search-node={node.id}>{label}<span>{humanLabel(node.type)}</span></button></li>;
      });
      renderTree(<Fragment>{items}</Fragment>, results!);
      results!.querySelectorAll<HTMLButtonElement>("[data-graph-search-node]").forEach((button) => {
        button.addEventListener("click", () => {
          setSelection(button.dataset.graphSearchNode || null);
          document.querySelector<HTMLElement>("#graph-selection")?.focus({ preventScroll: false });
        });
      });
    }
    draw(graph);
  }

  searchInput.addEventListener("input", () => {
    if (debounce !== undefined) window.clearTimeout(debounce);
    debounce = window.setTimeout(updateResults, 150);
  });
}

/** Repaints the canvas when either explicit or operating-system theme state changes. */
function bindGraphTheme(graph: GraphState): void {
  const repaint = () => {
    graph.colors = palette();
    draw(graph);
  };
  graph.themeObserver = new MutationObserver(repaint);
  graph.themeObserver.observe(document.documentElement, { attributes: true, attributeFilter: ["class", "data-theme"] });
  graph.themeMedia = globalThis.matchMedia("(prefers-color-scheme: dark)");
  graph.themeHandler = repaint;
  graph.themeMedia.addEventListener("change", repaint);
}

/** Binds graph export as PNG, downloading the canvas as a PNG image. */
function bindGraphExport(graph: GraphState, data: any): void {
  const exportButton = document.querySelector<HTMLButtonElement>("#graph-export-png");
  if (!exportButton) return;

  exportButton.addEventListener("click", () => {
    // Draw a clean version without search highlights for export
    const savedQuery = graph.searchQuery;
    graph.searchQuery = "";
    draw(graph);
    const captionHeight = Math.max(72, Math.round(72 * (window.devicePixelRatio || 1)));
    const exportCanvas = document.createElement("canvas");
    exportCanvas.width = graph.canvas.width;
    exportCanvas.height = graph.canvas.height + captionHeight;
    const exportContext = exportCanvas.getContext("2d")!;
    exportContext.fillStyle = graph.colors.surface;
    exportContext.fillRect(0, 0, exportCanvas.width, exportCanvas.height);
    exportContext.drawImage(graph.canvas, 0, 0);
    exportContext.fillStyle = graph.colors.text;
    exportContext.font = `${Math.max(12, Math.round(12 * (window.devicePixelRatio || 1)))}px sans-serif`;
    const filters = JSON.stringify(data.filters || {});
    const caption = `Research graph · run ${data.filters?.run_id || "not recorded"} · ${data.filters?.mode || "model not recorded"} · ${filters}`;
    exportContext.fillText(caption.slice(0, 240), 16, graph.canvas.height + Math.round(captionHeight * 0.58));
    const image = exportCanvas.toDataURL("image/png");
    graph.searchQuery = savedQuery;
    draw(graph);

    const downloadLink = document.createElement("a");
    downloadLink.download = "graph-export.png";
    downloadLink.href = image;
    downloadLink.click();
  });
}

/** Binds the graph expand button and fullscreen lifecycle. */
function bindGraphExpand(graph: GraphState): void {
  const expandButtonElement = document.querySelector<HTMLButtonElement>("#graph-expand");
  const expandViewportElement = document.querySelector<HTMLElement>("#graph-viewport");
  if (!expandButtonElement || !expandViewportElement) return;
  const expandButton = expandButtonElement;
  const expandViewport = expandViewportElement;
  var previouslyExpanded = false;
  var priorOverflow = "";
  /** Leaves the CSS fallback state and restores document and opener state. */
  function closeFallback(): void {
    if (!expandViewport.classList.contains("rw-graph__viewport--expanded")) return;
    expandViewport.classList.remove("rw-graph__viewport--expanded");
    document.body.style.overflow = priorOverflow;
    updateLabel();
  }
  /** Updates the expand button label and refits the graph after a size change. */
  function updateLabel(): void {
    const expanded = document.fullscreenElement === expandViewport || expandViewport.classList.contains("rw-graph__viewport--expanded");
    var label = "Expand graph";
    if (expanded) label = "Restore graph";
    expandButton.textContent = label;
    if (previouslyExpanded && !expanded) expandButton.focus();
    previouslyExpanded = expanded;
    requestAnimationFrame(() => {
      if (graph.resizeObserver) {
        const rect = graph.canvas.getBoundingClientRect();
        if (rect.width && rect.height) {
          fitGraph(graph);
        }
      }
    });
  }
  expandButton.addEventListener("click", async () => {
    try {
      if (document.fullscreenElement === expandViewport && document.exitFullscreen) {
        await document.exitFullscreen();
      } else if (expandViewport.requestFullscreen) {
        await expandViewport.requestFullscreen();
      } else {
        if (expandViewport.classList.contains("rw-graph__viewport--expanded")) {
          closeFallback();
        } else {
          priorOverflow = document.body.style.overflow;
          document.body.style.overflow = "hidden";
          expandViewport.classList.add("rw-graph__viewport--expanded");
          updateLabel();
        }
      }
    } catch (_) {
      if (expandViewport.classList.contains("rw-graph__viewport--expanded")) closeFallback();
      else {
        priorOverflow = document.body.style.overflow;
        document.body.style.overflow = "hidden";
        expandViewport.classList.add("rw-graph__viewport--expanded");
        updateLabel();
      }
    }
  });
  const escapeHandler = (event: KeyboardEvent) => {
    if (event.key === "Escape" && expandViewport.classList.contains("rw-graph__viewport--expanded")) {
      event.preventDefault();
      closeFallback();
    }
  };
  document.addEventListener("keydown", escapeHandler);
  graph.fullscreenHandler = updateLabel;
  document.addEventListener("fullscreenchange", updateLabel);
  graph.expandCleanup = () => {
    document.removeEventListener("keydown", escapeHandler);
    if (expandViewport.classList.contains("rw-graph__viewport--expanded")) {
      expandViewport.classList.remove("rw-graph__viewport--expanded");
      document.body.style.overflow = priorOverflow;
    }
  };
}

/** Renders the selected-node inspection panel. */
function SelectionMarkup(props: { node: GraphNode | undefined; neighbours: number }): JSX.Element {
  if (!props.node) {
    return <p>Select a node to inspect its direct relationships.</p>;
  }
  var typeLabel = "Reference mention";
  if (props.node.type === "article") {
    typeLabel = "Article revision";
  } else if (props.node.type === "author") {
    typeLabel = "Author occurrence";
  } else if (props.node.type === "referenced_author") {
    typeLabel = "Referenced-author string";
  }
  var identifier = "No DOI or ORCID recorded";
  if (props.node.doi) {
    identifier = props.node.doi;
  } else if (props.node.orcid) {
    identifier = props.node.orcid;
  }
  const href = graphLink(props.node);
  const clusterLabel = (props.node.cluster || 0) + 1;
  var recordMarkup: JSX.Element = <span className="ui faded text">No separate domain record exists for this raw referenced-author string.</span>;
  if (href) recordMarkup = <a href={href}>Open full record</a>;
  const summaryLine = (
    <p>
      {identifier}
      {"\u00B7"}
      cluster {clusterLabel}
      {" \u00B7 "}
      {props.node.degree} visible relationships
      {"\u00B7"}
      {props.neighbours} direct neighbours
    </p>
  );
  return (
    <Fragment>
      <h3>{typeLabel}</h3>
      <p><strong>{props.node.label || props.node.id}</strong></p>
      {summaryLine}
      {recordMarkup}
    </Fragment>
  );
}

/** Renders a linked or plain label for a graph node. */
function NodeMarkup(props: { node: GraphNode }): JSX.Element {
  const href = graphLink(props.node);
  const label = props.node.label || props.node.id;
  if (href) return <a href={href}>{label}</a>;
  return <Fragment>{label}</Fragment>;
}

/** Renders the paginated relationship table for the current selection or full graph. */
function renderEdgePage(graph: GraphState): void {
  const target = document.querySelector<HTMLElement>("#graph-edge-rows");
  if (!target) return;

  const pageSize = 20;
  var visibleEdges = graph.edges;
  if (graph.selection) {
    visibleEdges = graph.edges.filter((edge) => {
      return endpointID(edge.source) === graph.selection || endpointID(edge.target) === graph.selection;
    });
  }
  const pages = Math.max(1, Math.ceil(visibleEdges.length / pageSize));
  if (graph.edgePage > pages) graph.edgePage = pages;
  const rows = visibleEdges.slice((graph.edgePage - 1) * pageSize, graph.edgePage * pageSize);

  var rowsHtml: JSX.Element[] = [<tr><td colspan={4} className="empty">No relationships.</td></tr>];
  if (rows.length) {
    rowsHtml = rows.map((edge) => {
      return (
        <tr>
          <td>{relationshipLabel(edge)}</td>
          <td><NodeMarkup node={edge.source as GraphNode} /></td>
          <td><NodeMarkup node={edge.target as GraphNode} /></td>
          <td>{edgeDetails(edge)}</td>
        </tr>
      );
    });
  }

  var itemLabel = "relationships";
  if (graph.selection) itemLabel = "relationships in this neighbourhood";
  const paginationOptions = {
    itemLabel: itemLabel,
    pageAttribute: "data-graph-page",
    pageClass: " graph-page",
  };
  const paginationResult = {
    page: graph.edgePage,
    per_page: pageSize,
    total_rows: visibleEdges.length,
    total_pages: pages,
  };
  const paginationMarkup = <Pagination result={paginationResult} options={paginationOptions} />;
  const edgeTableMarkup = (
    <Fragment>
      <div className="table-wrap" aria-label="Relationship table">
        <table className="ui table">
          <thead>
            <tr>
              <th scope="col">Relationship</th>
              <th scope="col">From</th>
              <th scope="col">To</th>
              <th scope="col">Details</th>
            </tr>
          </thead>
          <tbody>{rowsHtml}</tbody>
        </table>
      </div>
      {paginationMarkup}
    </Fragment>
  );
  renderTree(edgeTableMarkup, target);

  const pageButtons = target.querySelectorAll<HTMLButtonElement>("[data-graph-page]");
  pageButtons.forEach((button) => {
    button.addEventListener("click", () => {
      graph.edgePage = Number(button.dataset.graphPage) || 1;
      renderEdgePage(graph);
    });
  });
}

/** Returns relationship-specific details for a graph edge row. */
function edgeDetails(edge: GraphEdge): string {
  if (edge.affiliation || edge.author_order) {
    var parts: string[] = [];
    if (edge.author_order) {
      parts.push(`Author ${edge.author_order}`);
    } else {
      parts.push("—");
    }
    if (edge.affiliation) {
      parts.push(edge.affiliation);
    }
    return parts.join(" \u00B7 ");
  }
  if (edge.shared_reference_count) {
    var plural = "s";
    if (edge.shared_reference_count === 1) plural = "";
    return `${edge.shared_reference_count} shared cited DOI${plural}`;
  }
  if (edge.type === "reference_author") {
    return "Raw author text captured on the reference mention";
  }
  if (edge.type === "coauthor") {
    return "Observed on the same article revision";
  }
  return "—";
}
