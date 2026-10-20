// Package graphview assembles bounded viewer relationship graphs over handwritten work-family APIs.
package graphview

import (
	"analysis/backend/database/work"
	"analysis/internal/jsonvalue"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

// Build reads no more than one sentinel row beyond the remaining response budget.
func Build(ctx context.Context, store *work.Store, mode string, articles []map[string]any, relatedBudget, edgeBudget int) ([]map[string]any, []map[string]any, bool, error) {
	if mode == "research_network" {
		return researchNetwork(ctx, store, articles, relatedBudget, edgeBudget)
	}
	nodes, edges := make([]map[string]any, 0, len(articles)), make([]map[string]any, 0)
	articleIDs := make([]int64, 0, len(articles))
	workIDs := make([]int64, 0, len(articles))
	revisionByWork := map[int64]map[string]any{}
	for _, article := range articles {
		id := article["id"].(int64)
		workID := article["work_id"].(int64)
		articleIDs = append(articleIDs, id)
		workIDs = append(workIDs, workID)
		revisionByWork[workID] = article
		nodes = append(nodes, map[string]any{"id": "article:" + fmt.Sprint(id), "type": "article", "revision_id": id, "work_id": workID, "label": article["title"], "doi": article["doi"]})
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
	queryLimit := edgeBudget + 1
	var items []map[string]any
	switch mode {
	case "article_author":
		authorLimit := relatedBudget
		if authorLimit == 0 {
			authorLimit = 1
		}
		authorships, err := store.ListGraphAuthorships(ctx, articleIDs, authorLimit, queryLimit)
		if err != nil {
			return nil, nil, false, err
		}
		for _, authorship := range authorships {
			items = append(items, graphAuthorshipItem(authorship))
		}
	case "citation":
		citations, err := store.ListGraphCitations(ctx, articleIDs, workIDs, queryLimit)
		if err != nil {
			return nil, nil, false, err
		}
		for _, citation := range citations {
			items = append(items, graphCitationItem(citation))
		}
	case "article_reference":
		if relatedBudget < edgeBudget {
			queryLimit = relatedBudget + 1
		}
		references, err := store.ListGraphReferences(ctx, articleIDs, queryLimit)
		if err != nil {
			return nil, nil, false, err
		}
		for _, reference := range references {
			items = append(items, graphReferenceItem(reference))
		}
	}
	related := map[string]bool{}
	truncated := len(items) == queryLimit
	if len(items) > edgeBudget {
		items = items[:edgeBudget]
	}
	for _, item := range items {
		if len(edges) >= edgeBudget {
			truncated = true
			break
		}
		from := "article:" + fmt.Sprint(item["work_revision_id"].(int64))
		switch mode {
		case "article_author":
			to := "author:" + fmt.Sprint(item["author_id"].(int64))
			if !related[to] {
				if len(related) >= relatedBudget {
					truncated = true
					continue
				}
				related[to] = true
				nodes = append(nodes, map[string]any{"id": to, "type": "author", "author_id": item["author_id"], "label": item["citation_name"], "orcid": item["orcid"]})
			}
			edges = append(edges, map[string]any{"id": "authorship:" + fmt.Sprint(item["work_revision_id"].(int64)) + ":" + fmt.Sprint(item["author_id"].(int64)) + ":" + fmt.Sprint(item["author_order"].(int64)), "source": from, "target": to, "type": "authorship", "author_order": item["author_order"], "affiliation": item["affiliation"]})
		case "citation":
			if target, ok := revisionByWork[item["resolved_work_id"].(int64)]; ok {
				to := "article:" + fmt.Sprint(target["id"].(int64))
				edges = append(edges, map[string]any{"id": "citation:" + fmt.Sprint(item["work_revision_id"].(int64)) + ":" + fmt.Sprint(item["resolved_work_id"].(int64)), "source": from, "target": to, "type": "citation"})
			}
		case "article_reference":
			to := "reference:" + fmt.Sprint(item["reference_id"].(int64))
			if !related[to] {
				if len(related) >= relatedBudget {
					truncated = true
					continue
				}
				related[to] = true
				nodes = append(nodes, map[string]any{"id": to, "type": "reference", "reference_id": item["reference_id"], "label": item["title"], "doi": item["doi"], "author": item["author"], "year": item["year"]})
			}
			edges = append(edges, map[string]any{"id": "reference:" + fmt.Sprint(item["reference_id"].(int64)), "source": from, "target": to, "type": "reference"})
		}
	}
	return nodes, edges, truncated, nil
}

// researchNetwork combines authorship, reference, citation, coauthor, and bibliographic-coupling relationships.
func researchNetwork(ctx context.Context, store *work.Store, articles []map[string]any, relatedBudget, edgeBudget int) ([]map[string]any, []map[string]any, bool, error) {
	if relatedBudget < 0 {
		relatedBudget = 0
	}
	if edgeBudget < 0 {
		edgeBudget = 0
	}
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
		if node["type"] != "article" && relatedNodes >= relatedBudget {
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
		if len(edges) >= edgeBudget {
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
			"id": "article:" + fmt.Sprint(id), "type": "article", "revision_id": id,
			"work_id": article["work_id"], "label": article["title"], "doi": article["doi"],
		})
	}

	for _, mode := range []string{"article_author", "article_reference", "citation"} {
		remainingNodes := relatedBudget - relatedNodes
		remainingEdges := edgeBudget - len(edges)
		if remainingNodes <= 0 || remainingEdges <= 0 {
			truncated = true
			break
		}
		modeNodes, modeEdges, modeTruncated, err := Build(ctx, store, mode, articles, remainingNodes, remainingEdges)
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
				if len(edges) >= edgeBudget {
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
	pairBudget := edgeBudget - len(edges)
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

// graphAuthorshipItem maps one work family graph authorship into a viewer row.
func graphAuthorshipItem(authorship *work.GraphAuthorship) map[string]any {
	return map[string]any{
		"work_revision_id": authorship.WorkRevisionID,
		"author_id":        authorship.AuthorID,
		"citation_name":    authorship.CitationName,
		"orcid":            jsonvalue.Optional(authorship.ORCID),
		"author_order":     int64(authorship.AuthorOrder),
		"affiliation":      jsonvalue.Optional(authorship.Affiliation),
	}
}

// graphCitationItem maps one work family graph citation into a viewer row.
func graphCitationItem(citation *work.GraphCitation) map[string]any {
	return map[string]any{
		"work_revision_id": citation.WorkRevisionID,
		"resolved_work_id": citation.ResolvedWorkID,
	}
}

// graphReferenceItem maps one work family graph reference into a viewer row.
func graphReferenceItem(reference *work.GraphReference) map[string]any {
	return map[string]any{
		"reference_id":     reference.ID,
		"work_revision_id": reference.WorkRevisionID,
		"doi":              jsonvalue.Optional(reference.DOI),
		"title":            jsonvalue.Optional(reference.Title),
		"author":           jsonvalue.Optional(reference.Author),
		"year":             jsonvalue.Optional(reference.Year),
		"source":           jsonvalue.Optional(reference.Source),
	}
}
