// D3 force layout and canvas rendering for the bounded relationship explorer.
import { forceCenter, forceCollide, forceLink, forceManyBody, forceSimulation } from '../vendor/d3-force.js';
import { esc, graphFilters, link, list, value } from '../state.js';
import { pagination } from './pagination.js';

let activeGraph;

/** Returns an escaped graph-filter input with its current URL value. */
export function graphField(name, label, type) {
  if (!type) {
    type = 'text';
  }
  return '<label>' + esc(label)
    + '<input name="' + esc(name) + '" type="' + esc(type) + '" value="' + esc(value(name)) + '">'
    + '</label>';
}

/** Returns the current graph-filter values keyed by query parameter. */
export function graphQuery() {
  var result = {};
  graphFilters.forEach(function(name) {
    result[name] = value(name);
  });
  return result;
}

/** Returns a context-preserving detail link for a graph node when one exists. */
export function graphLink(node) {
  if (node.type === 'article') {
    return link({ view: 'article', article_id: node.revision_id });
  }
  if (node.type === 'author') {
    return link({ view: 'author', author_id: node.author_id });
  }
  if (node.type === 'reference') {
    return link({ view: 'reference', reference_id: node.reference_id });
  }
  return '';
}

/** Returns an edge endpoint identifier from either an identifier or resolved node object. */
function endpointID(endpoint) {
  if (endpoint && typeof endpoint === 'object') {
    return endpoint.id;
  }
  return endpoint;
}

/** Finds deterministic connected components and maps graph nodes to cluster identifiers. */
export function graphClusters(sourceNodes, sourceEdges) {
  const adjacency = new Map(sourceNodes.map(function(node) { return [node.id, new Set()]; }));
  sourceEdges.forEach(function(edge) {
    const source = endpointID(edge.source);
    const target = endpointID(edge.target);
    if (adjacency.has(source) && adjacency.has(target)) {
      adjacency.get(source).add(target);
      adjacency.get(target).add(source);
    }
  });

  const byID = new Map();
  const components = [];
  const ordered = Array.from(adjacency.keys()).sort();
  ordered.forEach(function(start) {
    if (byID.has(start)) {
      return;
    }
    const cluster = components.length;
    const pending = [start];
    var size = 0;
    byID.set(start, cluster);
    while (pending.length) {
      const current = pending.pop();
      size += 1;
      adjacency.get(current).forEach(function(neighbour) {
        if (!byID.has(neighbour)) {
          byID.set(neighbour, cluster);
          pending.push(neighbour);
        }
      });
    }
    components.push({ id: cluster, size: size });
  });
  return { byID: byID, components: components };
}

/** Returns bounded graph controls, legend, canvas, selection, and relationship-table markup. */
export function graphResult(data) {
  const nodes = list(data, ['nodes']);
  const edges = list(data, ['edges']);
  const counts = data.counts || {};
  const limits = data.limits || {};

  if (!nodes.length) {
    return '<div class="rw-graph-empty"><h4>No relationships match these filters</h4>'
      + '<p>Broaden the filters or choose another graph model. The selected run context has been preserved.</p></div>';
  }

  const nodeTypes = counts.node_types || {};
  const edgeTypes = counts.edge_types || {};

  var warning = '';
  if (data.truncated) {
    warning = '<p class="rw-graph__truncation ui warning message" role="status">Graph results truncated. '
      + esc(String(counts.article_matches ?? nodes.length))
      + ' articles matched; '
      + esc(String(counts.article_rendered ?? 0))
      + ' articles, '
      + esc(String(counts.nodes_rendered ?? nodes.length))
      + ' nodes, and '
      + esc(String(counts.edges_rendered ?? edges.length))
      + ' edges are rendered. Refine the article filters to inspect relationships outside this bounded result.</p>';
  }

  const entityDefinitions = [
    ['article', 'Article revision', 'article'],
    ['author', 'Author occurrence', 'author'],
    ['reference', 'Reference mention', 'reference'],
    ['referenced_author', 'Referenced-author string', 'referenced-author']
  ];
  const relationshipDefinitions = [
    ['authorship', 'Authorship', ''],
    ['reference', 'Reference mention', ''],
    ['reference_author', 'Referenced author', 'derived'],
    ['citation', 'Internal citation', 'directed'],
    ['coauthor', 'Co-author', 'derived'],
    ['shared_reference', 'Shared reference', 'derived']
  ];
  const entityLegend = entityDefinitions.filter(function(definition) {
    return Number(nodeTypes[definition[0]] || 0) > 0;
  }).map(function(definition) {
    return '<span><i class="rw-graph__legend-mark rw-graph__legend-mark--' + definition[2] + '"></i>' + esc(definition[1]) + '</span>';
  }).join('');
  const relationshipLegend = relationshipDefinitions.filter(function(definition) {
    return Number(edgeTypes[definition[0]] || 0) > 0;
  }).map(function(definition) {
    const modifier = definition[2] ? ' rw-graph__legend-line--' + definition[2] : '';
    return '<span><i class="rw-graph__legend-line' + modifier + '"></i>' + esc(definition[1]) + '</span>';
  }).join('');

  return warning
    + '<div class="rw-graph__viewport" id="graph-viewport"><div class="rw-graph__toolbar">'
    + '<div class="rw-graph__search"><input type="text" id="graph-node-search" placeholder="Search nodes\u2026" aria-label="Search nodes by name or DOI"></div>'
    + '<button type="button" id="graph-fit" class="ui button">Fit graph</button>'
    + '<button type="button" id="graph-run-layout" class="ui basic button">Re-run layout</button>'
    + '<button type="button" id="graph-clear-selection" class="ui basic button" disabled>Show full graph</button>'
    + '<span class="rw-graph__zoom" id="graph-zoom-indicator" role="status">100%</span>'
    + '<span id="graph-layout-status" role="status" aria-live="polite">Preparing physics layout</span>'
    + '<button type="button" id="graph-expand" class="ui button">Expand graph</button>'
    + '<button type="button" id="graph-export-png" class="ui button" title="Download graph as PNG">Export PNG</button>'
    + '</div>'
    + '<p class="rw-graph__help">Shape identifies the entity type, color identifies a connected cluster, and size reflects visible relationship count. '
    + (nodes.length > 1500 ? 'Large networks open as a connected-cluster overview where bubble size reflects entity count; zoom in to reveal individual entities. ' : '')
    + 'Select a node to isolate its immediate neighbourhood; select it again or click the background to clear it. '
    + 'Use the mouse wheel to zoom around the pointer, or drag with the secondary mouse button to pan without selecting nodes.</p>'
    + '<div class="rw-graph__wrap"><div class="rw-graph__legend" aria-label="Visible graph encodings">'
    + '<div class="rw-graph__legend-group"><strong>Entities</strong>' + entityLegend + '</div>'
    + '<div class="rw-graph__legend-group"><strong>Relationships</strong>' + relationshipLegend + '</div></div>'
    + '<canvas class="rw-graph__canvas"></canvas></div></div>'
    + '<section class="rw-graph__selection" id="graph-selection" aria-live="polite"><p>Select a node to inspect its direct relationships.</p></section>'
    + '<section class="rw-graph__edges"><h3>Relationship table</h3>'
    + '<p>The exact, paginated relationship records behind the graph.</p>'
    + '<div id="graph-edge-rows"></div></section>';
}

/** Calculates a node radius from entity type and visible degree. */
function nodeSize(node, degree, maxDegree) {
  if (node.type === 'reference' || node.type === 'referenced_author') {
    return 7;
  }
  var min;
  var max;
  if (node.type === 'article') {
    min = 9;
    max = 22;
  } else {
    min = 8;
    max = 18;
  }
  return min + (max - min) * Math.sqrt(degree / Math.max(maxDegree, 1));
}

/** Returns a deterministic unsigned hash for stable graph placement. */
function hash(value) {
  var result = 2166136261;
  for (const character of String(value)) {
    result = Math.imul(result ^ character.charCodeAt(0), 16777619);
  }
  return result >>> 0;
}

/** Reads graph colors from active CSS custom properties with safe fallbacks. */
function palette() {
  const css = getComputedStyle(document.documentElement);
  /** Returns the associated state. */
  function get(name, fallback) {
    return css.getPropertyValue(name).trim() || fallback;
  }
  return {
    article: get('--accent', '#0b5e8e'),
    author: get('--warning', '#8f5f00'),
    reference: get('--success', '#2f6f52'),
    edge: get('--border-strong', '#adb9c2'),
    muted: get('--border', '#d2d9de'),
    text: get('--text', '#18232c'),
    surface: get('--surface', '#ffffff'),
    focus: get('--focus', '#d36d00'),
    clusters: [
      get('--graph-cluster-1', '#236b8e'), get('--graph-cluster-2', '#8b6417'),
      get('--graph-cluster-3', '#35765b'), get('--graph-cluster-4', '#6e58a3'),
      get('--graph-cluster-5', '#985468'), get('--graph-cluster-6', '#3b7c83')
    ]
  };
}

/** Draws diamond. */
function drawDiamond(context, x, y, radius) {
  context.beginPath();
  context.moveTo(x, y - radius);
  context.lineTo(x + radius, y);
  context.lineTo(x, y + radius);
  context.lineTo(x - radius, y);
  context.closePath();
}

/** Draws triangle. */
function drawTriangle(context, x, y, radius) {
  context.beginPath();
  context.moveTo(x, y - radius);
  context.lineTo(x + radius, y + radius);
  context.lineTo(x - radius, y + radius);
  context.closePath();
}

/** Returns the user-facing label for a graph edge type and its relevant metadata. */
function relationshipLabel(edge) {
  if (edge.type === 'authorship') {
    var order = '';
    if (edge.author_order) {
      order = ', author ' + edge.author_order;
    }
    return 'Authorship' + order;
  }
  if (edge.type === 'citation') {
    return 'Internal citation';
  }
  if (edge.type === 'reference') {
    return 'Reference mention';
  }
  if (edge.type === 'reference_author') {
    return 'Referenced-author string';
  }
  if (edge.type === 'coauthor') {
    return 'Co-author';
  }
  if (edge.type === 'shared_reference') {
    return 'Shared reference';
  }
  return edge.type || 'Relationship';
}

/** Destroys graph. */
export function destroyGraph() {
  if (!activeGraph) {
    return;
  }
  activeGraph.cancelled = true;
  activeGraph.simulation.stop();
  if (activeGraph.resizeObserver) {
    activeGraph.resizeObserver.disconnect();
  }
  if (activeGraph.fullscreenHandler) {
    document.removeEventListener('fullscreenchange', activeGraph.fullscreenHandler);
  }
  cancelAnimationFrame(activeGraph.frame);
  activeGraph = undefined;
}

/** Mounts graph. */
export function mountGraph(data) {
  destroyGraph();
  const canvas = document.querySelector('.rw-graph__canvas, .graph-canvas');
  if (!canvas) {
    return;
  }

  const context = canvas.getContext('2d');
  const status = document.querySelector('#graph-layout-status');
  const selectionPanel = document.querySelector('#graph-selection');
  const zoomIndicator = document.querySelector('#graph-zoom-indicator');
  const sourceNodes = list(data, ['nodes']);
  const sourceEdges = list(data, ['edges']);
  const clusters = graphClusters(sourceNodes, sourceEdges);

  const degree = new Map(sourceNodes.map(function(node) {
    return [node.id, 0];
  }));
  sourceEdges.forEach(function(edge) {
    degree.set(edge.source, (degree.get(edge.source) || 0) + 1);
    degree.set(edge.target, (degree.get(edge.target) || 0) + 1);
  });
  const maxDegree = Math.max.apply(null, Array.from(degree.values()).concat([1]));

  const nodes = sourceNodes.map(function(node) {
    const angle = hash(node.id) / 0xffffffff * Math.PI * 2;
    const cluster = clusters.byID.get(node.id) || 0;
    const clusterAngle = cluster / Math.max(clusters.components.length, 1) * Math.PI * 2;
    const clusterDistance = clusters.components.length > 1 ? 210 : 0;
    const distance = 40 + hash(node.id + ':distance') % 150;
    return {
      ...node,
      cluster: cluster,
      degree: degree.get(node.id) || 0,
      radius: nodeSize(node, degree.get(node.id) || 0, maxDegree),
      x: Math.cos(clusterAngle) * clusterDistance + Math.cos(angle) * distance,
      y: Math.sin(clusterAngle) * clusterDistance + Math.sin(angle) * distance
    };
  });

  const nodeByID = new Map(nodes.map(function(node) {
    return [node.id, node];
  }));

  const edges = sourceEdges.filter(function(edge) {
    return nodeByID.has(edge.source) && nodeByID.has(edge.target);
  }).map(function(edge, index) {
    return {
      ...edge,
      id: edge.id || edge.type + ':' + index,
      source: nodeByID.get(edge.source),
      target: nodeByID.get(edge.target)
    };
  });

  const neighbours = new Map(nodes.map(function(node) {
    return [node.id, new Set()];
  }));
  edges.forEach(function(edge) {
    neighbours.get(edge.source.id).add(edge.target.id);
    neighbours.get(edge.target.id).add(edge.source.id);
  });

  const graph = {
    cancelled: false,
    canvas: canvas,
    context: context,
    nodes: nodes,
    edges: edges,
    neighbours: neighbours,
    selection: null,
    hovered: null,
    searchQuery: '',
    colors: palette(),
    spatialIndex: null,
    clusterSummaries: clusterOverview(nodes),
    overviewOffset: { x: 0, y: 0 },
    overviewMode: false,
    layoutRunning: false,
    view: { x: 0, y: 0, scale: 1 },
    frame: 0,
    edgePage: 1
  };
  activeGraph = graph;

  const largeGraph = nodes.length > 2500;
  const simulation = forceSimulation(nodes)
    .force('link', forceLink(edges)
      .id(function(node) { return node.id; })
      .distance(function(edge) {
        if (edge.type === 'citation') {
          return 125;
        }
        if (edge.type === 'coauthor' || edge.type === 'shared_reference') {
          return 145;
        }
        if (edge.type === 'reference_author') {
          return 70;
        }
        return 92;
      })
      .strength(largeGraph ? 0.34 : 0.72))
    .force('charge', forceManyBody()
      .strength(function(node) {
        if (node.type === 'article') {
          return -260;
        }
        return -140;
      })
      .distanceMax(largeGraph ? 380 : 720))
    .force('center', forceCenter(0, 0))
    .alpha(1)
    .alphaDecay(nodes.length > 5000 ? 0.16 : nodes.length > 2500 ? 0.11 : nodes.length > 900 ? 0.065 : 0.028)
    .stop();

  if (!largeGraph) {
    simulation.force('collision', forceCollide()
      .radius(function(node) { return node.radius + 7; })
      .strength(0.92)
      .iterations(nodes.length > 900 ? 1 : 2));
  }

  graph.simulation = simulation;

  /** Resizes the backing canvas for its layout size and device pixel ratio. */
  function resize() {
    const rect = canvas.getBoundingClientRect();
    const ratio = Math.min(window.devicePixelRatio || 1, 2);
    canvas.width = Math.max(1, Math.round(rect.width * ratio));
    canvas.height = Math.max(1, Math.round(rect.height * ratio));
    draw(graph);
  }

  graph.resizeObserver = new ResizeObserver(resize);
  graph.resizeObserver.observe(canvas);
  resize();

  bindInteractions(graph, status, selectionPanel, zoomIndicator);
  bindGraphSearch(graph);
  bindGraphExport(graph, data);
  bindGraphExpand(graph);
  runLayout(graph, status);
}

/** Returns the radius-aware world-coordinate bounds of graph nodes. */
function graphBounds(nodes) {
  if (!nodes.length) {
    return { minX: -1, maxX: 1, minY: -1, maxY: 1 };
  }
  return nodes.reduce(function(bounds, node) {
    return {
      minX: Math.min(bounds.minX, node.x - node.radius),
      maxX: Math.max(bounds.maxX, node.x + node.radius),
      minY: Math.min(bounds.minY, node.y - node.radius),
      maxY: Math.max(bounds.maxY, node.y + node.radius)
    };
  }, { minX: Infinity, maxX: -Infinity, minY: Infinity, maxY: -Infinity });
}

/** Returns connected-cluster sizes ordered from largest to smallest. */
function clusterOverview(nodes) {
  const grouped = new Map();
  nodes.forEach(function(node) {
    if (!grouped.has(node.cluster)) {
      grouped.set(node.cluster, { id: node.cluster, size: 0 });
    }
    const cluster = grouped.get(node.cluster);
    cluster.size += 1;
  });
  return Array.from(grouped.values()).sort(function(a, b) { return b.size - a.size; });
}

/** Draws cluster overview. */
function drawClusterOverview(context, clusters, colors, width, height, offset, legendInset) {
  const top = 28;
  const horizontalPadding = 32;
  const bottomPadding = Math.max(28, legendInset);
  const usableWidth = Math.max(1, width - horizontalPadding * 2);
  const usableHeight = Math.max(1, height - top - bottomPadding);
  const columns = Math.max(1, Math.ceil(Math.sqrt(clusters.length * usableWidth / usableHeight)));
  const rows = Math.max(1, Math.ceil(clusters.length / columns));
  const cellWidth = usableWidth / columns;
  const cellHeight = usableHeight / rows;
  const maximumSize = Math.max.apply(null, clusters.map(function(cluster) { return cluster.size; }).concat([1]));

  return clusters.map(function(cluster, index) {
    const column = index % columns;
    const row = Math.floor(index / columns);
    const x = horizontalPadding + cellWidth * (column + 0.5) + offset.x;
    const y = top + cellHeight * (row + 0.36) + offset.y;
    const scale = 0.5 + 0.5 * Math.sqrt(cluster.size / maximumSize);
    const radius = Math.max(12, Math.min(cellWidth, cellHeight) * 0.34 * scale);
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
    context.textAlign = 'center';
    context.textBaseline = 'top';
    context.font = '600 10px ui-sans-serif, system-ui';
    context.fillText('Cluster ' + (cluster.id + 1), x, y + radius + 5);
    context.font = '9px ui-sans-serif, system-ui';
    context.fillText(cluster.size.toLocaleString() + (cluster.size === 1 ? ' entity' : ' entities'), x, y + radius + 17);
    return { id: cluster.id, x: x, y: y, radius: radius };
  });
}

/** Adjusts the graph transform to fit all node bounds in the canvas. */
function fitGraph(graph) {
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
function runLayout(graph, status) {
  graph.simulation.alpha(1).restart().stop();
  graph.spatialIndex = null;
  graph.layoutRunning = true;
  const reduced = matchMedia('(prefers-reduced-motion: reduce)').matches;
  var tickLimit;
  if (graph.nodes.length > 5000) {
    tickLimit = 20;
  } else if (graph.nodes.length > 2500) {
    tickLimit = 34;
  } else if (graph.nodes.length > 900) {
    tickLimit = 64;
  } else if (graph.nodes.length > 250) {
    tickLimit = reduced ? 90 : 140;
  } else {
    tickLimit = reduced ? 100 : 220;
  }

  const ticksPerFrame = graph.nodes.length > 5000 ? 10 : graph.nodes.length > 2500 ? 7 : graph.nodes.length > 900 ? 5 : 3;

  var ticks = 0;

  /** Advances and redraws the next batch of force-layout ticks. */
  function next() {
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
      if (reduced) {
        status.textContent = 'Physics layout placed with reduced motion';
      } else {
        status.textContent = 'Physics layout settled';
      }
    }
  }

  status.textContent = 'Running physics layout';
  graph.frame = requestAnimationFrame(next);
}

/** Draws the associated state. */
function draw(graph) {
  const canvas = graph.canvas;
  const context = graph.context;
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
    const legend = canvas.parentElement.querySelector('.rw-graph__legend');
    const legendInset = (legend ? legend.offsetHeight : 0) + 24;
    graph.overviewLayout = drawClusterOverview(context, graph.clusterSummaries, colors, width, height, graph.overviewOffset, legendInset);
    context.setTransform(ratio, 0, 0, ratio, 0, 0);
    context.globalAlpha = 1;
    return;
  }

  context.translate(width / 2 + view.x, height / 2 + view.y);
  context.scale(view.scale, view.scale);

  var focused = null;
  if (selection) {
    focused = new Set([selection, ...graph.neighbours.get(selection)]);
  }

  // Determine search-matched node IDs
  var searchMatchIds = null;
  if (searchQuery) {
    searchMatchIds = new Set();
    nodes.forEach(function(node) {
      const searchable = [node.label, node.id, node.doi, node.orcid, node.author]
        .filter(Boolean).join(' ').toLocaleLowerCase();
      if (searchable.includes(searchQuery)) {
        searchMatchIds.add(node.id);
      }
    });
  }

  for (const edge of edges) {
    var relevant;
    if (focused) {
      relevant = edge.source.id === selection || edge.target.id === selection;
    } else {
      relevant = true;
    }

    if (focused && !relevant) {
      context.globalAlpha = 0.18;
    } else {
      context.globalAlpha = 0.82;
    }

    if (relevant) {
      context.strokeStyle = colors.edge;
    } else {
      context.strokeStyle = colors.muted;
    }

    if (relevant) {
      context.lineWidth = 1.45 / view.scale;
    } else {
      context.lineWidth = 1 / view.scale;
    }

    if (edge.type === 'coauthor' || edge.type === 'shared_reference') {
      context.setLineDash([6 / view.scale, 4 / view.scale]);
    } else if (edge.type === 'reference_author') {
      context.setLineDash([2 / view.scale, 4 / view.scale]);
    } else {
      context.setLineDash([]);
    }
    context.beginPath();
    context.moveTo(edge.source.x, edge.source.y);
    context.lineTo(edge.target.x, edge.target.y);
    context.stroke();
    context.setLineDash([]);

    if (edge.type === 'citation' && relevant) {
      drawArrow(context, edge.source, edge.target, 5 / view.scale, colors.edge);
    }
  }

  for (const node of nodes) {
    var nodeRelevant;
    if (focused) {
      nodeRelevant = focused.has(node.id);
    } else {
      nodeRelevant = true;
    }

    // Search highlight: dim non-matching nodes, highlight matching nodes
    var isSearchMatch = searchMatchIds && searchMatchIds.has(node.id);
    var hasSearch = searchMatchIds !== null;

    if (hasSearch && !isSearchMatch) {
      context.globalAlpha = 0.12;
    } else if (nodeRelevant) {
      context.globalAlpha = 1;
    } else {
      context.globalAlpha = 0.22;
    }

    context.fillStyle = colors.clusters[node.cluster % colors.clusters.length];

    if (node.id === selection) {
      context.strokeStyle = colors.focus;
      context.lineWidth = 2.6 / view.scale;
    } else {
      context.strokeStyle = colors.text;
      context.lineWidth = 1.1 / view.scale;
    }

    if (node.type === 'reference') {
      drawDiamond(context, node.x, node.y, node.radius);
    } else if (node.type === 'author') {
      context.beginPath();
      context.rect(node.x - node.radius, node.y - node.radius, node.radius * 2, node.radius * 2);
    } else if (node.type === 'referenced_author') {
      drawTriangle(context, node.x, node.y, node.radius);
    } else {
      context.beginPath();
      context.arc(node.x, node.y, node.radius, 0, Math.PI * 2);
    }

    context.fill();
    context.stroke();

    // Draw search highlight ring
    if (isSearchMatch) {
      context.strokeStyle = colors.focus;
      context.lineWidth = 2.5 / view.scale;
      if (node.type === 'reference') {
        drawDiamond(context, node.x, node.y, node.radius + 4 / view.scale);
      } else if (node.type === 'author') {
        context.beginPath();
        const searchRadius = node.radius + 4 / view.scale;
        context.rect(node.x - searchRadius, node.y - searchRadius, searchRadius * 2, searchRadius * 2);
      } else if (node.type === 'referenced_author') {
        drawTriangle(context, node.x, node.y, node.radius + 4 / view.scale);
      } else {
        context.beginPath();
        context.arc(node.x, node.y, node.radius + 4 / view.scale, 0, Math.PI * 2);
      }
      context.stroke();
    }
  }

  var labelNode = null;
  if (hovered) {
    labelNode = nodes.find(function(node) { return node.id === hovered; });
  }
  if (!labelNode && selection) {
    labelNode = nodes.find(function(node) { return node.id === selection; });
  }

  if (labelNode) {
    const label = String(labelNode.label || labelNode.id);
    context.globalAlpha = 1;
    context.font = (12 / view.scale) + 'px ui-sans-serif, system-ui';
    context.textBaseline = 'middle';
    const labelX = labelNode.x + labelNode.radius + 7 / view.scale;
    const labelWidth = context.measureText(label).width;
    context.fillStyle = colors.surface;
    context.fillRect(labelX - 3 / view.scale, labelNode.y - 10 / view.scale, labelWidth + 6 / view.scale, 20 / view.scale);
    context.fillStyle = colors.text;
    context.fillText(label, labelX, labelNode.y);
  }

  context.setTransform(ratio, 0, 0, ratio, 0, 0);
  context.globalAlpha = 1;
}

/** Draws arrow. */
function drawArrow(context, source, target, radius, color) {
  const angle = Math.atan2(target.y - source.y, target.x - source.x);
  const x = target.x - Math.cos(angle) * (target.radius + 2);
  const y = target.y - Math.sin(angle) * (target.radius + 2);
  context.fillStyle = color;
  context.beginPath();
  context.moveTo(x, y);
  context.lineTo(x - Math.cos(angle - 0.5) * radius, y - Math.sin(angle - 0.5) * radius);
  context.lineTo(x - Math.cos(angle + 0.5) * radius, y - Math.sin(angle + 0.5) * radius);
  context.closePath();
  context.fill();
}

/** Converts a pointer event from canvas coordinates to graph world coordinates. */
function graphCoordinates(graph, event) {
  const rect = graph.canvas.getBoundingClientRect();
  return {
    x: (event.clientX - rect.left - rect.width / 2 - graph.view.x) / graph.view.scale,
    y: (event.clientY - rect.top - rect.height / 2 - graph.view.y) / graph.view.scale
  };
}

/** Returns a zoom transform that keeps the selected screen point stationary. */
export function zoomViewAt(view, screenPoint, nextScale) {
  const worldX = (screenPoint.x - view.x) / view.scale;
  const worldY = (screenPoint.y - view.y) / view.scale;
  return {
    x: screenPoint.x - worldX * nextScale,
    y: screenPoint.y - worldY * nextScale,
    scale: nextScale
  };
}

/** Returns the overview cluster hit by a pointer event, when any. */
function nearestOverviewCluster(graph, event) {
  const rect = graph.canvas.getBoundingClientRect();
  const x = event.clientX - rect.left;
  const y = event.clientY - rect.top;
  return (graph.overviewLayout || []).find(function(cluster) {
    return Math.hypot(cluster.x - x, cluster.y - y) <= cluster.radius + 8;
  });
}

/** Focuses cluster. */
function focusCluster(graph, clusterID) {
  const nodes = graph.nodes.filter(function(node) { return node.cluster === clusterID; });
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

/** Builds spatial index. */
function buildSpatialIndex(nodes) {
  const cellSize = 64;
  const cells = new Map();
  nodes.forEach(function(node) {
    const key = Math.floor(node.x / cellSize) + ':' + Math.floor(node.y / cellSize);
    if (!cells.has(key)) {
      cells.set(key, []);
    }
    cells.get(key).push(node);
  });
  return { cellSize: cellSize, cells: cells };
}

/** Returns nodes in the spatial-index cell surrounding a graph point. */
function nearbyNodes(index, point) {
  const x = Math.floor(point.x / index.cellSize);
  const y = Math.floor(point.y / index.cellSize);
  const candidates = [];
  for (var dx = -1; dx <= 1; dx += 1) {
    for (var dy = -1; dy <= 1; dy += 1) {
      const bucket = index.cells.get((x + dx) + ':' + (y + dy));
      if (bucket) {
        candidates.push(...bucket);
      }
    }
  }
  return candidates;
}

/** Returns the closest selectable node within its hit radius. */
function nearestNode(graph, point) {
  var best = null;
  if (graph.overviewMode) {
    return null;
  }
  if (graph.layoutRunning && graph.nodes.length > 2500) {
    return null;
  }
  const candidates = graph.spatialIndex ? nearbyNodes(graph.spatialIndex, point) : graph.nodes;
  for (const node of candidates) {
    const distance = Math.hypot(node.x - point.x, node.y - point.y);
    if (distance <= node.radius + 6) {
      if (!best || distance < best.distance) {
        best = { node: node, distance: distance };
      }
    }
  }
  if (best) {
    return best.node;
  }
  return null;
}

/** Binds DOM behavior for interactions. */
function bindInteractions(graph, status, selectionPanel, zoomIndicator) {
  var drag = null;
  const dragThreshold = 4;

  /** Sets selection. */
  function setSelection(id) {
    graph.selection = id;
    graph.edgePage = 1;
    var clearButton = document.querySelector('#graph-clear-selection');
    if (clearButton) {
      clearButton.disabled = !id;
    }
    if (id) {
      var node = graph.nodes.find(function(n) { return n.id === id; });
      var neighbours = graph.neighbours.get(id).size;
      selectionPanel.innerHTML = selectionMarkup(node, neighbours);
    } else {
      selectionPanel.innerHTML = '<p>Select a node to inspect its direct relationships.</p>';
    }
    const url = new URL(location.href);
    if (id) {
      url.searchParams.set('node', id);
    } else {
      url.searchParams.delete('node');
    }
    history.replaceState({}, '', url.toString());
    draw(graph);
    renderEdgePage(graph);
  }

  /** Updates zoom display. */
  function updateZoomDisplay() {
    if (zoomIndicator) {
      zoomIndicator.textContent = Math.round(graph.view.scale * 100) + '%';
    }
  }

  graph.canvas.addEventListener('pointerdown', function(event) {
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
        moved: false
      };
      graph.canvas.style.cursor = 'grabbing';
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
      drag = { node: node, x: event.clientX, y: event.clientY, moved: false };
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
        moved: false
      };
    }
    graph.canvas.setPointerCapture(event.pointerId);
  });

  graph.canvas.addEventListener('pointermove', function(event) {
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
      graph.canvas.style.cursor = nearestOverviewCluster(graph, event) ? 'pointer' : 'grab';
      return;
    }
    var node = nearestNode(graph, graphCoordinates(graph, event));
    var hovered = node ? node.id : null;
    if (hovered !== graph.hovered) {
      graph.hovered = hovered;
      if (hovered) {
        graph.canvas.style.cursor = 'pointer';
      } else {
        graph.canvas.style.cursor = 'grab';
      }
      draw(graph);
    }
  });

  graph.canvas.addEventListener('pointerup', function(event) {
    if (drag && drag.node && !drag.moved) {
      setSelection(graph.selection === drag.node.id ? null : drag.node.id);
    } else if (drag && drag.background && !drag.moved && !drag.secondary) {
      setSelection(null);
    }
    drag = null;
    if (graph.canvas.hasPointerCapture(event.pointerId)) {
      graph.canvas.releasePointerCapture(event.pointerId);
    }
    graph.canvas.style.cursor = graph.hovered ? 'pointer' : 'grab';
  });

  graph.canvas.addEventListener('pointercancel', function(event) {
    drag = null;
    if (graph.canvas.hasPointerCapture(event.pointerId)) {
      graph.canvas.releasePointerCapture(event.pointerId);
    }
    graph.canvas.style.cursor = 'grab';
  });

  graph.canvas.addEventListener('contextmenu', function(event) {
    event.preventDefault();
  });

  graph.canvas.addEventListener('wheel', function(event) {
    event.preventDefault();
    const previous = graph.view.scale;
    var factor;
    if (event.deltaY < 0) {
      factor = 1.16;
    } else {
      factor = 0.86;
    }
    const next = Math.max(0.08, Math.min(6, previous * factor));
    const rect = graph.canvas.getBoundingClientRect();
    const nextView = zoomViewAt(graph.view, {
      x: event.clientX - rect.left - rect.width / 2,
      y: event.clientY - rect.top - rect.height / 2
    }, next);
    graph.view.x = nextView.x;
    graph.view.y = nextView.y;
    graph.view.scale = nextView.scale;
    draw(graph);
    updateZoomDisplay();
  }, { passive: false });

  var runLayoutButton = document.querySelector('#graph-run-layout');
  if (runLayoutButton) {
    runLayoutButton.addEventListener('click', function() {
      runLayout(graph, status);
    });
  }

  var fitButton = document.querySelector('#graph-fit');
  if (fitButton) {
    fitButton.addEventListener('click', function() {
      fitGraph(graph);
      updateZoomDisplay();
    });
  }

  var clearButton = document.querySelector('#graph-clear-selection');
  if (clearButton) {
    clearButton.addEventListener('click', function() {
      setSelection(null);
    });
  }

  const requestedSelection = value('node');
  if (requestedSelection && graph.nodes.some(function(node) { return node.id === requestedSelection; })) {
    setSelection(requestedSelection);
  } else {
    renderEdgePage(graph);
  }
}

/**
 * Bind graph node search — highlights matching nodes by name/DOI.
 */
function bindGraphSearch(graph) {
  var searchInput = document.querySelector('#graph-node-search');
  if (!searchInput) return;

  searchInput.addEventListener('input', function() {
    graph.searchQuery = searchInput.value.trim().toLocaleLowerCase();
    draw(graph);
  });
}

/**
 * Bind graph export as PNG — downloads the canvas as a PNG image.
 */
function bindGraphExport(graph, data) {
  var exportBtn = document.querySelector('#graph-export-png');
  if (!exportBtn) return;

  exportBtn.addEventListener('click', function() {
    // Draw a clean version without search highlights for export
    var savedQuery = graph.searchQuery;
    graph.searchQuery = '';
    draw(graph);
    const image = graph.canvas.toDataURL('image/png');
    graph.searchQuery = savedQuery;
    draw(graph);

    var link = document.createElement('a');
    link.download = 'graph-export.png';
    link.href = image;
    link.click();
  });
}

/** Binds DOM behavior for graph expand. */
function bindGraphExpand(graph) {
  const button = document.querySelector('#graph-expand');
  const viewport = document.querySelector('#graph-viewport');
  if (!button || !viewport) {
    return;
  }
  /** Updates label. */
  function updateLabel() {
    const expanded = document.fullscreenElement === viewport || viewport.classList.contains('rw-graph__viewport--expanded');
    button.textContent = expanded ? 'Restore graph' : 'Expand graph';
    requestAnimationFrame(function() {
      if (graph.resizeObserver) {
        const rect = graph.canvas.getBoundingClientRect();
        if (rect.width && rect.height) {
          fitGraph(graph);
        }
      }
    });
  }
  button.addEventListener('click', async function() {
    try {
      if (document.fullscreenElement === viewport && document.exitFullscreen) {
        await document.exitFullscreen();
      } else if (viewport.requestFullscreen) {
        await viewport.requestFullscreen();
      } else {
        viewport.classList.toggle('rw-graph__viewport--expanded');
        updateLabel();
      }
    } catch (_) {
      viewport.classList.toggle('rw-graph__viewport--expanded');
      updateLabel();
    }
  });
  graph.fullscreenHandler = updateLabel;
  document.addEventListener('fullscreenchange', updateLabel);
}

/** Selects ion markup. */
function selectionMarkup(node, neighbours) {
  if (!node) {
    return '<p>Select a node to inspect its direct relationships.</p>';
  }
  var typeLabel;
  if (node.type === 'article') {
    typeLabel = 'Article revision';
  } else if (node.type === 'author') {
    typeLabel = 'Author occurrence';
  } else if (node.type === 'referenced_author') {
    typeLabel = 'Referenced-author string';
  } else {
    typeLabel = 'Reference mention';
  }
  var identifier;
  if (node.doi) {
    identifier = node.doi;
  } else if (node.orcid) {
    identifier = node.orcid;
  } else {
    identifier = 'No DOI or ORCID recorded';
  }
  const href = graphLink(node);
  var action = '<span class="ui faded text">No separate domain record exists for this raw referenced-author string.</span>';
  if (href) {
    action = '<a href="' + href + '">Open full record</a>';
  }
  return '<h3>' + esc(typeLabel) + '</h3>'
    + '<p><strong>' + esc(node.label || node.id) + '</strong></p>'
    + '<p>' + esc(identifier) + ' \u00B7 cluster ' + esc(String((node.cluster || 0) + 1))
    + ' \u00B7 ' + esc(String(node.degree)) + ' visible relationships \u00B7 ' + esc(String(neighbours)) + ' direct neighbours</p>'
    + action;
}

/** Returns escaped linked or plain label markup for a graph node. */
function nodeMarkup(node) {
  const href = graphLink(node);
  const label = esc(node.label || node.id);
  if (href) {
    return '<a href="' + href + '">' + label + '</a>';
  }
  return label;
}

/** Renders edge page. */
function renderEdgePage(graph) {
  const target = document.querySelector('#graph-edge-rows');
  if (!target) {
    return;
  }

  const pageSize = 20;
  const visibleEdges = graph.selection ? graph.edges.filter(function(edge) {
    return edge.source.id === graph.selection || edge.target.id === graph.selection;
  }) : graph.edges;
  const pages = Math.max(1, Math.ceil(visibleEdges.length / pageSize));
  if (graph.edgePage > pages) {
    graph.edgePage = pages;
  }
  const rows = visibleEdges.slice((graph.edgePage - 1) * pageSize, graph.edgePage * pageSize);

  var rowsHtml;
  if (rows.length) {
    rowsHtml = rows.map(function(edge) {
      return '<tr>'
        + '<td>' + esc(relationshipLabel(edge)) + '</td>'
        + '<td>' + nodeMarkup(edge.source) + '</td>'
        + '<td>' + nodeMarkup(edge.target) + '</td>'
        + '<td>' + esc(edgeDetails(edge)) + '</td>'
        + '</tr>';
    }).join('');
  } else {
    rowsHtml = '<tr><td colspan="4" class="empty">No relationships.</td></tr>';
  }

  target.innerHTML = '<div class="table-wrap" aria-label="Relationship table">'
    + '<table class="ui table"><thead><tr>'
    + '<th scope="col">Relationship</th>'
    + '<th scope="col">From</th>'
    + '<th scope="col">To</th>'
    + '<th scope="col">Details</th>'
    + '</tr></thead><tbody>'
    + rowsHtml
    + '</tbody></table></div>'
    + pagination({ page: graph.edgePage, per_page: pageSize, total_rows: visibleEdges.length, total_pages: pages }, {
      itemLabel: graph.selection ? 'relationships in this neighbourhood' : 'relationships',
      pageAttribute: 'data-graph-page', pageClass: ' graph-page'
    });

  target.querySelectorAll('[data-graph-page]').forEach(function(button) {
    button.addEventListener('click', function() {
      graph.edgePage = Number(button.dataset.graphPage) || 1;
      renderEdgePage(graph);
    });
  });
}

/** Returns relationship-specific details for a graph edge row. */
function edgeDetails(edge) {
  if (edge.affiliation || edge.author_order) {
    var parts = [];
    if (edge.author_order) {
      parts.push('Author ' + edge.author_order);
    } else {
      parts.push('—');
    }
    if (edge.affiliation) {
      parts.push(edge.affiliation);
    }
    return parts.join(' \u00B7 ');
  }
  if (edge.shared_reference_count) {
    return edge.shared_reference_count + ' shared cited DOI' + (edge.shared_reference_count === 1 ? '' : 's');
  }
  if (edge.type === 'reference_author') {
    return 'Raw author text captured on the reference mention';
  }
  if (edge.type === 'coauthor') {
    return 'Observed on the same article revision';
  }
  return '—';
}
