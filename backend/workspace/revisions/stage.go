// Package revisions owns the workspace immutable revision and relationship persistence.
package revisions

import (
	"analysis/backend/article"
	"analysis/backend/database"
	dbauthor "analysis/backend/database/author"

	dbwork "analysis/backend/database/work"

	"analysis/backend/normalization"

	"context"
	"encoding/json"
	"fmt"
)

// Persist persists workspace stage through the owning repository.
func Persist(ctx context.Context, db *database.Database, runID int64, articles []*article.Article, producerStage, stage, outcome string, reasons map[string][]string) (map[string]int64, map[string]int64, error) {
	workIDs := make(map[string]int64, len(articles))
	revisionIDs := make(map[string]int64, len(articles))
	for _, a := range articles {
		reason := ""
		stageOutcome := outcome
		if reasons != nil && len(reasons[a.DOI]) > 0 {
			reasonBytes, _ := json.Marshal(reasons[a.DOI])
			reason = string(reasonBytes)
			if stage == dbwork.StageNameValidate {
				stageOutcome = dbwork.OutcomeDiscarded
			}
		}
		workID, revisionID, err := persistWorkSnapshot(ctx, db, runID, a, producerStage, workspaceExtension(a, reasons[a.DOI]))
		if err != nil {
			return nil, nil, err
		}
		workIDs[a.DOI] = workID
		revisionIDs[a.DOI] = revisionID
		if err := db.Work.SetStageOutcome(ctx, dbwork.StageOutcomeInput{
			RunID: runID, WorkID: workID, StageName: stage, Outcome: stageOutcome, Reason: reason,
		}); err != nil {
			return nil, nil, err
		}
	}
	return workIDs, revisionIDs, nil
}

// persistWorkSnapshot persists work snapshot through the owning repository.
func persistWorkSnapshot(ctx context.Context, db *database.Database, runID int64, a *article.Article, producerStage, extensionData string) (int64, int64, error) {
	if a == nil {
		return 0, 0, fmt.Errorf("persist work snapshot: article is nil")
	}
	workID, err := db.Work.CreateWorkByDOI(ctx, a.DOI)
	if err != nil {
		return 0, 0, err
	}
	keywords, err := json.Marshal(a.Keywords)
	if err != nil {
		return 0, 0, err
	}
	keywordsPlus, err := json.Marshal(a.KeywordsAdditional)
	if err != nil {
		return 0, 0, err
	}
	revisionID, err := db.Work.CreateRevision(ctx, &dbwork.Revision{
		WorkID: workID, PipelineRunID: runID, ProducerStage: producerStage,
		Title: a.Title, Abstract: a.Abstract, Year: a.Year, Journal: a.Journal,
		Publisher: a.Publisher, Source: a.Source, Keywords: string(keywords),
		KeywordsPlus: string(keywordsPlus), CitationCount: a.CitationCount,
		ReferenceCount: len(a.CitedReferences), ExtensionData: extensionData,
	})
	if err != nil {
		return 0, 0, err
	}
	for order, authorRecord := range a.Authors {
		occurrenceID, err := db.Author.CreateOccurrence(ctx, &dbauthor.Occurrence{
			CitationName: authorRecord.CitationName, FirstName: authorRecord.FirstName,
			LastName: authorRecord.LastName, ORCID: authorRecord.Orcid,
		})
		if err != nil {
			return 0, 0, err
		}
		if _, err := db.Author.CreateAuthorship(ctx, &dbauthor.Authorship{
			WorkRevisionID: revisionID, AuthorOccurrenceID: occurrenceID,
			AuthorOrder: order + 1, Affiliation: authorRecord.Affiliation,
		}); err != nil {
			return 0, 0, err
		}
	}
	for order, reference := range a.CitedReferences {
		if _, err := db.Work.CreateReference(ctx, &dbwork.Reference{
			WorkRevisionID: revisionID, MentionOrder: order + 1, RawReference: reference.Raw,
			DOI: reference.DOI, Title: reference.Title, Author: reference.Author,
			Year: reference.Year, Source: reference.Source,
		}); err != nil {
			return 0, 0, err
		}
	}
	return workID, revisionID, nil
}

// workspaceRevisionExtension stores normalized values and validation evidence outside canonical revision columns.
type workspaceRevisionExtension struct {
	NormalizedJournal string            `json:"normalized_journal,omitempty"`
	ValidationReasons []string          `json:"validation_reasons,omitempty"`
	NormalizedAuthors map[string]string `json:"normalized_authors,omitempty"`
}

// workspaceExtension serializes normalized journal, author names, and validation reasons for a revision.
func workspaceExtension(a *article.Article, reasons []string) string {
	names := make(map[string]string)
	for _, author := range a.Authors {
		if author.NormalizedName != "" {
			names[author.CitationName] = author.NormalizedName
		}
	}
	data, _ := json.Marshal(workspaceRevisionExtension{NormalizedJournal: normalization.NormalizeJournal(a.Journal), ValidationReasons: reasons, NormalizedAuthors: names})
	return string(data)
}
