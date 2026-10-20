// Package validator owns the workspace article validation partitioning and audit evidence.
package validator

import (
	"analysis/backend/article"
	"analysis/backend/database"

	"analysis/backend/manifest"

	"analysis/backend/validation"

	"context"
	"encoding/json"
	"fmt"

	"strconv"

	"time"
)

// Validate partitions articles by validation outcome and records discard reasons by DOI.
func Validate(articles []*article.Article) (valid, discarded []*article.Article, reasons map[string][]string) {
	reasons = make(map[string][]string)
	for _, a := range articles {
		problems := validation.ValidateFields(validation.Fields{DOI: a.DOI, Title: a.Title, Year: a.Year, Publisher: a.Publisher, ReferenceCount: len(a.CitedReferences)}, len(a.Authors))
		if len(problems) == 0 {
			valid = append(valid, a)
			continue
		}
		discarded = append(discarded, a)
		reasons[a.DOI] = problems
	}
	return valid, discarded, reasons
}

// RecordAudit records validation audit.
func RecordAudit(ctx context.Context, db *database.Database, runID int64, articles []*article.Article, reasons map[string][]string) error {
	for _, a := range articles {
		work, err := db.Work.GetWorkByDOI(ctx, a.DOI)
		if err != nil {
			return err
		}
		if work == nil {
			return fmt.Errorf("work missing for validation audit DOI %q", a.DOI)
		}
		status := "valid"
		metadata := "{}"
		if problems := reasons[a.DOI]; len(problems) > 0 {
			status = "discarded"
			data, err := json.Marshal(map[string]any{"reasons": problems})
			if err != nil {
				return err
			}
			metadata = string(data)
		}
		if _, err := db.Audit.Insert(ctx, &manifest.AuditEvent{OccurredAt: time.Now().UTC().Format(time.RFC3339Nano), Actor: "pipeline", PipelineRunID: runID, EntityType: "work", EntityID: strconv.FormatInt(work.ID, 10), Action: manifest.AuditValidationChanged, AfterJSON: fmt.Sprintf(`{"status":%q}`, status), MetadataJSON: metadata, CorrelationID: "validation-" + strconv.FormatInt(runID, 10)}); err != nil {
			return err
		}
	}
	return nil
}
