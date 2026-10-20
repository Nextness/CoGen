// normalization_unit_test.go tests the author name, affiliation, publisher,
// and journal normalisation functions.

package normalization_test

import subject "analysis/backend/normalization"

import "testing"

// =========================================================================
// NormalizeAuthorName tests
// =========================================================================

// TestNormalizeAuthorName verifies normalize author name.
func TestNormalizeAuthorName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Maarouk, Toufik Messaoud", "Maarouk, T. M."},
		{"Wil M. P. van der Aalst", "van der Aalst, W. M. P."},
		{"J.Mendling", "Mendling, J."},
		{"Ackoff RL.", "Ackoff, R. L."},
		{"AHMED KHAN, Shoab", "Ahmed Khan, S."},
		{"SomeCorp", "Somecorp"},
		{"", ""},
		{"John Michael Doe J.", "John Michael Doe, J."},
		{"Érico Álvares", "Álvares, É."},
	}

	for _, tt := range tests {
		got := subject.NormalizeAuthorName(tt.input)
		if got != tt.want {
			t.Errorf("NormalizeAuthorName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}

	// Test nil-like (empty string handled above)
	if subject.NormalizeAuthorName("   ") != "" {
		t.Error("expected empty for whitespace")
	}

	// Test multi-word surname with initials (3+ words where last is initials)
	result := subject.NormalizeAuthorName("John Michael Doe J.")
	if result != "John Michael Doe, J." {
		t.Errorf("expected 'John Michael Doe, J.', got %q", result)
	}

	// Test 3+ given words without initials
	result = subject.NormalizeAuthorName("University of Somewhere")
	if result == "" {
		t.Error("expected non-empty for 'University of Somewhere'")
	}
}

// =========================================================================
// SplitFirstLast tests
// =========================================================================

// TestSplitFirstLast verifies split first last.
func TestSplitFirstLast(t *testing.T) {
	first, last := subject.SplitFirstLast("Maarouk, T. M.")
	if first != "T. M." || last != "Maarouk" {
		t.Errorf("expected (T. M., Maarouk), got (%q, %q)", first, last)
	}

	first, last = subject.SplitFirstLast("SingleName")
	if first != "" || last != "SingleName" {
		t.Errorf("expected ('', SingleName), got (%q, %q)", first, last)
	}
}

// =========================================================================
// NormalizeAffiliation tests
// =========================================================================

// TestNormalizeAffiliation verifies normalize affiliation.
func TestNormalizeAffiliation(t *testing.T) {
	// Abbreviation expansion
	result := subject.NormalizeAffiliation("Univ. of California")
	if !contains(result, "University") {
		t.Errorf("expected 'University' in %q", result)
	}

	// Title casing
	result = subject.NormalizeAffiliation("federal university of rio de janeiro")
	if result != "Federal University of Rio de Janeiro" {
		t.Errorf("expected 'Federal University of Rio de Janeiro', got %q", result)
	}

	// Strip trailing period
	result = subject.NormalizeAffiliation("Stanford University.")
	if result != "Stanford University" {
		t.Errorf("expected 'Stanford University', got %q", result)
	}

	// Empty
	if subject.NormalizeAffiliation("") != "" {
		t.Error("expected empty for empty string")
	}

	// Roman numerals
	result = subject.NormalizeAffiliation("University of California, San Francisco, campus box Iii")
	if !contains(result, "III") {
		t.Errorf("expected 'III' in %q", result)
	}

	// Acronym-like words
	result = subject.NormalizeAffiliation("Sao Paulo")
	if !contains(result, "Sao") {
		t.Errorf("expected 'Sao' in %q", result)
	}

	// Hyphenated
	result = subject.NormalizeAffiliation("state-of-the-art lab")
	if !contains(result, "State-of-the-art Lab") {
		t.Errorf("expected 'State-of-the-art Lab' in %q", result)
	}

	// Various abbreviations
	result = subject.NormalizeAffiliation("dept. of comp. sci., univ. of oxford")
	if !contains(result, "Department") {
		t.Errorf("expected 'Department' in %q", result)
	}
	if !contains(result, "University") {
		t.Errorf("expected 'University' in %q", result)
	}

	// Dash-separated segments
	result = subject.NormalizeAffiliation("Main Campus — Secondary Site")
	if !contains(result, "Main") {
		t.Errorf("expected 'Main' in %q", result)
	}

	result = subject.NormalizeAffiliation("école supérieure de paris")
	if result != "École Supérieure de Paris" {
		t.Errorf("expected Unicode title casing, got %q", result)
	}
}

// contains supports the package test suite's contains setup or assertions.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && containsStr(s, substr)
}

// containsStr supports the package test suite's contains str setup or assertions.
func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// =========================================================================
// NormalizePublisher tests
// =========================================================================

// TestNormalizePublisher verifies normalize publisher.
func TestNormalizePublisher(t *testing.T) {
	// Canonical mapping
	result := subject.NormalizePublisher("Institute of Electrical and Electronics Engineers (IEEE)")
	if result != "IEEE" {
		t.Errorf("expected 'IEEE', got %q", result)
	}

	// HTML unescape
	result = subject.NormalizePublisher("ACM &amp; IEEE")
	if !contains(result, "&") {
		t.Errorf("expected '&' in %q", result)
	}

	// Title casing
	result = subject.NormalizePublisher("springer nature")
	if result == "" || result[0] < 'A' || result[0] > 'Z' {
		t.Errorf("expected title-cased, got %q", result)
	}

	// Legal suffix preserved
	result = subject.NormalizePublisher("acme corp.")
	if !contains(result, "Corp.") {
		t.Errorf("expected 'Corp.' in %q", result)
	}

	// Dot stripped non-legal
	result = subject.NormalizePublisher("Random Publisher.")
	if result != "" && result[len(result)-1] == '.' {
		t.Errorf("expected no trailing dot, got %q", result)
	}

	// S.r.o. suffix
	result = subject.NormalizePublisher("Some Company s.r.o.")
	if !contains(result, "s.r.o.") {
		t.Errorf("expected 's.r.o.' in %q", result)
	}

	result = subject.NormalizePublisher("éditions universitaires")
	if result != "Éditions Universitaires" {
		t.Errorf("expected Unicode title casing, got %q", result)
	}
}

// =========================================================================
// NormalizeJournal tests
// =========================================================================

// TestNormalizeJournal verifies normalize journal.
func TestNormalizeJournal(t *testing.T) {
	// Canonical mapping
	result := subject.NormalizeJournal("INFORMATION SYSTEMS")
	if result != "Information Systems" {
		t.Errorf("expected 'Information Systems', got %q", result)
	}

	// Subtitle stripping
	result = subject.NormalizeJournal("Future Generation Computer Systems-The International Journal of eScience")
	if result != "Future Generation Computer Systems" {
		t.Errorf("expected 'Future Generation Computer Systems', got %q", result)
	}

	// Acronym in name
	result = subject.NormalizeJournal("ieee transactions on software engineering")
	if !hasPrefix(result, "IEEE") {
		t.Errorf("expected 'IEEE...', got %q", result)
	}

	// Parenthesized content
	result = subject.NormalizeJournal("Journal of Research (Science)")
	if !contains(result, "(Science)") {
		t.Errorf("expected '(Science)' in %q", result)
	}

	result = subject.NormalizeJournal("journal of research (science (technical))")
	if result != "Journal of Research (Science (Technical))" {
		t.Errorf("nested parentheses = %q", result)
	}

	result = subject.NormalizeJournal("journal of research (science")
	if result != "Journal of Research (Science" {
		t.Errorf("unmatched opening parenthesis = %q", result)
	}

	result = subject.NormalizeJournal("journal of research science)")
	if result != "Journal of Research Science)" {
		t.Errorf("unmatched closing parenthesis = %q", result)
	}

	// Ampersand
	result = subject.NormalizeJournal("research & development journal")
	if !contains(result, "&") {
		t.Errorf("expected '&' in %q", result)
	}

	// Hyphenated words
	result = subject.NormalizeJournal("journal of software-evolution and process")
	if !contains(result, "Software-Evolution") && !contains(result, "Software-evolution") {
		t.Errorf("expected 'Software-Evolution' variant in %q", result)
	}

	result = subject.NormalizeJournal("revista de administração")
	if result != "Revista de Administração" {
		t.Errorf("expected Unicode title casing, got %q", result)
	}
}

// hasPrefix supports the package test suite's has prefix setup or assertions.
func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

// TestAuthorNameBoundaryCases covers the original helper inputs through normalized names and first/last projections.
func TestAuthorNameBoundaryCases(t *testing.T) {
	for _, test := range []struct{ input, want string }{
		{"Smith WM", "Smith, W. M."}, {"Smith RL", "Smith, R. L."},
		{"John van", "van, J."}, {"John der", "der, J."},
		{"van", "van"}, {"der", "der"}, {"mccarthy", "McCarthy"}, {"mcdonald", "McDonald"}, {"macdonald", "MacDonald"},
		{"o'brien", "O'Brien"}, {"d'lambert", "D'Lambert"}, {"saint-pierre", "Saint-Pierre"}, {"smith", "Smith"},
		{"Smith, F.", "Smith, F."}, {"Smith, John", "Smith, J."}, {"Smith, abc", "Smith, A."}, {"Smith, Äbc", "Smith, Ä."}, {"Smith, a", "Smith, A."},
		{"Smith, h", "Smith, H."}, {"Smith, é", "Smith, É."},
		{"", ""}, {"   ", ""}, {"R.L.", "R. L."}, {"RL.", "R. L."}, {"  Smith", "Smith"},
		{"Wil M. P. van der Aalst", "van der Aalst, W. M. P."},
		{"SingleName", "Singlename"}, {"Name,", "Name,"}, {",Name", ",name"},
		{"Hello", "Hello"}, {"École", "École"}, {"123abc", "123abc"}, {"über", "Über"},
		{"IEEE", "Ieee"}, {"ieee", "Ieee"}, {"Abc", "Abc"}, {"123", "123"}, {"A", "A."}, {"A B", "B., A."},
	} {
		t.Run(test.input, func(t *testing.T) {
			if got := subject.NormalizeAuthorName(test.input); got != test.want {
				t.Errorf("NormalizeAuthorName(%q)=%q, want %q", test.input, got, test.want)
			}
		})
	}
}

// TestAffiliationBoundaryCases covers abbreviations, particles, numerals, Unicode, and compound casing through the public normalizer.
func TestAffiliationBoundaryCases(t *testing.T) {
	for _, test := range []struct{ input, want string }{
		{"Univ. of Oxford", "University of Oxford"}, {"Dept. of CS", "Department of CS"}, {"Acme Corp.", "Acme Corporation"},
		{"", ""}, {"NoAbbreviation", "Noabbreviation"}, {"univ.", "University"}, {"Research of Science", "Research of Science"}, {"Of", "Of"},
		{"Lab iii", "Lab III"}, {"federal", "Federal"}, {"de", "De"}, {"state-of-the-art", "State-of-the-art"}, {"l'école", "L'école"},
		{"federal university of rio de janeiro", "Federal University of Rio de Janeiro"}, {"main campus — secondary site", "Main Campus — Secondary Site"},
	} {
		t.Run(test.input, func(t *testing.T) {
			if got := subject.NormalizeAffiliation(test.input); got != test.want {
				t.Errorf("NormalizeAffiliation(%q)=%q, want %q", test.input, got, test.want)
			}
		})
	}
}

// TestPublisherBoundaryCases covers acronym bounds, legal suffixes, punctuation, and title words through the public normalizer.
func TestPublisherBoundaryCases(t *testing.T) {
	for _, test := range []struct{ input, want string }{
		{"IEEE", "IEEE"}, {"ABC", "ABC"}, {"Word", "Word"}, {"", ""}, {"A", "a"}, {"ABCDEFG", "ABCDEFG"}, {"ABCDEFGH", "Abcdefgh"}, {"IEEE.", "IEEE"},
		{"of", "of"}, {"the", "the"}, {"ltd", "Ltd"}, {"inc", "Inc."}, {"springer", "Springer"}, {"non-standard", "Non-standard"},
		{"  ", ""}, {"and", "and"}, {"Springer", "Springer"}, {"Éditions", "Éditions"}, {"Acme Corp.", "Acme Corp."},
		{"Company Ltd", "Company Ltd"}, {"Some Company s.r.o.", "Some Company s.r.o."}, {"Sp. z o.o.", "Sp. z o.o."},
		{"springer nature", "Springer Nature"}, {"acme (UK)", "Acme (UK)"}, {"of the world", "of the World"},
	} {
		t.Run(test.input, func(t *testing.T) {
			if got := subject.NormalizePublisher(test.input); got != test.want {
				t.Errorf("NormalizePublisher(%q)=%q, want %q", test.input, got, test.want)
			}
		})
	}
}

// TestJournalBoundaryCases covers acronym punctuation, subtitles, and parenthesis-aware casing through the public normalizer.
func TestJournalBoundaryCases(t *testing.T) {
	for _, test := range []struct{ input, want string }{
		{"IEEE", "IEEE"}, {"ACM", "ACM"}, {"Nature", "Nature"}, {"", ""}, {"ieee", "IEEE"}, {"ACM.", "ACM."},
		{"Future Generation Computer Systems-The International Journal of eScience", "Future Generation Computer Systems"},
		{"IEEE Access", "IEEE Access"}, {"Journal of Research — A Subtitle", "Journal of Research"}, {"Software and Systems Modeling", "Software and Systems Modeling"},
		{"  ", ""}, {"information systems", "Information Systems"}, {"ieee transactions on software engineering", "IEEE Transactions on Software Engineering"},
		{"journal of the acm", "Journal of the ACM"}, {"journal of research (science)", "Journal of Research (Science)"},
		{"journal of software-evolution and process", "Journal of Software-Evolution and Process"},
	} {
		t.Run(test.input, func(t *testing.T) {
			if got := subject.NormalizeJournal(test.input); got != test.want {
				t.Errorf("NormalizeJournal(%q)=%q, want %q", test.input, got, test.want)
			}
		})
	}
}
