// graph.go provides the bounded citation-graph endpoint that returns
// a D3-force-compatible JSON graph of works and their reference
// edges, scoped to the selected pipeline run.
package server

import (
	"analysis/backend/server/graphview"
	viewrequest "analysis/backend/server/request"
	"context"

	"net/http"

	"strconv"

	"analysis/backend/database/work"
)

const (
	defaultArticleLimit = 2000
	maxArticleLimit     = 2000
	maxRelatedNodes     = 10000
	maxGraphEdges       = 20000
)

// graph validates graph filters and returns a bounded relationship graph for one run.
func (s *Server) graph(w http.ResponseWriter, r *http.Request) {
	allowed := []string{"run_id", "mode", "q", "author", "orcid", "reference", "source", "year_min", "year_max", "citation_min", "citation_max", "reference_min", "reference_max", "article_limit"}
	if err := viewrequest.KnownQuery(r, allowed...); err != nil {
		s.respond(w, r, nil, err)
		return
	}
	runID, err := viewrequest.PositiveID(r.URL.Query().Get("run_id"))
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	mode := r.URL.Query().Get("mode")
	if mode == "" {
		mode = "article_author"
	}
	if mode != "research_network" && mode != "article_author" && mode != "citation" && mode != "article_reference" {
		s.respond(w, r, nil, viewrequest.BadRequest("mode must be research_network, article_author, citation, or article_reference"))
		return
	}
	limit := defaultArticleLimit
	if raw := r.URL.Query().Get("article_limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > maxArticleLimit {
			s.respond(w, r, nil, viewrequest.BadRequest("article_limit must be between 1 and 2000"))
			return
		}
		limit = parsed
	}
	ctx, cancel := queryContext(r)
	defer cancel()
	articles, articleMatches, err := s.graphArticles(ctx, r, runID, limit)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	selectedTruncated := articleMatches > len(articles)
	nodes, edges, relatedTruncated, err := graphview.Build(ctx, s.workStore, mode, articles, maxRelatedNodes, maxGraphEdges)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	truncated := selectedTruncated || relatedTruncated
	reason := ""
	if selectedTruncated {
		reason = "article_limit"
	} else if relatedTruncated {
		reason = "node_or_edge_limit"
	}
	nodeTypes := make(map[string]int)
	for _, node := range nodes {
		if nodeType, ok := node["type"].(string); ok {
			nodeTypes[nodeType]++
		}
	}
	edgeTypes := make(map[string]int)
	for _, edge := range edges {
		if edgeType, ok := edge["type"].(string); ok {
			edgeTypes[edgeType]++
		}
	}
	s.respond(w, r, map[string]any{
		"nodes": nodes, "edges": edges, "filters": graphFilters(r, runID, mode, limit),
		"truncated": truncated, "limits": map[string]int{"article_nodes": maxArticleLimit, "related_nodes": maxRelatedNodes, "edges": maxGraphEdges},
		"counts": map[string]any{
			"article_matches": articleMatches, "article_rendered": len(articles), "nodes_rendered": len(nodes), "edges_rendered": len(edges),
			"node_types": nodeTypes, "edge_types": edgeTypes,
		},
		"truncation_reason": reason,
	}, nil)
}

// graphArticles selects normalized, valid article nodes matching the request filters and limit.
func (s *Server) graphArticles(ctx context.Context, r *http.Request, runID int64, limit int) ([]map[string]any, int, error) {
	filter, err := graphArticleFilter(r, runID, limit)
	if err != nil {
		return nil, 0, err
	}
	articles, matches, err := s.workStore.ListGraphArticles(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	items := make([]map[string]any, 0, len(articles))
	for _, article := range articles {
		items = append(items, graphArticleRow(article))
	}
	return items, matches, nil
}

// graphArticleFilter parses and validates the graph article request filters.
func graphArticleFilter(r *http.Request, runID int64, limit int) (work.GraphFilter, error) {
	filter := work.GraphFilter{RunID: runID, Limit: limit}
	query := r.URL.Query()
	filter.Query = query.Get("q")
	filter.Source = query.Get("source")
	filter.Author = query.Get("author")
	filter.ORCID = query.Get("orcid")
	filter.Reference = query.Get("reference")
	for _, value := range []struct {
		name  string
		value string
	}{
		{"q", filter.Query},
		{"source", filter.Source},
		{"author", filter.Author},
		{"orcid", filter.ORCID},
		{"reference", filter.Reference},
	} {
		if len(value.value) > viewrequest.SearchLimit {
			return work.GraphFilter{}, viewrequest.BadRequest(value.name + " must be at most 200 characters")
		}
	}
	for _, bound := range []struct {
		parameter string
		target    **int64
	}{
		{"year_min", &filter.YearMin},
		{"year_max", &filter.YearMax},
		{"citation_min", &filter.CitationMin},
		{"citation_max", &filter.CitationMax},
		{"reference_min", &filter.ReferenceMin},
		{"reference_max", &filter.ReferenceMax},
	} {
		raw := query.Get(bound.parameter)
		if raw == "" {
			continue
		}
		value, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return work.GraphFilter{}, viewrequest.BadRequest(bound.parameter + " must be an integer")
		}
		*bound.target = &value
	}
	return filter, nil
}

// graphFilters returns the effective non-empty graph filters for response metadata.
func graphFilters(r *http.Request, runID int64, mode string, limit int) map[string]any {
	result := map[string]any{"run_id": runID, "mode": mode, "article_limit": limit}
	for _, key := range []string{"q", "author", "orcid", "reference", "source", "year_min", "year_max", "citation_min", "citation_max", "reference_min", "reference_max"} {
		if value := r.URL.Query().Get(key); value != "" {
			result[key] = value
		}
	}
	return result
}
