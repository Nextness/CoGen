// Unit tests for components/graph.tsx — graph field, query, link, result, and pure internal functions.
import { describe, it } from "node:test";
import assert from "node:assert/strict";

import "../../../../support/node/setup.ts";
import { GraphField, graphQuery, graphLink, GraphResult, graphClusters, zoomViewAt, destroyGraph, mountGraph } from "../../../../../frontend/src/components/graph.tsx";
import { renderToString } from "../../../../support/node/helpers/jsx-render.ts";

const graphField = (name: string, label: string, type?: JSX.RWInputType): string => {
  return renderToString(GraphField({
    name,
    label,
    type,
  }));
};
const graphResult = (data: any): string => {
  return renderToString(GraphResult({ data }));
};

describe("graph.tsx — graphField", () => {

  it("renders a labeled input field", () => {
    const result = graphField("q", "Search");
    assert.ok(result.includes("label"));
    assert.ok(result.includes("Search"));
    assert.ok(result.includes(`name="q"`));
    assert.ok(result.includes(`type="text"`));
  });

  it("uses provided type", () => {
    const result = graphField("year_min", "Year from", "number");
    assert.ok(result.includes(`type="number"`));
  });

  it("escapes HTML in label", () => {
    const result = graphField("q", "<script>");
    assert.ok(!result.includes("<script>"));
    assert.ok(result.includes("&lt;script&gt;"));
  });

});

describe("graph.tsx — graphQuery", () => {

  it("returns an object with all graph filter values", () => {
    const result = graphQuery();
    assert.ok(typeof result === "object");
    assert.ok("mode" in result);
    assert.ok("q" in result);
    assert.ok("author" in result);
    assert.ok("article_limit" in result);
  });

});

describe("graph.tsx — graphLink", () => {

  it("links to article view for article nodes", () => {
    const result = graphLink({
      type: "article",
      revision_id: "r1",
    } as unknown as import("../../../../../frontend/src/components/graph.tsx").GraphNode);
    assert.equal(result.href, "/article");
    assert.equal(result.state.article_id, "r1");
  });

  it("links to author view for author nodes", () => {
    const result = graphLink({
      type: "author",
      author_id: "a1",
    } as unknown as import("../../../../../frontend/src/components/graph.tsx").GraphNode);
    assert.equal(result.href, "/author");
    assert.equal(result.state.author_id, "a1");
  });

  it("links to reference view for other nodes", () => {
    const result = graphLink({
      type: "reference",
      reference_id: "ref1",
    } as unknown as import("../../../../../frontend/src/components/graph.tsx").GraphNode);
    assert.equal(result.href, "/reference");
    assert.equal(result.state.reference_id, "ref1");
  });

  it("does not fabricate a detail route for a raw referenced-author string", () => {
    assert.deepEqual(graphLink({
      type: "referenced_author",
      id: "raw:1",
    } as unknown as import("../../../../../frontend/src/components/graph.tsx").GraphNode), {
      href: "",
      state: {},
    });
  });

});

describe("graph.tsx — graphResult", () => {

  it("renders graph result with nodes and edges", () => {
    const data = {
      nodes: [{
        id: "n1",
        label: "Node 1",
      }],
      edges: [{
        id: "e1",
        source: "n1",
        target: "n2",
      }],
      counts: {
        article_matches: 1,
        article_rendered: 1,
        nodes_rendered: 1,
        edges_rendered: 1,
      },
    };
    const result = graphResult(data);
    assert.ok(result.includes("rw-graph__legend"));
    assert.ok(result.includes("rw-graph__toolbar"));
    assert.ok(result.includes("rw-graph__canvas"));
    assert.ok(result.includes("rw-graph__selection"));
    assert.ok(result.includes("rw-graph__edges"));
  });

  it("includes truncation warning when data is truncated", () => {
    const data = {
      nodes: [{ id: "n1" }],
      edges: [],
      truncated: true,
      counts: {
        article_matches: 100,
        article_rendered: 50,
        nodes_rendered: 50,
        edges_rendered: 30,
      },
    };
    const result = graphResult(data);
    assert.ok(result.includes("rw-graph__truncation"));
    assert.ok(result.includes("Graph results truncated"));
  });

  it("handles missing counts", () => {
    const data = {
      nodes: [],
      edges: [],
    };
    const result = graphResult(data);
    assert.ok(result.includes("rw-graph-empty"));
    assert.ok(result.includes("No relationships match"));
  });

  it("includes node search input in toolbar", () => {
    const data = {
      nodes: [{
        id: "article:1",
        type: "article",
      }],
      edges: [],
      counts: {
        article_matches: 1,
        article_rendered: 1,
        nodes_rendered: 1,
        edges_rendered: 0,
      },
    };
    const result = graphResult(data);
    assert.ok(result.includes(`id="graph-node-search"`));
    assert.ok(result.includes(`placeholder="Search nodes\u2026"`));
  });

  it("includes zoom indicator in toolbar", () => {
    const data = {
      nodes: [{
        id: "article:1",
        type: "article",
      }],
      edges: [],
      counts: {
        article_matches: 1,
        article_rendered: 1,
        nodes_rendered: 1,
        edges_rendered: 0,
      },
    };
    const result = graphResult(data);
    assert.ok(result.includes(`id="graph-zoom-indicator"`));
    assert.ok(result.includes("100%"));
  });

  it("includes export PNG button in toolbar", () => {
    const data = {
      nodes: [{
        id: "article:1",
        type: "article",
      }],
      edges: [],
      counts: {
        article_matches: 1,
        article_rendered: 1,
        nodes_rendered: 1,
        edges_rendered: 0,
      },
    };
    const result = graphResult(data);
    assert.ok(result.includes(`id="graph-export-png"`));
    assert.ok(result.includes("Export PNG"));
  });

});

describe("graph.tsx — graphClusters", () => {
  it("assigns deterministic connected components", () => {
    const result = graphClusters(
      [{
        id: "a",
      }, {
        id: "b",
      }, {
        id: "c",
      }] as unknown as import("../../../../../frontend/src/components/graph.tsx").GraphNode[],
      [{
        source: "a",
        target: "b",
      }] as unknown as import("../../../../../frontend/src/components/graph.tsx").GraphEdge[],
    );
    assert.equal(result.byID.get("a"), result.byID.get("b"));
    assert.notEqual(result.byID.get("a"), result.byID.get("c"));
    assert.deepEqual(result.components.map((item) => {
      return item.size;
    }), [2, 1]);
  });
});

describe("graph.tsx — zoomViewAt", () => {
  it("keeps the world position beneath the pointer fixed while zooming", () => {
    const view = {
      x: 15,
      y: -10,
      scale: 2,
    };
    const pointer = {
      x: 115,
      y: 50,
    };
    const worldBefore = {
      x: (pointer.x - view.x) / view.scale,
      y: (pointer.y - view.y) / view.scale,
    };
    const next = zoomViewAt(view, pointer, 3.5);
    assert.equal((pointer.x - next.x) / next.scale, worldBefore.x);
    assert.equal((pointer.y - next.y) / next.scale, worldBefore.y);
  });
});

describe("graph.tsx — destroyGraph", () => {

  it("does nothing when no active graph", () => {
    destroyGraph();
    assert.ok(true);
  });

});

describe("graph.tsx — mountGraph", () => {

  it("does nothing when canvas is missing", () => {
    const canvas = document.querySelector(".rw-graph__canvas");
    if (canvas) {
      canvas.remove();
    }
    mountGraph({
      nodes: [],
      edges: [],
      filters: {},
      truncated: false,
      limits: {},
      counts: {
        article_matches: 0,
        article_rendered: 0,
        nodes_rendered: 0,
        edges_rendered: 0,
        node_types: {},
        edge_types: {},
      },
      truncation_reason: "",
    });
    assert.ok(true);

    const newCanvas = document.createElement("canvas");
    newCanvas.className = "rw-graph__canvas";
    newCanvas.width = 800;
    newCanvas.height = 600;
    document.body.appendChild(newCanvas);
  });

});
