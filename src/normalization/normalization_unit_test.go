// normalization_unit_test.go tests the author name, affiliation, publisher,
// and journal normalisation functions.
//go:build unit

package normalization

import "testing"

// =========================================================================
// isInitials tests
// =========================================================================

// TestIsInitials verifies is initials.
func TestIsInitials(t *testing.T) {
	if !isInitials("WM") {
		t.Error("expected 'WM' to be initials")
	}
	if !isInitials("RL") {
		t.Error("expected 'RL' to be initials")
	}
	if isInitials("van") {
		t.Error("expected 'van' not to be initials")
	}
	if isInitials("der") {
		t.Error("expected 'der' not to be initials")
	}
}

// =========================================================================
// smartTitle tests
// =========================================================================

// TestSmartTitle verifies smart title.
func TestSmartTitle(t *testing.T) {
	if smartTitle("van") != "van" {
		t.Errorf("expected 'van', got %q", smartTitle("van"))
	}
	if smartTitle("der") != "der" {
		t.Errorf("expected 'der', got %q", smartTitle("der"))
	}
	if smartTitle("mccarthy") != "McCarthy" {
		t.Errorf("expected 'McCarthy', got %q", smartTitle("mccarthy"))
	}
	if smartTitle("mcdonald") != "McDonald" {
		t.Errorf("expected 'McDonald', got %q", smartTitle("mcdonald"))
	}
	if smartTitle("macdonald") != "MacDonald" {
		t.Errorf("expected 'MacDonald', got %q", smartTitle("macdonald"))
	}
	if smartTitle("o'brien") != "O'Brien" {
		t.Errorf("expected O'Brien, got %q", smartTitle("o'brien"))
	}
	if smartTitle("d'lambert") != "D'Lambert" {
		t.Errorf("expected D'Lambert, got %q", smartTitle("d'lambert"))
	}
	if smartTitle("saint-pierre") != "Saint-Pierre" {
		t.Errorf("expected 'Saint-Pierre', got %q", smartTitle("saint-pierre"))
	}
	if smartTitle("smith") != "Smith" {
		t.Errorf("expected 'Smith', got %q", smartTitle("smith"))
	}
}

// =========================================================================
// wordToInitial tests
// =========================================================================

// TestWordToInitial verifies word to initial.
func TestWordToInitial(t *testing.T) {
	if wordToInitial("F.") != "F." {
		t.Errorf("expected 'F.', got %q", wordToInitial("F."))
	}
	if wordToInitial("John") != "J." {
		t.Errorf("expected 'J.', got %q", wordToInitial("John"))
	}
	if wordToInitial("") != "" {
		t.Errorf("expected '', got %q", wordToInitial(""))
	}
}

// =========================================================================
// toInitials tests
// =========================================================================

// TestToInitials verifies to initials.
func TestToInitials(t *testing.T) {
	if toInitials("") != "" {
		t.Errorf("expected '', got %q", toInitials(""))
	}
	if toInitials("   ") != "" {
		t.Errorf("expected '', got %q", toInitials("   "))
	}
}

// =========================================================================
// normalizeFamilyWords tests
// =========================================================================

// TestNormalizeFamilyWords verifies normalize family words.
func TestNormalizeFamilyWords(t *testing.T) {
	result := normalizeFamilyWords([]string{"van"})
	if len(result) != 1 || result[0] != "van" {
		t.Errorf("expected ['van'], got %v", result)
	}

	result = normalizeFamilyWords([]string{"R.L."})
	if len(result) != 1 || result[0] != "R. L." {
		t.Errorf("expected ['R. L.'], got %v", result)
	}

	result = normalizeFamilyWords([]string{"RL."})
	if len(result) != 1 || result[0] != "R. L." {
		t.Errorf("expected ['R. L.'], got %v", result)
	}

	result = normalizeFamilyWords([]string{"Smith"})
	if len(result) != 1 || result[0] != "Smith" {
		t.Errorf("expected ['Smith'], got %v", result)
	}

	result = normalizeFamilyWords([]string{"", "Smith"})
	if len(result) != 1 || result[0] != "Smith" {
		t.Errorf("expected ['Smith'], got %v", result)
	}
}

// =========================================================================
// splitGivenFamily tests
// =========================================================================

// TestSplitGivenFamily verifies split given family.
func TestSplitGivenFamily(t *testing.T) {
	given, family := splitGivenFamily(nil)
	if len(given) != 0 || len(family) != 0 {
		t.Errorf("expected empty, got given=%v family=%v", given, family)
	}

	given, family = splitGivenFamily([]string{"Wil", "M.", "P.", "van", "der", "Aalst"})
	if len(given) != 3 || given[0] != "Wil" || given[1] != "M." || given[2] != "P." {
		t.Errorf("unexpected given: %v", given)
	}
	if len(family) != 3 || family[0] != "van" || family[1] != "der" || family[2] != "Aalst" {
		t.Errorf("unexpected family: %v", family)
	}
}

// =========================================================================
// splitOnComma tests
// =========================================================================

// TestSplitOnComma verifies split on comma.
func TestSplitOnComma(t *testing.T) {
	if splitOnComma("SingleName") != nil {
		t.Error("expected nil for no comma")
	}
	if splitOnComma("Name,") != nil {
		t.Error("expected nil for empty after comma")
	}
	if splitOnComma(",Name") != nil {
		t.Error("expected nil for empty before comma")
	}
}

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
		got := NormalizeAuthorName(tt.input)
		if got != tt.want {
			t.Errorf("NormalizeAuthorName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}

	// Test nil-like (empty string handled above)
	if NormalizeAuthorName("   ") != "" {
		t.Error("expected empty for whitespace")
	}

	// Test multi-word surname with initials (3+ words where last is initials)
	result := NormalizeAuthorName("John Michael Doe J.")
	if result != "John Michael Doe, J." {
		t.Errorf("expected 'John Michael Doe, J.', got %q", result)
	}

	// Test 3+ given words without initials
	result = NormalizeAuthorName("University of Somewhere")
	if result == "" {
		t.Error("expected non-empty for 'University of Somewhere'")
	}
}

// =========================================================================
// SplitFirstLast tests
// =========================================================================

// TestSplitFirstLast verifies split first last.
func TestSplitFirstLast(t *testing.T) {
	first, last := SplitFirstLast("Maarouk, T. M.")
	if first != "T. M." || last != "Maarouk" {
		t.Errorf("expected (T. M., Maarouk), got (%q, %q)", first, last)
	}

	first, last = SplitFirstLast("SingleName")
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
	result := NormalizeAffiliation("Univ. of California")
	if !contains(result, "University") {
		t.Errorf("expected 'University' in %q", result)
	}

	// Title casing
	result = NormalizeAffiliation("federal university of rio de janeiro")
	if result != "Federal University of Rio de Janeiro" {
		t.Errorf("expected 'Federal University of Rio de Janeiro', got %q", result)
	}

	// Strip trailing period
	result = NormalizeAffiliation("Stanford University.")
	if result != "Stanford University" {
		t.Errorf("expected 'Stanford University', got %q", result)
	}

	// Empty
	if NormalizeAffiliation("") != "" {
		t.Error("expected empty for empty string")
	}

	// Roman numerals
	result = NormalizeAffiliation("University of California, San Francisco, campus box Iii")
	if !contains(result, "III") {
		t.Errorf("expected 'III' in %q", result)
	}

	// Acronym-like words
	result = NormalizeAffiliation("Sao Paulo")
	if !contains(result, "Sao") {
		t.Errorf("expected 'Sao' in %q", result)
	}

	// Hyphenated
	result = NormalizeAffiliation("state-of-the-art lab")
	if !contains(result, "State-of-the-art Lab") {
		t.Errorf("expected 'State-of-the-art Lab' in %q", result)
	}

	// Various abbreviations
	result = NormalizeAffiliation("dept. of comp. sci., univ. of oxford")
	if !contains(result, "Department") {
		t.Errorf("expected 'Department' in %q", result)
	}
	if !contains(result, "University") {
		t.Errorf("expected 'University' in %q", result)
	}

	// Dash-separated segments
	result = NormalizeAffiliation("Main Campus — Secondary Site")
	if !contains(result, "Main") {
		t.Errorf("expected 'Main' in %q", result)
	}

	result = NormalizeAffiliation("école supérieure de paris")
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
	result := NormalizePublisher("Institute of Electrical and Electronics Engineers (IEEE)")
	if result != "IEEE" {
		t.Errorf("expected 'IEEE', got %q", result)
	}

	// HTML unescape
	result = NormalizePublisher("ACM &amp; IEEE")
	if !contains(result, "&") {
		t.Errorf("expected '&' in %q", result)
	}

	// Title casing
	result = NormalizePublisher("springer nature")
	if result == "" || result[0] < 'A' || result[0] > 'Z' {
		t.Errorf("expected title-cased, got %q", result)
	}

	// Legal suffix preserved
	result = NormalizePublisher("acme corp.")
	if !contains(result, "Corp.") {
		t.Errorf("expected 'Corp.' in %q", result)
	}

	// Dot stripped non-legal
	result = NormalizePublisher("Random Publisher.")
	if result != "" && result[len(result)-1] == '.' {
		t.Errorf("expected no trailing dot, got %q", result)
	}

	// S.r.o. suffix
	result = NormalizePublisher("Some Company s.r.o.")
	if !contains(result, "s.r.o.") {
		t.Errorf("expected 's.r.o.' in %q", result)
	}

	result = NormalizePublisher("éditions universitaires")
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
	result := NormalizeJournal("INFORMATION SYSTEMS")
	if result != "Information Systems" {
		t.Errorf("expected 'Information Systems', got %q", result)
	}

	// Subtitle stripping
	result = NormalizeJournal("Future Generation Computer Systems-The International Journal of eScience")
	if result != "Future Generation Computer Systems" {
		t.Errorf("expected 'Future Generation Computer Systems', got %q", result)
	}

	// Acronym in name
	result = NormalizeJournal("ieee transactions on software engineering")
	if !hasPrefix(result, "IEEE") {
		t.Errorf("expected 'IEEE...', got %q", result)
	}

	// Parenthesized content
	result = NormalizeJournal("Journal of Research (Science)")
	if !contains(result, "(Science)") {
		t.Errorf("expected '(Science)' in %q", result)
	}

	result = NormalizeJournal("journal of research (science (technical))")
	if result != "Journal of Research (Science (Technical))" {
		t.Errorf("nested parentheses = %q", result)
	}

	result = NormalizeJournal("journal of research (science")
	if result != "Journal of Research (Science" {
		t.Errorf("unmatched opening parenthesis = %q", result)
	}

	result = NormalizeJournal("journal of research science)")
	if result != "Journal of Research Science)" {
		t.Errorf("unmatched closing parenthesis = %q", result)
	}

	// Ampersand
	result = NormalizeJournal("research & development journal")
	if !contains(result, "&") {
		t.Errorf("expected '&' in %q", result)
	}

	// Hyphenated words
	result = NormalizeJournal("journal of software-evolution and process")
	if !contains(result, "Software-Evolution") && !contains(result, "Software-evolution") {
		t.Errorf("expected 'Software-Evolution' variant in %q", result)
	}

	result = NormalizeJournal("revista de administração")
	if result != "Revista de Administração" {
		t.Errorf("expected Unicode title casing, got %q", result)
	}
}

// hasPrefix supports the package test suite's has prefix setup or assertions.
func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

// =========================================================================
// firstRune tests
// =========================================================================

// TestFirstRune verifies first rune.
func TestFirstRune(t *testing.T) {
	if firstRune("") != "" {
		t.Errorf("expected '', got %q", firstRune(""))
	}
	if firstRune("abc") != "a" {
		t.Errorf("expected 'a', got %q", firstRune("abc"))
	}
	if firstRune("Äbc") != "Ä" {
		t.Errorf("expected 'Ä', got %q", firstRune("Äbc"))
	}
	if firstRune("a") != "a" {
		t.Errorf("expected 'a', got %q", firstRune("a"))
	}
}

// =========================================================================
// firstLetterIsNonASCII tests
// =========================================================================

// TestFirstLetterIsNonASCII verifies first letter is non ascii.
func TestFirstLetterIsNonASCII(t *testing.T) {
	if firstLetterIsNonASCII("") {
		t.Error("expected false for empty string")
	}
	if firstLetterIsNonASCII("Hello") {
		t.Error("expected false for ASCII start")
	}
	if !firstLetterIsNonASCII("École") {
		t.Error("expected true for Unicode start")
	}
	if firstLetterIsNonASCII("123abc") {
		t.Error("expected false for digit start")
	}
	if !firstLetterIsNonASCII("über") {
		t.Error("expected true for ü start")
	}
}

// =========================================================================
// titleFirstRune tests
// =========================================================================

// TestTitleFirstRune verifies title first rune.
func TestTitleFirstRune(t *testing.T) {
	if titleFirstRune("") != "" {
		t.Errorf("expected '', got %q", titleFirstRune(""))
	}
	if titleFirstRune("hello") != "Hello" {
		t.Errorf("expected 'Hello', got %q", titleFirstRune("hello"))
	}
	if titleFirstRune("Hello") != "Hello" {
		t.Errorf("expected 'Hello', got %q", titleFirstRune("Hello"))
	}
	if titleFirstRune("h") != "H" {
		t.Errorf("expected 'H', got %q", titleFirstRune("h"))
	}
	if titleFirstRune("é") != "É" {
		t.Errorf("expected 'É', got %q", titleFirstRune("é"))
	}
}

// =========================================================================
// isUpperAlpha tests
// =========================================================================

// TestIsUpperAlpha verifies is upper alpha.
func TestIsUpperAlpha(t *testing.T) {
	if !isUpperAlpha("IEEE") {
		t.Error("expected true for 'IEEE'")
	}
	if isUpperAlpha("ieee") {
		t.Error("expected false for 'ieee'")
	}
	if isUpperAlpha("Abc") {
		t.Error("expected false for 'Abc'")
	}
	if isUpperAlpha("123") {
		t.Error("expected false for digits")
	}
	if isUpperAlpha("") {
		t.Error("expected false for empty")
	}
	if !isUpperAlpha("A") {
		t.Error("expected true for 'A'")
	}
	if isUpperAlpha("A B") {
		t.Error("expected false for 'A B' (space)")
	}
}

// =========================================================================
// expandAbbreviations tests
// =========================================================================

// TestExpandAbbreviations verifies expand abbreviations.
func TestExpandAbbreviations(t *testing.T) {
	if expandAbbreviations("Univ. of Oxford") != "University of Oxford" {
		t.Errorf("expected 'University of Oxford', got %q", expandAbbreviations("Univ. of Oxford"))
	}
	if expandAbbreviations("Dept. of CS") != "Department of CS" {
		t.Errorf("expected 'Department of CS', got %q", expandAbbreviations("Dept. of CS"))
	}
	if expandAbbreviations("Acme Corp.") != "Acme Corporation" {
		t.Errorf("expected 'Acme Corporation', got %q", expandAbbreviations("Acme Corp."))
	}
	if expandAbbreviations("") != "" {
		t.Errorf("expected '', got %q", expandAbbreviations(""))
	}
	if expandAbbreviations("NoAbbreviation") != "NoAbbreviation" {
		t.Errorf("expected 'NoAbbreviation', got %q", expandAbbreviations("NoAbbreviation"))
	}
}

// =========================================================================
// affTitleWord tests
// =========================================================================

// TestAffTitleWord verifies aff title word.
func TestAffTitleWord(t *testing.T) {
	if affTitleWord("", true) != "" {
		t.Errorf("expected '', got %q", affTitleWord("", true))
	}
	if affTitleWord("univ.", true) != "Univ." {
		t.Errorf("expected 'Univ.', got %q", affTitleWord("univ.", true))
	}
	if affTitleWord("of", false) != "of" {
		t.Errorf("expected 'of', got %q", affTitleWord("of", false))
	}
	if affTitleWord("Of", true) != "Of" {
		t.Errorf("expected 'Of', got %q", affTitleWord("Of", true))
	}
	if affTitleWord("iii", false) != "III" {
		t.Errorf("expected 'III', got %q", affTitleWord("iii", false))
	}
	if affTitleWord("federal", true) != "Federal" {
		t.Errorf("expected 'Federal', got %q", affTitleWord("federal", true))
	}
	if affTitleWord("de", true) != "De" {
		t.Errorf("expected 'De', got %q", affTitleWord("de", true))
	}
	if affTitleWord("state-of-the-art", true) != "State-of-the-art" {
		t.Errorf("expected 'State-of-the-art', got %q", affTitleWord("state-of-the-art", true))
	}
	if affTitleWord("l'école", true) != "L'école" {
		t.Errorf("expected 'L'école', got %q", affTitleWord("l'école", true))
	}
}

// =========================================================================
// normalizeAffiliationCase tests
// =========================================================================

// TestNormalizeAffiliationCase verifies normalize affiliation case.
func TestNormalizeAffiliationCase(t *testing.T) {
	if normalizeAffiliationCase("") != "" {
		t.Errorf("expected '', got %q", normalizeAffiliationCase(""))
	}
	result := normalizeAffiliationCase("federal university of rio de janeiro")
	if result != "Federal University of Rio de Janeiro" {
		t.Errorf("expected 'Federal University of Rio de Janeiro', got %q", result)
	}
	result = normalizeAffiliationCase("main campus — secondary site")
	if result != "Main Campus — Secondary Site" {
		t.Errorf("expected 'Main Campus — Secondary Site', got %q", result)
	}
}

// =========================================================================
// isAcronym tests
// =========================================================================

// TestIsAcronym verifies is acronym.
func TestIsAcronym(t *testing.T) {
	if !isAcronym("IEEE") {
		t.Error("expected true for 'IEEE'")
	}
	if !isAcronym("ABC") {
		t.Error("expected true for 'ABC'")
	}
	if isAcronym("Word") {
		t.Error("expected false for 'Word'")
	}
	if isAcronym("") {
		t.Error("expected false for empty")
	}
	if isAcronym("A") {
		t.Error("expected false for single letter")
	}
	if !isAcronym("ABCDEFG") {
		t.Error("expected true for 'ABCDEFG'")
	}
	if isAcronym("ABCDEFGH") {
		t.Error("expected false for 8 letters")
	}
	if !isAcronym("IEEE.") {
		t.Error("expected true for 'IEEE.' (trim dot)")
	}
}

// =========================================================================
// pubTitleCore tests
// =========================================================================

// TestPubTitleCore verifies pub title core.
func TestPubTitleCore(t *testing.T) {
	if pubTitleCore("") != "" {
		t.Errorf("expected '', got %q", pubTitleCore(""))
	}
	if pubTitleCore("of") != "of" {
		t.Errorf("expected 'of', got %q", pubTitleCore("of"))
	}
	if pubTitleCore("the") != "the" {
		t.Errorf("expected 'the', got %q", pubTitleCore("the"))
	}
	if pubTitleCore("ltd") != "Ltd" {
		t.Errorf("expected 'Ltd', got %q", pubTitleCore("ltd"))
	}
	if pubTitleCore("inc") != "Inc." {
		t.Errorf("expected 'Inc.', got %q", pubTitleCore("inc"))
	}
	if pubTitleCore("IEEE") != "IEEE" {
		t.Errorf("expected 'IEEE', got %q", pubTitleCore("IEEE"))
	}
	if pubTitleCore("springer") != "Springer" {
		t.Errorf("expected 'Springer', got %q", pubTitleCore("springer"))
	}
	if pubTitleCore("non-standard") != "Non-Standard" {
		t.Errorf("expected 'Non-Standard', got %q", pubTitleCore("non-standard"))
	}
}

// =========================================================================
// pubTitleWord tests
// =========================================================================

// TestPubTitleWord verifies pub title word.
func TestPubTitleWord(t *testing.T) {
	if pubTitleWord("") != "" {
		t.Errorf("expected '', got %q", pubTitleWord(""))
	}
	if pubTitleWord("  ") != "  " {
		t.Errorf("expected '  ', got %q", pubTitleWord("  "))
	}
	if pubTitleWord("the") != "the" {
		t.Errorf("expected 'the', got %q", pubTitleWord("the"))
	}
	if pubTitleWord("and") != "and" {
		t.Errorf("expected 'and', got %q", pubTitleWord("and"))
	}
	if pubTitleWord("ltd") != "Ltd" {
		t.Errorf("expected 'Ltd', got %q", pubTitleWord("ltd"))
	}
	if pubTitleWord("IEEE") != "IEEE" {
		t.Errorf("expected 'IEEE', got %q", pubTitleWord("IEEE"))
	}
	if pubTitleWord("Springer") != "Springer" {
		t.Errorf("expected 'Springer', got %q", pubTitleWord("Springer"))
	}
	if pubTitleWord("Éditions") != "Éditions" {
		t.Errorf("expected 'Éditions', got %q", pubTitleWord("Éditions"))
	}
}

// =========================================================================
// detachLegalSuffix tests
// =========================================================================

// TestDetachLegalSuffix verifies detach legal suffix.
func TestDetachLegalSuffix(t *testing.T) {
	prefix, suffix := detachLegalSuffix("")
	if prefix != "" || suffix != "" {
		t.Errorf("expected ('',''), got (%q,%q)", prefix, suffix)
	}
	// detachLegalSuffix only matches sp. z o.o., s.r.o., kft. — not Corp./Ltd
	prefix, suffix = detachLegalSuffix("Acme Corp.")
	if prefix != "Acme Corp." || suffix != "" {
		t.Errorf("expected ('Acme Corp.',''), got (%q,%q)", prefix, suffix)
	}
	prefix, suffix = detachLegalSuffix("IEEE")
	if prefix != "IEEE" || suffix != "" {
		t.Errorf("expected ('IEEE',''), got (%q,%q)", prefix, suffix)
	}
	prefix, suffix = detachLegalSuffix("Company Ltd")
	if prefix != "Company Ltd" || suffix != "" {
		t.Errorf("expected ('Company Ltd',''), got (%q,%q)", prefix, suffix)
	}
	prefix, suffix = detachLegalSuffix("Some Company s.r.o.")
	if prefix != "Some Company" || suffix != "s.r.o." {
		t.Errorf("expected ('Some Company','s.r.o.'), got (%q,%q)", prefix, suffix)
	}
	prefix, suffix = detachLegalSuffix("Sp. z o.o.")
	if prefix != "" || suffix != "Sp. z o.o." {
		t.Errorf("expected ('','Sp. z o.o.'), got (%q,%q)", prefix, suffix)
	}
}

// =========================================================================
// titleCasePublisher tests
// =========================================================================

// TestTitleCasePublisher verifies title case publisher.
func TestTitleCasePublisher(t *testing.T) {
	result := titleCasePublisher("springer nature")
	if result != "Springer Nature" {
		t.Errorf("expected 'Springer Nature', got %q", result)
	}
	result = titleCasePublisher("acme corp.")
	if result != "Acme Corp." {
		t.Errorf("expected 'Acme Corp.', got %q", result)
	}
	result = titleCasePublisher("IEEE")
	if result != "IEEE" {
		t.Errorf("expected 'IEEE', got %q", result)
	}
	result = titleCasePublisher("acme (UK)")
	if result != "Acme (UK)" {
		t.Errorf("expected 'Acme (UK)', got %q", result)
	}
	result = titleCasePublisher("of the world")
	if result != "of the World" {
		t.Errorf("expected 'of the World', got %q", result)
	}
	result = titleCasePublisher("")
	if result != "" {
		t.Errorf("expected '', got %q", result)
	}
}

// =========================================================================
// journalIsAcronym tests
// =========================================================================

// TestJournalIsAcronym verifies journal is acronym.
func TestJournalIsAcronym(t *testing.T) {
	if !journalIsAcronym("IEEE") {
		t.Error("expected true for 'IEEE'")
	}
	if !journalIsAcronym("ACM") {
		t.Error("expected true for 'ACM'")
	}
	if journalIsAcronym("Nature") {
		t.Error("expected false for 'Nature'")
	}
	if journalIsAcronym("") {
		t.Error("expected false for empty")
	}
	if !journalIsAcronym("ieee") {
		t.Error("expected true for lowercase 'ieee'")
	}
	if !journalIsAcronym("ACM.") {
		t.Error("expected true for 'ACM.'")
	}
}

// =========================================================================
// stripJournalSubtitle tests
// =========================================================================

// TestStripJournalSubtitle verifies strip journal subtitle.
func TestStripJournalSubtitle(t *testing.T) {
	result := stripJournalSubtitle("Future Generation Computer Systems-The International Journal of eScience")
	if result != "Future Generation Computer Systems" {
		t.Errorf("expected 'Future Generation Computer Systems', got %q", result)
	}
	result = stripJournalSubtitle("IEEE Access")
	if result != "IEEE Access" {
		t.Errorf("expected 'IEEE Access', got %q", result)
	}
	result = stripJournalSubtitle("")
	if result != "" {
		t.Errorf("expected '', got %q", result)
	}
	result = stripJournalSubtitle("Journal of Research — A Subtitle")
	if result != "Journal of Research" {
		t.Errorf("expected 'Journal of Research', got %q", result)
	}
	result = stripJournalSubtitle("Software and Systems Modeling")
	if result != "Software and Systems Modeling" {
		t.Errorf("expected 'Software and Systems Modeling', got %q", result)
	}
}

// =========================================================================
// titleCaseJournal tests
// =========================================================================

// TestTitleCaseJournal verifies title case journal.
func TestTitleCaseJournal(t *testing.T) {
	if titleCaseJournal("") != "" {
		t.Errorf("expected '', got %q", titleCaseJournal(""))
	}
	if titleCaseJournal("  ") != "  " {
		t.Errorf("expected '  ', got %q", titleCaseJournal("  "))
	}
	result := titleCaseJournal("information systems")
	if result != "Information Systems" {
		t.Errorf("expected 'Information Systems', got %q", result)
	}
	result = titleCaseJournal("ieee transactions on software engineering")
	if result != "IEEE Transactions on Software Engineering" {
		t.Errorf("expected 'IEEE Transactions on Software Engineering', got %q", result)
	}
	result = titleCaseJournal("journal of the acm")
	if result != "Journal of the ACM" {
		t.Errorf("expected 'Journal of the ACM', got %q", result)
	}
	result = titleCaseJournal("journal of research (science)")
	if result != "Journal of Research (Science)" {
		t.Errorf("expected 'Journal of Research (Science)', got %q", result)
	}
	result = titleCaseJournal("software and systems modeling")
	if result != "Software and Systems Modeling" {
		t.Errorf("expected 'Software and Systems Modeling', got %q", result)
	}
	result = titleCaseJournal("journal of software-evolution and process")
	if result != "Journal of Software-Evolution and Process" {
		t.Errorf("expected 'Journal of Software-Evolution and Process', got %q", result)
	}
}
