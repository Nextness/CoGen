// Package normalization provides author name, affiliation, publisher, and
// journal normalization for the corpus pipeline. All normalizers work on
// individual values and return normalized strings.
package normalization

import (
	"html"
	"regexp"
	"strings"
	"unicode"
)

// =========================================================================
// Shared helpers
// =========================================================================

var (
	wordSplitRE           = regexp.MustCompile(`[ ]+`)
	whitespaceRE          = regexp.MustCompile(`\s+`)
	asciiLetterRE         = regexp.MustCompile(`[A-Za-z]`)
	surroundedASCIIWordRE = regexp.MustCompile(`^([^a-zA-Z]*)([a-zA-Z].*[a-zA-Z]|[a-zA-Z])([^a-zA-Z]*)$`)
)

// titleFirstRune lowercases a string and uppercases its first Unicode rune.
func titleFirstRune(value string) string {
	runes := []rune(strings.ToLower(value))
	if len(runes) == 0 {
		return value
	}
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

// firstRune returns the first Unicode rune as a string, or an empty string for empty input.
func firstRune(value string) string {
	for _, r := range value {
		return string(r)
	}
	return ""
}

// firstLetterIsNonASCII reports whether the first Unicode letter is outside ASCII.
func firstLetterIsNonASCII(value string) bool {
	for _, r := range value {
		if unicode.IsLetter(r) {
			return r > unicode.MaxASCII
		}
	}
	return false
}

// =========================================================================
// Author name normalization
// =========================================================================

var lowerPrefixes = map[string]bool{
	"van": true, "von": true, "der": true, "den": true, "de": true,
	"ter": true, "ten": true, "op": true, "af": true, "da": true,
	"do": true, "das": true, "dos": true, "del": true, "della": true,
	"dei": true, "d´": true, "d'": true, "o`": true, "mac": true, "mc": true,
}

var initialsWordRE = regexp.MustCompile(`^[A-Z](?:\.[A-Z])*\.?$`)
var initialsCapsRE = regexp.MustCompile(`^([A-Z]{2,4})\.$`)

// isInitials reports whether a word is a supported compact or punctuated initial sequence.
func isInitials(word string) bool {
	w := strings.TrimSpace(word)
	if w == "" {
		return false
	}
	if initialsWordRE.MatchString(w) {
		return true
	}
	if initialsCapsRE.MatchString(w) {
		return true
	}
	if isUpperAlpha(w) && len(w) >= 1 && len(w) <= 3 && !lowerPrefixes[strings.ToLower(w)] {
		return true
	}
	return false
}

// isUpperAlpha reports whether a non-empty string contains only ASCII uppercase letters.
func isUpperAlpha(s string) bool {
	for _, r := range s {
		if r < 'A' || r > 'Z' {
			return false
		}
	}
	return len(s) > 0
}

// smartTitle applies author-name casing rules to one word while preserving particles and compounds.
func smartTitle(word string) string {
	lower := strings.ToLower(word)
	if lowerPrefixes[lower] {
		return lower
	}
	if len(word) > 1 && strings.ToLower(word[:2]) == "mc" {
		return "Mc" + strings.Title(strings.ToLower(word[2:]))
	}
	if len(word) > 3 && strings.ToLower(word[:3]) == "mac" {
		return "Mac" + strings.Title(strings.ToLower(word[3:]))
	}
	if len(word) > 2 && (strings.ToLower(word[:2]) == "o'" || strings.ToLower(word[:2]) == "d'") {
		return strings.ToUpper(word[:2]) + strings.Title(strings.ToLower(word[2:]))
	}
	if strings.Contains(word, "-") {
		parts := strings.Split(word, "-")
		for i, p := range parts {
			if p != "" {
				parts[i] = titleFirstRune(p)
			}
		}
		return strings.Join(parts, "-")
	}
	if word == "" {
		return word
	}
	return titleFirstRune(word)
}

// splitGivenFamily splits given family into its component values.
func splitGivenFamily(words []string) (given, family []string) {
	if len(words) == 0 {
		return nil, nil
	}
	n := len(words)
	prefixStart := -1
	for i := n - 1; i >= 0; i-- {
		if lowerPrefixes[strings.ToLower(words[i])] {
			prefixStart = i
		} else if prefixStart >= 0 {
			break
		}
	}
	if prefixStart >= 0 {
		return words[:prefixStart], words[prefixStart:]
	}
	return words[:n-1], words[n-1:]
}

// wordToInitial converts a given-name word or existing initial sequence to dotted uppercase initials.
func wordToInitial(word string) string {
	if word == "" {
		return ""
	}
	if isInitials(word) {
		letters := asciiLetterRE.FindAllString(word, -1)
		parts := make([]string, len(letters))
		for i, l := range letters {
			parts[i] = strings.ToUpper(l)
		}
		return strings.Join(parts, ". ") + "."
	}
	clean := strings.TrimRight(word, ".")
	if clean == "" {
		return "."
	}
	return strings.ToUpper(firstRune(clean)) + "."
}

// toInitials converts whitespace-separated given names to canonical dotted initials.
func toInitials(givenStr string) string {
	if givenStr == "" || strings.TrimSpace(givenStr) == "" {
		return ""
	}
	words := wordSplitRE.Split(strings.TrimSpace(givenStr), -1)
	parts := make([]string, 0, len(words))
	for _, w := range words {
		if w != "" {
			parts = append(parts, wordToInitial(w))
		}
	}
	return strings.Join(parts, " ")
}

// normalizeFamilyWords normalizes family words.
func normalizeFamilyWords(words []string) []string {
	result := make([]string, 0, len(words))
	for _, w := range words {
		if w == "" {
			continue
		}
		if lowerPrefixes[strings.ToLower(w)] {
			result = append(result, strings.ToLower(w))
		} else if isInitials(w) {
			letters := asciiLetterRE.FindAllString(w, -1)
			parts := make([]string, len(letters))
			for i, l := range letters {
				parts[i] = strings.ToUpper(l)
			}
			result = append(result, strings.Join(parts, ". ")+".")
		} else if cleaned := strings.ReplaceAll(w, ".", ""); cleaned != "" && isInitials(cleaned) {
			parts := make([]string, 0, len(cleaned))
			for _, r := range cleaned {
				parts = append(parts, strings.ToUpper(string(r)))
			}
			result = append(result, strings.Join(parts, ". ")+".")
		} else {
			result = append(result, smartTitle(w))
		}
	}
	return result
}

var fusedInitialRE = regexp.MustCompile(`(\b[A-Z]\.)([A-Z][a-z])`)

// NormalizeAuthorName normalizes a single author name to 'Lastname, F. M.' format.
func NormalizeAuthorName(name string) string {
	if name == "" || strings.TrimSpace(name) == "" {
		return ""
	}
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, "\u00a0", " ")
	name = wordSplitRE.ReplaceAllString(name, " ")
	name = fusedInitialRE.ReplaceAllString(name, "$1 $2")

	parsed := splitOnComma(name)
	var familyWords, givenWords []string

	if parsed != nil {
		familyStr, givenStr := parsed[0], parsed[1]
		familyWords = wordSplitRE.Split(familyStr, -1)
		givenWords = wordSplitRE.Split(givenStr, -1)
	} else {
		words := wordSplitRE.Split(name, -1)
		if len(words) == 0 {
			return ""
		} else if len(words) == 1 {
			familyWords = words
			givenWords = nil
		} else if len(words) == 2 && !isInitials(words[0]) && len(words[0]) > 1 && isInitials(words[1]) {
			familyWords = []string{words[0]}
			givenWords = []string{words[1]}
		} else if len(words) >= 3 && (isInitials(words[len(words)-1]) ||
			(isUpperAlpha(words[len(words)-1]) && len(words[len(words)-1]) <= 4)) {
			familyWords = words[:len(words)-1]
			givenWords = []string{words[len(words)-1]}
		} else {
			givenWords, familyWords = splitGivenFamily(words)
			if len(givenWords) >= 3 {
				hasInitials := false
				for _, w := range givenWords {
					if isInitials(w) {
						hasInitials = true
						break
					}
				}
				if !hasInitials {
					familyWords = append(givenWords, familyWords...)
					givenWords = nil
				}
			}
		}
	}

	givenNormalized := ""
	if len(givenWords) > 0 {
		givenNormalized = toInitials(strings.Join(givenWords, " "))
	}
	familyNormalized := strings.Join(normalizeFamilyWords(familyWords), " ")

	if givenNormalized != "" {
		return familyNormalized + ", " + givenNormalized
	}
	return familyNormalized
}

// splitOnComma splits on comma into its component values.
func splitOnComma(name string) []string {
	if !strings.Contains(name, ",") {
		return nil
	}
	parts := strings.SplitN(name, ",", 2)
	if len(parts) != 2 {
		return nil
	}
	before := strings.TrimSpace(parts[0])
	after := strings.TrimSpace(parts[1])
	if before == "" || after == "" {
		return nil
	}
	return []string{before, after}
}

// SplitFirstLast splits 'Lastname, F. M.' into (first_name, last_name).
func SplitFirstLast(normalized string) (first, last string) {
	if !strings.Contains(normalized, ",") {
		return "", normalized
	}
	parts := strings.SplitN(normalized, ",", 2)
	return strings.TrimSpace(parts[1]), strings.TrimSpace(parts[0])
}

// =========================================================================
// Affiliation normalization
// =========================================================================

var affLowerWords = map[string]bool{
	"de": true, "di": true, "da": true, "do": true, "das": true, "dos": true,
	"del": true, "della": true, "dei": true, "van": true, "von": true,
	"der": true, "den": true, "ter": true, "ten": true, "op": true, "af": true,
	"the": true, "a": true, "an": true, "in": true, "at": true, "of": true,
	"for": true, "to": true, "by": true, "on": true, "with": true, "und": true,
	"and": true, "or": true, "et": true, "en": true, "el": true, "la": true,
	"le": true, "les": true, "des": true, "du": true, "au": true, "aux": true,
	"d'": true, "l'": true, "am": true, "zum": true, "zur": true,
}

var affRoman = map[string]bool{
	"i": true, "ii": true, "iii": true, "iv": true, "v": true,
	"vi": true, "vii": true, "viii": true, "ix": true, "x": true,
}

var affAcronymRE = regexp.MustCompile(`^[A-Z]{2,5}$`)

var affAcronymLike = map[string]bool{
	"rio": true, "sao": true,
}

var affAbbreviations = map[string]string{
	"univ.": "University", "inst.": "Institute", "dept.": "Department",
	"sch.": "School", "coll.": "College", "lab.": "Laboratory",
	"co.": "Company", "inc.": "Inc.", "ltd.": "Ltd.", "gmbh": "GmbH",
	"s.a.": "S.A.", "s.l.": "S.L.", "s.p.a.": "S.p.A.", "b.v.": "B.V.",
	"corp.": "Corporation", "assoc.": "Association", "acad.": "Academy",
	"st.": "St.", "mt.": "Mt.", "dr.": "Dr.", "prof.": "Prof.",
}

var affAbbreviationRE = func() map[string]*regexp.Regexp {
	patterns := make(map[string]*regexp.Regexp, len(affAbbreviations))
	for abbreviation := range affAbbreviations {
		patterns[abbreviation] = regexp.MustCompile(`(?i)` + regexp.QuoteMeta(abbreviation))
	}
	return patterns
}()

// expandAbbreviations expands recognized affiliation abbreviations case-insensitively.
func expandAbbreviations(text string) string {
	result := text
	for abbr, full := range affAbbreviations {
		result = affAbbreviationRE[abbr].ReplaceAllString(result, full)
	}
	return result
}

// affTitleWord applies affiliation-specific casing while preserving punctuation and acronyms.
func affTitleWord(word string, firstWord bool) string {
	if word == "" {
		return word
	}
	stripped := strings.Trim(word, ".,;:()[]\"'")
	prefix := word[:len(word)-len(strings.TrimLeft(word, ".,;:()[]\"'"))]
	suffix := ""
	if stripped != "" {
		suffix = word[len(prefix)+len(stripped):]
	} else {
		return word
	}
	lower := strings.ToLower(stripped)

	if affRoman[lower] {
		return prefix + strings.ToUpper(stripped) + suffix
	}
	if !firstWord && affLowerWords[lower] {
		return prefix + lower + suffix
	}
	if affAcronymRE.MatchString(stripped) && !affLowerWords[lower] {
		if affAcronymLike[lower] {
			return prefix + string(stripped[0]) + strings.ToLower(stripped[1:]) + suffix
		}
		return word
	}
	if firstWord {
		return prefix + titleFirstRune(stripped) + suffix
	}
	if len(stripped) > 1 && (stripped[0] == 'd' || stripped[0] == 'D' || stripped[0] == 'l' || stripped[0] == 'L' || stripped[0] == 'o' || stripped[0] == 'O') && stripped[1] == '\'' {
		return prefix + strings.ToUpper(string(stripped[0])) + strings.ToLower(stripped[1:]) + suffix
	}
	if strings.Contains(stripped, "-") {
		parts := strings.Split(stripped, "-")
		for i, p := range parts {
			if p != "" {
				parts[i] = titleFirstRune(p)
			}
		}
		return prefix + strings.Join(parts, "-") + suffix
	}
	return prefix + titleFirstRune(stripped) + suffix
}

var dashSegmentRE = regexp.MustCompile(`(\s*[—–]\s*)`)

// normalizeAffiliationCase normalizes affiliation case.
func normalizeAffiliationCase(text string) string {
	segments := dashSegmentRE.Split(text, -1)
	dashes := dashSegmentRE.FindAllString(text, -1)

	var cased []string
	segIdx := 0
	for i, seg := range segments {
		if i > 0 && segIdx < len(dashes) {
			cased = append(cased, dashes[segIdx])
			segIdx++
		}
		words := wordSplitRE.Split(seg, -1)
		if len(words) == 0 || (len(words) == 1 && words[0] == "") {
			cased = append(cased, seg)
			continue
		}
		normalized := make([]string, 0, len(words))
		insideParens := false
		for i, w := range words {
			if w == "" {
				continue
			}
			if strings.Contains(w, "(") {
				insideParens = true
			}
			isFirst := (i == 0 && len(cased) == 0) || (insideParens && strings.HasPrefix(w, "("))
			normalized = append(normalized, affTitleWord(w, isFirst))
			if strings.Contains(w, ")") {
				insideParens = false
			}
		}
		cased = append(cased, strings.Join(normalized, " "))
	}
	return strings.Join(cased, "")
}

// NormalizeAffiliation normalizes an affiliation string.
func NormalizeAffiliation(aff string) string {
	if aff == "" || strings.TrimSpace(aff) == "" {
		return ""
	}
	aff = strings.TrimSpace(aff)
	aff = strings.ReplaceAll(aff, "\u00a0", " ")
	aff = expandAbbreviations(aff)
	aff = wordSplitRE.ReplaceAllString(aff, " ")
	aff = normalizeAffiliationCase(aff)
	aff = strings.TrimRight(aff, ".")
	return strings.TrimSpace(aff)
}

// =========================================================================
// Publisher normalization
// =========================================================================

var publisherCanonical = map[string]string{
	"World Scientific Pub Co Pte Lt":                                             "World Scientific",
	"World Scientific Pub Co Pte Ltd":                                            "World Scientific",
	"Institute of Electrical and Electronics Engineers (IEEE)":                   "IEEE",
	"Institution of Engineering and Technology (IET)":                            "IET",
	"Association for Computing Machinery (ACM)":                                  "ACM",
	"Korean Society for Internet Information (KSII)":                             "KSII",
	"Institute of Electronics, Information and Communications Engineers (IEICE)": "IEICE",
	"International Association of Online Engineering (IAOE)":                     "IAOE",
	"Frontiers Media SA":                                                         "Frontiers",
	"MDPI AG":                                                                    "MDPI",
}

var pubLowerWords = map[string]bool{
	"of": true, "de": true, "di": true, "van": true, "der": true,
	"den": true, "ter": true, "ten": true, "et": true, "la": true,
	"le": true, "les": true, "del": true, "della": true, "das": true,
	"dos": true, "da": true, "do": true, "in": true, "for": true,
	"and": true, "the": true, "a": true, "an": true, "per": true,
	"con": true, "sul": true, "sulla": true, "auf": true, "und": true,
	"mit": true, "von": true, "zum": true, "zur": true, "des": true,
	"il": true, "lo": true, "gli": true, "al": true, "alla": true,
	"degli": true, "delle": true, "nei": true, "nelle": true, "sui": true,
	"dans": true, "sur": true, "pour": true, "avec": true, "ces": true,
	"sans": true, "chez": true, "i": true, "y": true, "o": true, "u": true,
}

var pubSuffix = map[string]string{
	"llc": "LLC", "ltd": "Ltd", "inc": "Inc.", "corp": "Corp.",
	"bv": "BV", "ag": "AG", "sa": "SA", "gmbh": "GmbH", "plc": "PLC",
	"co": "Co.", "spa": "S.p.A.", "srl": "S.r.l.", "lp": "LP", "llp": "LLP",
}

var pubLegalSuffixRE = regexp.MustCompile(`(?i)(sp\.\s*z\s*o\.\s*o\.?|s\.r\.o\.?|kft\.?)$`)

// isAcronym reports whether a word is a short all-uppercase publisher acronym.
func isAcronym(word string) bool {
	w := strings.TrimRight(strings.TrimSpace(word), ".")
	return len(w) >= 2 && len(w) <= 7 && isUpperAlpha(w)
}

// pubTitleCore applies publisher-specific casing to an unpunctuated word core.
func pubTitleCore(core string) string {
	lower := strings.ToLower(core)
	if pubLowerWords[lower] {
		return lower
	}
	if s, ok := pubSuffix[lower]; ok {
		return s
	}
	if isAcronym(core) {
		return core
	}
	if strings.Contains(core, "-") {
		parts := strings.Split(core, "-")
		for i, p := range parts {
			parts[i] = pubTitleCore(p)
		}
		return strings.Join(parts, "-")
	}
	if core == "" {
		return core
	}
	return titleFirstRune(core)
}

// pubTitleWord applies publisher casing while preserving surrounding punctuation.
func pubTitleWord(word string) string {
	if word == "" || strings.TrimSpace(word) == "" {
		return word
	}
	stripped := strings.TrimSpace(word)
	if firstLetterIsNonASCII(stripped) {
		return titleFirstRune(stripped)
	}
	m := surroundedASCIIWordRE.FindStringSubmatch(stripped)
	if m != nil {
		prefix, core, suffixExtra := m[1], m[2], m[3]
		cased := prefix + pubTitleCore(core) + suffixExtra
		if strings.HasSuffix(cased, "..") {
			cased = strings.TrimSuffix(cased, ".")
		}
		return cased
	}
	return titleFirstRune(stripped)
}

// detachLegalSuffix separates a recognized publisher legal suffix from the preceding name.
func detachLegalSuffix(text string) (string, string) {
	m := pubLegalSuffixRE.FindStringIndex(text)
	if m != nil {
		return strings.TrimSpace(text[:m[0]]), strings.TrimSpace(text[m[0]:])
	}
	return text, ""
}

// titleCasePublisher applies publisher-specific title casing across words and parenthesized text.
func titleCasePublisher(name string) string {
	prefix, legalSuffix := detachLegalSuffix(name)
	name = prefix

	type part struct {
		typ  string // "" or "paren"
		text string
	}
	var parts []part
	parenDepth := 0
	var current []byte
	for i := 0; i < len(name); i++ {
		ch := name[i]
		if ch == '(' && parenDepth == 0 {
			if len(current) > 0 {
				parts = append(parts, part{"", string(current)})
				current = nil
			}
			parenDepth = 1
		} else if ch == ')' && parenDepth == 1 {
			parenDepth = 0
			parts = append(parts, part{"paren", string(current)})
			current = nil
		} else {
			current = append(current, ch)
		}
	}
	if len(current) > 0 {
		parts = append(parts, part{"", string(current)})
	}

	var result strings.Builder
	for pi, p := range parts {
		words := strings.Fields(p.text)
		if len(words) == 0 {
			continue
		}
		casedWords := make([]string, len(words))
		for i, w := range words {
			if (i == 0 || i == len(words)-1) && p.typ != "paren" {
				if firstLetterIsNonASCII(w) {
					casedWords[i] = titleFirstRune(w)
					continue
				}
				m := surroundedASCIIWordRE.FindStringSubmatch(w)
				if m != nil {
					pre, core, suf := m[1], m[2], m[3]
					lower := strings.ToLower(core)
					if pubLowerWords[lower] {
						casedWords[i] = pre + lower + suf
					} else if s, ok := pubSuffix[lower]; ok {
						cased := pre + s + suf
						if strings.HasSuffix(cased, "..") {
							cased = strings.TrimSuffix(cased, ".")
						}
						casedWords[i] = cased
					} else if isAcronym(core) {
						casedWords[i] = pre + core + suf
					} else {
						cased := pre + titleFirstRune(core) + suf
						if strings.HasSuffix(cased, "..") {
							cased = strings.TrimSuffix(cased, ".")
						}
						casedWords[i] = cased
					}
				} else {
					casedWords[i] = pubTitleWord(w)
				}
			} else {
				casedWords[i] = pubTitleWord(w)
			}
		}
		partResult := strings.Join(casedWords, " ")
		if p.typ == "paren" {
			partResult = "(" + partResult + ")"
		}
		if pi > 0 && strings.HasPrefix(partResult, "(") && !strings.HasSuffix(result.String(), " ") {
			result.WriteString(" ")
		}
		result.WriteString(partResult)
	}

	if legalSuffix != "" {
		if result.Len() > 0 && !strings.HasSuffix(result.String(), " ") {
			result.WriteString(" ")
		}
		result.WriteString(legalSuffix)
	}
	return result.String()
}

// NormalizePublisher normalizes a publisher name.
func NormalizePublisher(publisher string) string {
	if publisher == "" || strings.TrimSpace(publisher) == "" {
		return ""
	}
	text := strings.TrimSpace(publisher)
	text = html.UnescapeString(text)
	text = whitespaceRE.ReplaceAllString(text, " ")
	text = strings.TrimSpace(text)

	if v, ok := publisherCanonical[text]; ok {
		return v
	}

	text = titleCasePublisher(text)

	if strings.HasSuffix(text, ".") &&
		!strings.HasSuffix(strings.ToLower(text), "ltd.") &&
		!strings.HasSuffix(strings.ToLower(text), "inc.") &&
		!strings.HasSuffix(strings.ToLower(text), "corp.") &&
		!strings.HasSuffix(strings.ToLower(text), "co.") &&
		!strings.HasSuffix(strings.ToLower(text), "llc.") &&
		!pubLegalSuffixRE.MatchString(text) {
		text = strings.TrimRight(text, ".")
	}

	return text
}

// =========================================================================
// Journal normalization
// =========================================================================

var journalAcronyms = map[string]bool{
	"IEEE": true, "ACM": true, "IET": true, "IEICE": true, "KSII": true,
	"IAOE": true, "AI": true, "IT": true, "ICT": true, "BPMN": true,
	"DMN": true, "SPEM": true, "BPM": true, "SN": true, "ARPN": true,
	"CAA": true,
}

var journalLowerWords = map[string]bool{
	"of": true, "and": true, "the": true, "for": true, "in": true,
	"on": true, "at": true, "to": true, "by": true, "with": true,
	"from": true, "an": true, "a": true, "is": true, "or": true,
	"as": true, "per": true, "via": true, "de": true, "di": true,
	"da": true, "do": true, "das": true, "dos": true, "del": true,
	"della": true, "dei": true, "van": true, "von": true, "der": true,
	"den": true, "ter": true, "ten": true, "et": true, "en": true,
	"el": true, "la": true, "le": true, "les": true, "des": true,
	"du": true, "au": true, "aux": true, "un": true, "une": true,
	"und": true, "mit": true, "fur": true, "uber": true, "aus": true,
	"bei": true, "nach": true, "um": true, "sobre": true, "para": true,
	"com": true, "nas": true, "nos": true, "sul": true, "sulla": true,
	"its": true, "their": true, "her": true, "his": true,
}

var journalCanonical = map[string]string{
	"INFORMATION SYSTEMS":                               "Information Systems",
	"BUSINESS PROCESS MANAGEMENT JOURNAL":               "Business Process Management Journal",
	"IEEE ACCESS":                                       "IEEE Access",
	"SOFTWARE AND SYSTEMS MODELING":                     "Software and Systems Modeling",
	"COMPOSITE STRUCTURES":                              "Composite Structures",
	"JOURNAL OF SOFTWARE-EVOLUTION AND PROCESS":         "Journal of Software: Evolution and Process",
	"CONCURRENCY AND COMPUTATION-PRACTICE & EXPERIENCE": "Concurrency and Computation: Practice and Experience",
	"FUTURE GENERATION COMPUTER SYSTEMS-THE INTERNATIONAL JOURNAL OF ESCIENCE":              "Future Generation Computer Systems",
	"ENTERPRISE MODELLING AND INFORMATION SYSTEMS ARCHITECTURES-AN   INTERNATIONAL JOURNAL": "Enterprise Modelling and Information Systems Architectures",
	"ACM TRANSACTIONS ON AUTONOMOUS AND ADAPTIVE SYSTEMS":                                   "ACM Transactions on Autonomous and Adaptive Systems",
	"INTERNATIONAL JOURNAL ON SOFTWARE TOOLS FOR TECHNOLOGY TRANSFER":                       "International Journal on Software Tools for Technology Transfer",
	"KNOWLEDGE AND PROCESS MANAGEMENT":                                                      "Knowledge and Process Management",
	"EGYPTIAN INFORMATICS JOURNAL":                                                          "Egyptian Informatics Journal",
	"INTERNATIONAL JOURNAL OF SOFTWARE ENGINEERING AND KNOWLEDGE ENGINEERING":               "International Journal of Software Engineering and Knowledge Engineering",
	"JOURNAL OF THE OPERATIONAL RESEARCH SOCIETY":                                           "Journal of the Operational Research Society",
	"Softwarex":                                        "SoftwareX",
	"Ieee/caa Journal of Automatica Sinica":            "IEEE/CAA Journal of Automatica Sinica",
	"Sn Computer Science":                              "SN Computer Science",
	"Arpn Journal of Engineering and Applied Sciences": "ARPN Journal of Engineering and Applied Sciences",
}

// journalIsAcronym reports whether a word is in the recognized journal-acronym set.
func journalIsAcronym(word string) bool {
	return journalAcronyms[strings.ToUpper(strings.TrimRight(word, "."))]
}

var stripSubtitleRE = regexp.MustCompile(`(?i)\s*[—–\-]\s*(the|an?)\s+.*$`)

// stripJournalSubtitle removes a dash-delimited generic article subtitle from a journal name.
func stripJournalSubtitle(name string) string {
	return strings.TrimSpace(stripSubtitleRE.ReplaceAllString(name, ""))
}

// titleCaseJournal applies journal-specific casing across words, punctuation, and parenthesized text.
func titleCaseJournal(name string) string {
	if name == "" || strings.TrimSpace(name) == "" {
		return name
	}
	name = strings.TrimSpace(name)
	var result strings.Builder
	var segment strings.Builder
	depth := 0
	flush := func() {
		if segment.Len() == 0 {
			return
		}
		result.WriteString(titleCaseJournalSegment(segment.String(), depth > 0))
		segment.Reset()
	}
	for _, character := range name {
		switch character {
		case '(':
			flush()
			result.WriteRune(character)
			depth++
		case ')':
			flush()
			result.WriteRune(character)
			if depth > 0 {
				depth--
			}
		default:
			segment.WriteRune(character)
		}
	}
	flush()
	return result.String()
}

// titleCaseJournalSegment cases one text segment while preserving its surrounding whitespace.
func titleCaseJournalSegment(value string, parenthesized bool) string {
	leadingLength := len(value) - len(strings.TrimLeft(value, " "))
	trailingLength := len(value) - len(strings.TrimRight(value, " "))
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return value
	}
	words := wordSplitRE.Split(trimmed, -1)
	for index, word := range words {
		if word == "" {
			continue
		}
		match := surroundedASCIIWordRE.FindStringSubmatch(word)
		if match == nil {
			words[index] = titleFirstRune(word)
			continue
		}
		prefix, core, suffix := match[1], match[2], match[3]
		lowerCore := strings.ToLower(core)
		switch {
		case journalIsAcronym(core):
			words[index] = prefix + strings.ToUpper(core) + suffix
		case journalLowerWords[lowerCore] && !parenthesized && (index == 0 || index == len(words)-1):
			words[index] = prefix + titleFirstRune(core) + suffix
		case journalLowerWords[lowerCore]:
			words[index] = prefix + lowerCore + suffix
		case strings.Contains(core, "-"):
			segments := strings.Split(core, "-")
			for segmentIndex, segment := range segments {
				if segment != "" {
					segments[segmentIndex] = titleFirstRune(segment)
				}
			}
			words[index] = normalizedPunctuationWord(prefix + strings.Join(segments, "-") + suffix)
		default:
			words[index] = normalizedPunctuationWord(prefix + titleFirstRune(core) + suffix)
		}
	}
	return value[:leadingLength] + strings.Join(words, " ") + value[len(value)-trailingLength:]
}

// normalizedPunctuationWord removes a duplicate terminal period introduced by casing.
func normalizedPunctuationWord(value string) string {
	if strings.HasSuffix(value, "..") {
		return strings.TrimSuffix(value, ".")
	}
	return value
}

// NormalizeJournal normalizes a journal name.
func NormalizeJournal(journal string) string {
	if journal == "" || strings.TrimSpace(journal) == "" {
		return ""
	}
	text := strings.TrimSpace(journal)
	text = html.UnescapeString(text)
	text = strings.ReplaceAll(text, "\\&", "&")
	text = whitespaceRE.ReplaceAllString(text, " ")
	text = strings.TrimSpace(text)

	if v, ok := journalCanonical[text]; ok {
		return v
	}

	text = stripJournalSubtitle(text)
	text = titleCaseJournal(text)
	return strings.TrimSpace(text)
}
