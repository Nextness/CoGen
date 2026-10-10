// author_reads.go converts author family application types into the viewer's
// stable JSON row shapes. Nullable fields are made explicit here so family
// types never leak into responses.
package server

import (
	"analysis/database/author"
)

// authorOccurrenceRow maps one run-scoped author occurrence into a viewer row.
func authorOccurrenceRow(occurrence *author.ViewerOccurrence) map[string]any {
	row := map[string]any{
		"id":            occurrence.ID,
		"citation_name": occurrence.CitationName,
		"created_at":    occurrence.CreatedAt,
	}
	if occurrence.PersonID != nil {
		row["person_id"] = *occurrence.PersonID
	} else {
		row["person_id"] = nil
	}
	row["first_name"] = optionalText(occurrence.FirstName)
	row["last_name"] = optionalText(occurrence.LastName)
	row["orcid"] = optionalText(occurrence.ORCID)
	row["person_orcid"] = optionalText(occurrence.PersonORCID)
	return row
}
