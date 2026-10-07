// relationships.go provides link resolution and backlink navigation across
// note versions, work revisions, anchors, and external targets without
// rewriting persisted link identity.
package review

import (
	"context"
	"database/sql"
	"errors"
	"net/url"
	"strconv"

	generated "analysis/database/review/internal/sql"
)

// linksForVersion reads links in source order and resolves them against the selected context.
func (s *Store) linksForVersion(ctx context.Context, queries *generated.Queries, contextID, noteID, versionID int64) ([]Link, error) {
	rows, err := queries.ListReviewNoteLinks(ctx, versionID)
	if err != nil {
		return nil, err
	}
	links := make([]Link, 0, len(rows))
	for _, row := range rows {
		link := Link{
			Ordinal:       int(row.Ordinal),
			TargetType:    row.TargetType,
			RawTarget:     row.RawTarget,
			UTF16Position: int(row.Utf16Position),
			UTF16Length:   int(row.Utf16Length),
		}
		if row.DisplayText.Valid {
			link.DisplayText = &row.DisplayText.String
		}
		links = append(links, link)
	}
	for index := range links {
		if err := s.resolveLink(ctx, queries, contextID, noteID, &links[index]); err != nil {
			return nil, err
		}
	}
	return links, nil
}

// resolveLink enriches a syntactically valid link without rewriting persisted link identity.
func (s *Store) resolveLink(ctx context.Context, queries *generated.Queries, contextID, sourceNoteID int64, link *Link) error {
	switch link.TargetType {
	case "note":
		noteID, _ := strconv.ParseInt(link.RawTarget, 10, 64)
		revisionID, err := queries.ResolveReviewLinkNote(ctx, generated.ResolveReviewLinkNoteParams{
			ReviewContextID: contextID,
			NoteID:          noteID,
		})
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		link.Resolved, link.WorkRevisionID, link.NoteID = true, &revisionID, &noteID
	case "article":
		revisionID, err := queries.ResolveReviewLinkArticle(ctx, generated.ResolveReviewLinkArticleParams{
			ReviewContextID: contextID,
			Doi:             sql.NullString{String: link.RawTarget, Valid: true},
		})
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		link.Resolved, link.WorkRevisionID = true, &revisionID
	case "pdf_page":
		page, _ := strconv.Atoi(link.RawTarget)
		revisionID, err := queries.ResolveReviewLinkPDFPage(ctx, generated.ResolveReviewLinkPDFPageParams{
			ReviewContextID: contextID,
			NoteID:          sourceNoteID,
		})
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		link.Resolved, link.WorkRevisionID, link.Page = true, &revisionID, &page
	case "anchor":
		row, err := queries.ResolveReviewLinkAnchor(ctx, generated.ResolveReviewLinkAnchorParams{
			ReviewContextID: contextID,
			AnchorID:        link.RawTarget,
		})
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		revisionID := row.WorkRevisionID
		page := int(row.Page.Int64)
		anchorID := link.RawTarget
		link.Resolved, link.WorkRevisionID, link.AnchorID, link.Page = true, &revisionID, &anchorID, &page
	case "ext":
		parsed, err := url.Parse(link.RawTarget)
		if err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != "" {
			value := parsed.String()
			link.Resolved, link.URL = true, &value
		}
	}
	return nil
}

// ListBacklinks returns links from current note heads only.
func (s *Store) ListBacklinks(ctx context.Context, contextID int64, targetType, targetID string, sourceWorkID, cursor int64, limit int) ([]Note, error) {
	if limit < 1 || limit > reviewListLimit {
		return nil, reviewValidation("backlink fetch limit must be between 1 and 101")
	}
	rows, err := s.queries.ListReviewBacklinks(ctx, generated.ListReviewBacklinksParams{
		PreviewBytes:    reviewTextPreviewBytes,
		ReviewContextID: contextID,
		TargetType:      targetType,
		TargetID:        targetID,
		SourceWorkID:    sourceWorkID,
		Cursor:          cursor,
		PageLimit:       int64(limit),
	})
	if err != nil {
		return nil, err
	}
	projections := make([]noteSummaryProjection, 0, len(rows))
	for _, row := range rows {
		projections = append(projections, noteSummaryFromBacklinkRow(row))
	}
	return noteSummaries(projections, contextID), nil
}
