// graph.go provides the bounded citation-graph endpoint that returns
// a D3-force-compatible JSON graph of works and their reference
// edges, scoped to the selected pipeline run.
package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"sort"
	"strconv"
	"strings"
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
	if err := validateKnownQuery(r, allowed...); err != nil {
		s.respond(w, r, nil, err)
		return
	}
	runID, err := positiveID(r.URL.Query().Get("run_id"))
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	mode := r.URL.Query().Get("mode")
	if mode == "" {
		mode = "article_author"
	}
	if mode != "research_network" && mode != "article_author" && mode != "citation" && mode != "article_reference" {
		s.respond(w, r, nil, badRequest("mode must be research_network, article_author, citation, or article_reference"))
		return
	}
	limit := defaultArticleLimit
	if raw := r.URL.Query().Get("article_limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > maxArticleLimit {
			s.respond(w, r, nil, badRequest("article_limit must be between 1 and 2000"))
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
	nodes, edges, relatedTruncated, err := s.graphEdges(ctx, mode, articles)
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
	clauses, args := []string{
		"wr.pipeline_run_id=?",
		currentNormalizedRevisionPredicate("wr"),
	}, []any{runID}
	query := r.URL.Query()
	if value := query.Get("q"); value != "" {
		clauses = append(clauses, "(lower(COALESCE(wr.title,'')) LIKE lower(?) OR lower(COALESCE(w.doi,'')) LIKE lower(?))")
		like := "%" + value + "%"
		args = append(args, like, like)
	}
	if value := query.Get("source"); value != "" {
		clauses = append(clauses, "wr.source=?")
		args = append(args, value)
	}
	for _, filter := range []struct{ parameter, column, operator string }{{"year_min", "wr.year", ">="}, {"year_max", "wr.year", "<="}, {"citation_min", "wr.citation_count", ">="}, {"citation_max", "wr.citation_count", "<="}, {"reference_min", "wr.reference_count", ">="}, {"reference_max", "wr.reference_count", "<="}} {
		if raw := query.Get(filter.parameter); raw != "" {
			value, err := strconv.ParseInt(raw, 10, 64)
			if err != nil {
				return nil, 0, badRequest(filter.parameter + " must be an integer")
			}
			clauses = append(clauses, filter.column+filter.operator+"?")
			args = append(args, value)
		}
	}
	if author, orcid := query.Get("author"), query.Get("orcid"); author != "" || orcid != "" {
		conditions := make([]string, 0, 2)
		authorArgs := make([]any, 0, 2)
		if author != "" {
			conditions = append(conditions, "lower(COALESCE(ao.citation_name,'')) LIKE lower(?)")
			authorArgs = append(authorArgs, "%"+author+"%")
		}
		if orcid != "" {
			conditions = append(conditions, "ao.orcid=?")
			authorArgs = append(authorArgs, orcid)
		}
		clauses = append(clauses, "EXISTS (SELECT 1 FROM authorships a JOIN author_occurrences ao ON ao.id=a.author_occurrence_id WHERE a.work_revision_id=wr.id AND "+strings.Join(conditions, " AND ")+")")
		args = append(args, authorArgs...)
	}
	if reference := query.Get("reference"); reference != "" {
		like := "%" + reference + "%"
		clauses = append(clauses, "EXISTS (SELECT 1 FROM reference_mentions rm WHERE rm.work_revision_id=wr.id AND (lower(COALESCE(rm.doi,'')) LIKE lower(?) OR lower(COALESCE(rm.title,'')) LIKE lower(?) OR lower(COALESCE(rm.author,'')) LIKE lower(?)))")
		args = append(args, like, like, like)
	}
	where := strings.Join(clauses, " AND ")
	var matches int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM work_revisions wr JOIN works w ON w.id=wr.work_id WHERE `+where, args...).Scan(&matches); err != nil {
		return nil, 0, err
	}
	args = append(args, limit)
	rows, err := s.db.QueryContext(ctx, `SELECT wr.id, wr.work_id, wr.title, wr.year, wr.source, w.doi
		FROM work_revisions wr JOIN works w ON w.id=wr.work_id WHERE `+where+` ORDER BY wr.id LIMIT ?`, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items, err := rowsAsMaps(rows)
	if err != nil {
		return nil, 0, err
	}
	return items, matches, nil
}

// graphEdges builds bounded nodes and edges for one supported relationship mode.
func (s *Server) graphEdges(ctx context.Context, mode string, articles []map[string]any) ([]map[string]any, []map[string]any, bool, error) {
	return s.graphEdgesWithinBudget(ctx, mode, articles, maxRelatedNodes, maxGraphEdges)
}

// graphEdgesWithinBudget reads no more than one sentinel row beyond the remaining response budget.
func (s *Server) graphEdgesWithinBudget(ctx context.Context, mode string, articles []map[string]any, relatedBudget, edgeBudget int) ([]map[string]any, []map[string]any, bool, error) {
	if mode == "research_network" {
		return s.graphResearchNetwork(ctx, articles)
	}
	nodes, edges := make([]map[string]any, 0, len(articles)), make([]map[string]any, 0)
	articleIDs := make([]int64, 0, len(articles))
	revisionByWork := map[int64]map[string]any{}
	for _, article := range articles {
		id := article["id"].(int64)
		workID := article["work_id"].(int64)
		articleIDs = append(articleIDs, id)
		revisionByWork[workID] = article
		nodes = append(nodes, map[string]any{"id": "article:" + stringID(id), "type": "article", "revision_id": id, "work_id": workID, "label": article["title"], "doi": article["doi"]})
	}
	if len(articleIDs) == 0 {
		return nodes, edges, false, nil
	}
	if relatedBudget < 0 {
		relatedBudget = 0
	}
	if edgeBudget < 0 {
		edgeBudget = 0
	}
	placeholders, args := placeholders(articleIDs)
	var query string
	switch mode {
	case "article_author":
		query = `SELECT a.work_revision_id, ao.id AS author_id, ao.citation_name, ao.orcid, a.author_order, a.affiliation
            FROM authorships a JOIN author_occurrences ao ON ao.id=a.author_occurrence_id
            WHERE a.work_revision_id IN (` + placeholders + `) ORDER BY a.id`
	case "citation":
		query = `SELECT rm.work_revision_id, rm.resolved_work_id FROM reference_mentions rm
            WHERE rm.work_revision_id IN (` + placeholders + `) AND rm.resolved_work_id IS NOT NULL ORDER BY rm.id`
	case "article_reference":
		query = `SELECT rm.id AS reference_id, rm.work_revision_id, rm.doi, rm.title, rm.author, rm.year, rm.source
            FROM reference_mentions rm WHERE rm.work_revision_id IN (` + placeholders + `) ORDER BY rm.id`
	}
	query += " LIMIT ?"
	args = append(args, edgeBudget+1)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, nil, false, err
	}
	defer rows.Close()
	items, err := rowsAsMaps(rows)
	if err != nil {
		return nil, nil, false, err
	}
	related := map[string]bool{}
	truncated := len(items) > edgeBudget
	if len(items) > edgeBudget {
		items = items[:edgeBudget]
	}
	for _, item := range items {
		if len(edges) >= maxGraphEdges {
			truncated = true
			break
		}
		from := "article:" + stringID(item["work_revision_id"].(int64))
		switch mode {
		case "article_author":
			to := "author:" + stringID(item["author_id"].(int64))
			if !related[to] {
				if len(related) >= relatedBudget {
					truncated = true
					continue
				}
				related[to] = true
				nodes = append(nodes, map[string]any{"id": to, "type": "author", "author_id": item["author_id"], "label": item["citation_name"], "orcid": item["orcid"]})
			}
			edges = append(edges, map[string]any{"id": "authorship:" + stringID(item["work_revision_id"].(int64)) + ":" + stringID(item["author_id"].(int64)) + ":" + stringID(item["author_order"].(int64)), "source": from, "target": to, "type": "authorship", "author_order": item["author_order"], "affiliation": item["affiliation"]})
		case "citation":
			if target, ok := revisionByWork[item["resolved_work_id"].(int64)]; ok {
				to := "article:" + stringID(target["id"].(int64))
				edges = append(edges, map[string]any{"id": "citation:" + stringID(item["work_revision_id"].(int64)) + ":" + stringID(item["resolved_work_id"].(int64)), "source": from, "target": to, "type": "citation"})
			}
		case "article_reference":
			to := "reference:" + stringID(item["reference_id"].(int64))
			if !related[to] {
				if len(related) >= relatedBudget {
					truncated = true
					continue
				}
				related[to] = true
				nodes = append(nodes, map[string]any{"id": to, "type": "reference", "reference_id": item["reference_id"], "label": item["title"], "doi": item["doi"], "author": item["author"], "year": item["year"]})
			}
			edges = append(edges, map[string]any{"id": "reference:" + stringID(item["reference_id"].(int64)), "source": from, "target": to, "type": "reference"})
		}
	}
	return nodes, edges, truncated, nil
}

// graphResearchNetwork combines authorship, reference, citation, coauthor, and bibliographic-coupling relationships.
func (s *Server) graphResearchNetwork(ctx context.Context, articles []map[string]any) ([]map[string]any, []map[string]any, bool, error) {
	nodes := make([]map[string]any, 0, len(articles)*3)
	edges := make([]map[string]any, 0, len(articles)*5)
	nodeIDs := make(map[string]bool)
	edgeIDs := make(map[string]bool)
	relatedNodes := 0
	truncated := false

	addNode := func(node map[string]any) bool {
		id, _ := node["id"].(string)
		if id == "" || nodeIDs[id] {
			return nodeIDs[id]
		}
		if node["type"] != "article" && relatedNodes >= maxRelatedNodes {
			truncated = true
			return false
		}
		nodeIDs[id] = true
		nodes = append(nodes, node)
		if node["type"] != "article" {
			relatedNodes++
		}
		return true
	}
	addEdge := func(edge map[string]any) bool {
		if len(edges) >= maxGraphEdges {
			truncated = true
			return false
		}
		id, _ := edge["id"].(string)
		source, _ := edge["source"].(string)
		target, _ := edge["target"].(string)
		if id == "" || edgeIDs[id] || !nodeIDs[source] || !nodeIDs[target] {
			return false
		}
		edgeIDs[id] = true
		edges = append(edges, edge)
		return true
	}

	for _, article := range articles {
		id := article["id"].(int64)
		addNode(map[string]any{
			"id": "article:" + stringID(id), "type": "article", "revision_id": id,
			"work_id": article["work_id"], "label": article["title"], "doi": article["doi"],
		})
	}

	for _, mode := range []string{"article_author", "article_reference", "citation"} {
		remainingNodes := maxRelatedNodes - relatedNodes
		remainingEdges := maxGraphEdges - len(edges)
		if remainingNodes <= 0 || remainingEdges <= 0 {
			truncated = true
			break
		}
		modeNodes, modeEdges, modeTruncated, err := s.graphEdgesWithinBudget(ctx, mode, articles, remainingNodes, remainingEdges)
		if err != nil {
			return nil, nil, false, err
		}
		truncated = truncated || modeTruncated
		for _, node := range modeNodes {
			addNode(node)
		}
		for _, edge := range modeEdges {
			addEdge(edge)
		}
	}

	// Raw reference-author strings are evidence, not confirmed person identities.
	for _, node := range append([]map[string]any(nil), nodes...) {
		if node["type"] != "reference" {
			continue
		}
		author, _ := node["author"].(string)
		author = strings.TrimSpace(author)
		if author == "" {
			continue
		}
		sum := sha256.Sum256([]byte(strings.ToLower(author)))
		authorID := "referenced_author:" + hex.EncodeToString(sum[:8])
		if !addNode(map[string]any{"id": authorID, "type": "referenced_author", "label": author, "identity_status": "raw_reference_string"}) {
			continue
		}
		referenceID, _ := node["id"].(string)
		addEdge(map[string]any{
			"id":     "reference_author:" + strings.TrimPrefix(referenceID, "reference:") + ":" + strings.TrimPrefix(authorID, "referenced_author:"),
			"source": referenceID, "target": authorID, "type": "reference_author",
		})
	}

	// Co-author edges are derived only from authorships attached to the same article.
	authorsByArticle := make(map[string][]string)
	articleByReference := make(map[string]string)
	referenceDOI := make(map[string]string)
	for _, edge := range edges {
		edgeType, _ := edge["type"].(string)
		source, _ := edge["source"].(string)
		target, _ := edge["target"].(string)
		if edgeType == "authorship" {
			authorsByArticle[source] = append(authorsByArticle[source], target)
		}
		if edgeType == "reference" {
			articleByReference[target] = source
		}
	}
	for _, node := range nodes {
		if node["type"] != "reference" {
			continue
		}
		id, _ := node["id"].(string)
		doi, _ := node["doi"].(string)
		doi = strings.ToLower(strings.TrimSpace(doi))
		if doi != "" {
			referenceDOI[id] = doi
		}
	}

	articleIDs := make([]string, 0, len(authorsByArticle))
	for articleID := range authorsByArticle {
		articleIDs = append(articleIDs, articleID)
	}
	sort.Strings(articleIDs)
coauthorPairs:
	for _, articleID := range articleIDs {
		authors := authorsByArticle[articleID]
		for left := 0; left < len(authors); left++ {
			for right := left + 1; right < len(authors); right++ {
				if len(edges) >= maxGraphEdges {
					truncated = true
					break coauthorPairs
				}
				pair := []string{authors[left], authors[right]}
				sort.Strings(pair)
				addEdge(map[string]any{
					"id":     "coauthor:" + strings.TrimPrefix(articleID, "article:") + ":" + pair[0] + ":" + pair[1],
					"source": pair[0], "target": pair[1], "type": "coauthor", "article_id": articleID,
				})
			}
		}
	}

	// Bibliographic coupling is derived from two selected articles citing the same DOI.
	articlesByDOI := make(map[string]map[string]bool)
	for referenceID, doi := range referenceDOI {
		articleID := articleByReference[referenceID]
		if articleID == "" {
			continue
		}
		if articlesByDOI[doi] == nil {
			articlesByDOI[doi] = make(map[string]bool)
		}
		articlesByDOI[doi][articleID] = true
	}
	pairCounts := make(map[string]int)
	pairArticles := make(map[string][2]string)
	pairBudget := maxGraphEdges - len(edges)
pairCollection:
	for _, articlesForDOI := range articlesByDOI {
		ids := make([]string, 0, len(articlesForDOI))
		for id := range articlesForDOI {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for left := 0; left < len(ids); left++ {
			for right := left + 1; right < len(ids); right++ {
				key := ids[left] + "|" + ids[right]
				if _, exists := pairCounts[key]; !exists && len(pairCounts) >= pairBudget {
					truncated = true
					break pairCollection
				}
				pairCounts[key]++
				pairArticles[key] = [2]string{ids[left], ids[right]}
			}
		}
	}
	pairKeys := make([]string, 0, len(pairCounts))
	for key := range pairCounts {
		pairKeys = append(pairKeys, key)
	}
	sort.Strings(pairKeys)
	for _, key := range pairKeys {
		pair := pairArticles[key]
		addEdge(map[string]any{
			"id":     "shared_reference:" + strings.TrimPrefix(pair[0], "article:") + ":" + strings.TrimPrefix(pair[1], "article:"),
			"source": pair[0], "target": pair[1], "type": "shared_reference", "shared_reference_count": pairCounts[key],
		})
	}

	return nodes, edges, truncated, nil
}

// placeholders returns a comma-separated SQL placeholder list and matching identifier arguments.
func placeholders(ids []int64) (string, []any) {
	parts, args := make([]string, len(ids)), make([]any, len(ids))
	for i, id := range ids {
		parts[i] = "?"
		args[i] = id
	}
	return strings.Join(parts, ","), args
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
