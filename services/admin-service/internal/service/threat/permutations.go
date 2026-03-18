package threat

import (
	"sort"
	"strings"
	"unicode/utf8"
)

// ─────────────────────────────────────────────────────────────────────────────
//
//	permutation.go
//
//	Target: 20,000–45,000 raw variants → ~15,000–25,000 after dedup
//
//	How volume is achieved — three levers:
//
//	  1. CROSS-PRODUCT  Every mutated label × every TLD in the list.
//	                    ~400 label mutations × 43 TLDs = ~17,000 variants.
//	                    The current code only keeps the original TLD per mutation.
//	                    This single change multiplies output by 43×.
//
//	  2. MULTI-CHAR HOMOGLYPHS  Replace 2 confusable chars simultaneously.
//	                    C(n,2) pairs × subs² adds ~500–600 variants per domain.
//
//	  3. INSERTION × ALL TLDs  Insert a char at every position × every TLD.
//	                    (n+1) positions × 36 chars × 43 TLDs ≈ 13,000+ variants.
//
// ─────────────────────────────────────────────────────────────────────────────

type permutation struct {
	Domain      string
	Label       string
	TLD         string
	Algorithm   string
	Priority    int
	LexicalRisk float64
}

// maxPermutationCount caps the raw permutation list so downstream DNS fan-out
// stays within a predictable 20–25k band per scan.
const maxPermutationCount = 9999999

func generatePermutations(domainName string) []permutation {
	label, tld := splitDomain(domainName)
	label = normalizeLabel(label)
	if label == "" || tld == "" {
		return nil
	}

	seen := make(map[string]permutation, 30000)

	// add registers one candidate. It is called millions of times so kept tight.
	add := func(candidateLabel, candidateTLD, algorithm string, priority int) {
		if candidateLabel == "" || candidateTLD == "" {
			return
		}
		if candidateLabel == label && candidateTLD == tld {
			return
		}
		domain := candidateLabel + "." + candidateTLD
		if _, exists := seen[domain]; exists {
			return
		}
		seen[domain] = permutation{
			Domain:    domain,
			Label:     candidateLabel,
			TLD:       candidateTLD,
			Algorithm: algorithm,
			Priority:  priority,
		}
	}

	// addAcrossTLDs applies a mutated label to the original TLD PLUS every
	// TLD in the list. This is the primary volume multiplier.
	addAcrossTLDs := func(mutatedLabel, algorithm string, priority int) {
		add(mutatedLabel, tld, algorithm, priority)
		for _, t := range allTLDs {
			if t != tld {
				penalty := 2
				if obscureTLDs[t] {
					penalty = 12
				}
				add(mutatedLabel, t, algorithm, priority-penalty)
			}
		}
	}

	runes := []rune(label)
	n := len(runes)

	// ── 1. OMISSION ─────────────────────────────────────────────────────────
	// Remove one char at each position.
	// acmecorp → cmecorp, amecorp, acecorp …  (n variants)
	for i := 0; i < n; i++ {
		mutated := string(runes[:i]) + string(runes[i+1:])
		if mutated != "" {
			addAcrossTLDs(mutated, "omission", 84)
		}
	}

	// ── 2. DOUBLE OMISSION ──────────────────────────────────────────────────
	// Remove two adjacent chars. Less common but covers fat-finger double delete.
	for i := 0; i < n-1; i++ {
		mutated := string(runes[:i]) + string(runes[i+2:])
		if len([]rune(mutated)) >= 2 {
			addAcrossTLDs(mutated, "double_omission", 71)
		}
	}

	// ── 3. TRANSPOSITION ────────────────────────────────────────────────────
	// Swap every adjacent pair. Most common human typo.
	// acmecorp → camecorp, acemcorp …
	for i := 0; i < n-1; i++ {
		c := make([]rune, n)
		copy(c, runes)
		c[i], c[i+1] = c[i+1], c[i]
		addAcrossTLDs(string(c), "transposition", 81)
	}

	// ── 4. SKIP TRANSPOSITION ────────────────────────────────────────────────
	// Swap chars with one position gap between them.
	for i := 0; i < n-2; i++ {
		c := make([]rune, n)
		copy(c, runes)
		c[i], c[i+2] = c[i+2], c[i]
		addAcrossTLDs(string(c), "skip_transposition", 69)
	}

	// ── 5. REPETITION ────────────────────────────────────────────────────────
	// Duplicate one char (keyboard bounce / held key).
	for i := 0; i < n; i++ {
		mutated := string(runes[:i+1]) + string(runes[i]) + string(runes[i+1:])
		addAcrossTLDs(mutated, "repetition", 76)
	}

	// ── 6. HYPHENATION ───────────────────────────────────────────────────────
	// Insert hyphen at each position. Hyphens are DNS-valid and appear in
	// legitimate domains ("my-bank.com"), which reduces user suspicion.
	for i := 1; i < n; i++ {
		mutated := string(runes[:i]) + "-" + string(runes[i:])
		add(mutated, tld, "hyphenation", 74)
		for _, t := range []string{"com", "co", "io", "net", "app"} {
			if t != tld {
				add(mutated, t, "hyphenation", 72)
			}
		}
	}

	if strings.Contains(label, "-") {
		addAcrossTLDs(strings.ReplaceAll(label, "-", ""), "hyphen_omission", 79)
	}

	// ── 7. KEYBOARD REPLACEMENT ──────────────────────────────────────────────
	// Replace each char with each of its keyboard neighbors.
	for i, ch := range runes {
		for _, neighbor := range qwertyKeyboard[ch] {
			c := make([]rune, n)
			copy(c, runes)
			c[i] = neighbor
			addAcrossTLDs(string(c), "replacement", 79)
		}
	}

	// ── 8. KEYBOARD INSERTION ────────────────────────────────────────────────
	// Insert a neighbor key before or after each position.
	for i, ch := range runes {
		for _, neighbor := range qwertyKeyboard[ch] {
			before := string(runes[:i]) + string(neighbor) + string(runes[i:])
			add(before, tld, "keyboard_insertion", 61)
			after := string(runes[:i+1]) + string(neighbor) + string(runes[i+1:])
			add(after, tld, "keyboard_insertion", 61)
		}
	}

	// ── 9. SINGLE HIT (next letter) ──────────────────────────────────────────
	for i := range runes {
		c := make([]rune, n)
		copy(c, runes)
		c[i] = nextLetter(c[i])
		addAcrossTLDs(string(c), "single_hit", 72)
	}

	// ── 10. DOUBLE HIT (next letter × 2) ─────────────────────────────────────
	for i := 0; i < n-1; i++ {
		c := make([]rune, n)
		copy(c, runes)
		c[i] = nextLetter(c[i])
		c[i+1] = nextLetter(c[i+1])
		addAcrossTLDs(string(c), "double_hit", 77)
	}

	// ── 11. HOMOGLYPH — single char ──────────────────────────────────────────
	// CRITICAL: call idna.ToASCII() before DNS lookup — stored as unicode here.
	for i, ch := range runes {
		for _, sub := range unicodeHomoglyphs[ch] {
			if !utf8.ValidRune(sub) {
				continue
			}
			c := make([]rune, n)
			copy(c, runes)
			c[i] = sub
			addAcrossTLDs(string(c), "homoglyph", 88)
		}
	}

	// ── 12. HOMOGLYPH — double char ──────────────────────────────────────────
	for i := 0; i < n; i++ {
		subsI := unicodeHomoglyphs[runes[i]]
		if len(subsI) == 0 {
			continue
		}
		for j := i + 1; j < n; j++ {
			subsJ := unicodeHomoglyphs[runes[j]]
			if len(subsJ) == 0 {
				continue
			}
			for _, si := range subsI {
				for _, sj := range subsJ {
					if !utf8.ValidRune(si) || !utf8.ValidRune(sj) {
						continue
					}
					c := make([]rune, n)
					copy(c, runes)
					c[i] = si
					c[j] = sj
					add(string(c), tld, "homoglyph_double", 86)
				}
			}
		}
	}

	// ── 13. ASCII SIMILAR ────────────────────────────────────────────────────
	for i, ch := range runes {
		for _, sub := range asciiSimilar[ch] {
			c := make([]rune, n)
			copy(c, runes)
			c[i] = sub
			addAcrossTLDs(string(c), "ascii_similar", 82)
		}
	}

	// ── 14. NUMERAL SWAP ─────────────────────────────────────────────────────
	for i, ch := range runes {
		for _, sub := range numeralSwapMap[ch] {
			c := make([]rune, n)
			copy(c, runes)
			c[i] = sub
			addAcrossTLDs(string(c), "numeral_swap", 80)
		}
	}

	// ── 15. VOWEL SWAP ───────────────────────────────────────────────────────
	for i, ch := range runes {
		for _, v := range []rune{'a', 'e', 'i', 'o', 'u'} {
			if v == ch {
				continue
			}
			switch ch {
			case 'a', 'e', 'i', 'o', 'u':
				c := make([]rune, n)
				copy(c, runes)
				c[i] = v
				addAcrossTLDs(string(c), "vowel_swap", 64)
			}
		}
	}

	// ── 16. BITSQUATTING ─────────────────────────────────────────────────────
	for i, ch := range runes {
		for bit := 0; bit < 8; bit++ {
			flipped := rune(byte(ch) ^ (1 << bit))
			if flipped == ch {
				continue
			}
			valid := (flipped >= 'a' && flipped <= 'z') ||
				(flipped >= '0' && flipped <= '9') ||
				(flipped == '-' && i > 0 && i < n-1)
			if !valid {
				continue
			}
			c := make([]rune, n)
			copy(c, runes)
			c[i] = flipped
			addAcrossTLDs(string(c), "bitsquatting", 65)
		}
	}

	// ── 17. SEQUENCE SWAP ────────────────────────────────────────────────────
	for source, replacements := range sequenceReplacements {
		if !strings.Contains(label, source) {
			continue
		}
		for _, rep := range replacements {
			mutated := strings.Replace(label, source, rep, 1)
			addAcrossTLDs(mutated, "sequence_swap", 82)
		}
	}

	// ── 18. SUBDOMAIN SPLIT ──────────────────────────────────────────────────
	for i := 1; i < n; i++ {
		mutated := string(runes[:i]) + "." + string(runes[i:]) + "." + tld
		add(mutated, tld, "subdomain", 75)
	}

	// ── 19. INSERTION ────────────────────────────────────────────────────────
	insertionChars := []rune("abcdefghijklmnopqrstuvwxyz0123456789")
	for _, t := range allTLDs {
		for pos := 0; pos <= n; pos++ {
			for _, ch := range insertionChars {
				mutated := string(runes[:pos]) + string(ch) + string(runes[pos:])
				add(mutated, t, "insertion", 75)
			}
		}
	}

	// ── 20. PREFIX × ALL TLDs ────────────────────────────────────────────────
	for _, p := range allPrefixes {
		for _, t := range allTLDs {
			add(p+"-"+label, t, "prefix", 72)
			add(p+label, t, "prefix_compaction", 68)
		}
		add(label+"-"+p, tld, "suffix_phrase", 62)
	}

	// ── 21. SUFFIX × ALL TLDs ────────────────────────────────────────────────
	for _, s := range allSuffixes {
		for _, t := range allTLDs {
			add(label+"-"+s, t, "suffix", 70)
			add(label+s, t, "suffix_compaction", 65)
		}
	}

	// ── 22. TLD REPLACE ──────────────────────────────────────────────────────
	for _, t := range allTLDs {
		if t != tld {
			add(label, t, "tld_replace", 83)
		}
	}

	// ── 23. TLD REPEAT ───────────────────────────────────────────────────────
	add(label+"."+tld+"."+tld, tld, "tld_repeat_dot", 57)
	add(label+strings.ReplaceAll(tld, ".", ""), tld, "tld_repeat_compact", 57)
	for _, t := range allTLDs {
		if t != tld {
			add(label+strings.ReplaceAll(tld, ".", ""), t, "tld_appended", 55)
		}
	}

	// ── 24. ADDITION ─────────────────────────────────────────────────────────
	additionChars := []rune("abcdefghijklmnopqrstuvwxyz0123456789")
	for _, ch := range additionChars {
		for _, t := range allTLDs {
			add(label+string(ch), t, "addition", 78)
		}
	}

	// ── 25. PLURALISATION ────────────────────────────────────────────────────
	if !strings.HasSuffix(label, "s") {
		addAcrossTLDs(label+"s", "pluralization", 59)
	} else {
		addAcrossTLDs(strings.TrimSuffix(label, "s"), "singularization", 58)
	}

	// ── 26. WWW PREFIX ───────────────────────────────────────────────────────
	add("www"+label, tld, "www_compaction", 66)
	for _, t := range allTLDs {
		add("www-"+label, t, "www_hyphen", 64)
	}

	// ── 27. COMMON MISSPELLINGS ──────────────────────────────────────────────
	for wrong, correct := range commonMisspellings {
		if strings.Contains(label, correct) {
			mutated := strings.Replace(label, correct, wrong, 1)
			addAcrossTLDs(mutated, "misspelling", 76)
		}
		if strings.Contains(label, wrong) {
			mutated := strings.Replace(label, wrong, correct, 1)
			addAcrossTLDs(mutated, "misspelling_correct", 71)
		}
	}

	// ── 28. PREFIX-TLD COMBO ─────────────────────────────────────────────────
	highRiskPrefixes := []string{"login", "secure", "portal", "support", "auth", "verify", "signin", "account", "billing", "pay"}
	highRiskTLDs := []string{"com", "net", "org", "co", "io", "app", "co.uk", "ai", "dev"}
	for _, p := range highRiskPrefixes {
		for _, t := range highRiskTLDs {
			add(p+"-"+label, t, "prefix_tld_combo", 93)
			add(p+label, t, "prefix_tld_combo", 90)
		}
	}

	// ── Score and sort ────────────────────────────────────────────────────────
	//
	// LexicalRisk is always computed against the ORIGINAL brand label.
	// For compound candidates like "login-paytm", compare against the
	// brand core rather than the full compound string.
	results := make([]permutation, 0, len(seen))
	for _, candidate := range seen {
		// Compare the full candidate label against the brand label.
		// Do NOT normalize compound labels (e.g. "remotenike" → "nike")
		// because that inflates LexicalRisk to 1.0 for all prefix/suffix
		// variants, causing them to dominate over single-char mutations.
		candidate.LexicalRisk = lexicalSimilarity(label, candidate.Label)
		results = append(results, candidate)
	}

	sort.Slice(results, func(i, j int) bool {
		li := float64(results[i].Priority) + results[i].LexicalRisk*20
		lj := float64(results[j].Priority) + results[j].LexicalRisk*20
		if li == lj {
			return results[i].Domain < results[j].Domain
		}
		return li > lj
	})

	return shortlistPermutations(results, maxPermutationCount)
}

func shortlistPermutations(perms []permutation, limit int) []permutation {
	if len(perms) <= limit {
		return perms
	}
	return perms[:limit]
}

// ─────────────────────────────────────────────────────────────────────────────
//  Pre-score filter
// ─────────────────────────────────────────────────────────────────────────────

// preScoreFilter cuts the full permutation list down to only the candidates
// worth DNS-resolving, using only the in-memory fields computed by the
// permutation generator. Zero network calls.
//
// TUNED: thresholds widened to surface more domains (matching haveibeensquatted).
func preScoreFilter(perms []permutation) []permutation {
	if len(perms) == 0 {
		return nil
	}

	const (
		// TUNED: check ALL permutations for DNS — matching haveibeensquatted.
		maxDNSCandidates = 9999999 // Cap removed per user request

		// Lowered from 0.70 to 0.45 — catches more single-char mutations.
		highSimilarity float64 = 0.45

		// Lowered from 70/0.50 to 50/0.30 — lets addition/insertion through.
		highPriorityAlg             = 50
		minSimilarityForPriorityAlg float64 = 0.30
	)

	// Expanded always-check set.
	alwaysCheck := map[string]bool{
		"homoglyph":          true,
		"homoglyph_double":   true,
		"omission":           true,
		"insertion":          true,
		"transposition":      true,
		"repetition":         true,
		"prefix_tld_combo":   true,
		"ascii_similar":      true,
		"sequence_swap":      true,
		"addition":           true,
		"replacement":        true,
		"bitsquatting":       true,
		"numeral_swap":       true,
		"vowel_swap":         true,
		"tld_replace":        true,
		"prefix":             true,
		"suffix":             true,
		"prefix_compaction":  true,
		"suffix_compaction":  true,
		"keyboard_insertion": true,
	}

	type scored struct {
		p     permutation
		score float64
	}

	candidates := make([]scored, 0, 512)

	for _, p := range perms {
		keep := p.LexicalRisk >= highSimilarity ||
			(p.Priority >= highPriorityAlg && p.LexicalRisk >= minSimilarityForPriorityAlg) ||
			alwaysCheck[p.Algorithm]

		if !keep {
			continue
		}

		score := p.LexicalRisk*30 + float64(p.Priority)/5
		candidates = append(candidates, scored{p: p, score: score})
	}

	if len(candidates) == 0 {
		return shortlistPermutations(perms, maxDNSCandidates)
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].score == candidates[j].score {
			return candidates[i].p.Domain < candidates[j].p.Domain
		}
		return candidates[i].score > candidates[j].score
	})

	if len(candidates) > maxDNSCandidates {
		candidates = candidates[:maxDNSCandidates]
	}

	out := make([]permutation, len(candidates))
	for i, c := range candidates {
		out[i] = c.p
	}
	return out
}


var qwertyKeyboard = map[rune][]rune{
	'a': {'q', 'w', 's', 'z'},
	'b': {'v', 'g', 'h', 'n'},
	'c': {'x', 'd', 'f', 'v'},
	'd': {'s', 'e', 'r', 'f', 'c', 'x'},
	'e': {'w', 's', 'd', 'r'},
	'f': {'d', 'r', 't', 'g', 'v', 'c'},
	'g': {'f', 't', 'y', 'h', 'b', 'v'},
	'h': {'g', 'y', 'u', 'j', 'n', 'b'},
	'i': {'u', 'j', 'k', 'o'},
	'j': {'h', 'u', 'i', 'k', 'm', 'n'},
	'k': {'j', 'i', 'o', 'l', 'm'},
	'l': {'k', 'o', 'p'},
	'm': {'n', 'j', 'k'},
	'n': {'b', 'h', 'j', 'm'},
	'o': {'i', 'k', 'l', 'p'},
	'p': {'o', 'l'},
	'q': {'w', 'a', 's'},
	'r': {'e', 'd', 'f', 't'},
	's': {'a', 'w', 'e', 'd', 'x', 'z'},
	't': {'r', 'f', 'g', 'y'},
	'u': {'y', 'h', 'j', 'i'},
	'v': {'c', 'f', 'g', 'b'},
	'w': {'q', 'a', 's', 'e'},
	'x': {'z', 's', 'd', 'c'},
	'y': {'t', 'g', 'h', 'u'},
	'z': {'a', 's', 'x'},
	'0': {'9', 'o'},
	'1': {'2', 'q'},
	'2': {'1', '3', 'w'},
	'3': {'2', '4', 'e'},
	'4': {'3', '5', 'r'},
	'5': {'4', '6', 't'},
	'6': {'5', '7', 'y'},
	'7': {'6', '8', 'u'},
	'8': {'7', '9', 'i'},
	'9': {'8', '0', 'o'},
}

var unicodeHomoglyphs = map[rune][]rune{
	'a': {'\u0430', '\u0251', '\u03B1', '\u00E0'},
	'b': {'\u0432', '\u0253', '\u00DF'},
	'c': {'\u0441', '\u03F2', '\u00E7'},
	'd': {'\u0501', '\u0257', '\u00F0'},
	'e': {'\u0435', '\u04BD', '\u03B5', '\u00E8'},
	'f': {'\u0192'},
	'g': {'\u0261', '\u018D', '\u0581'},
	'h': {'\u04BB', '\u0570', '\u043D'},
	'i': {'\u0456', '\u04CF', '\u0131', '\u00ED'},
	'j': {'\u0458', '\u03F3'},
	'k': {'\u03BA', '\u043A'},
	'l': {'\u04CF', '\u1D0C', '\u0269', '\u007C'},
	'm': {'\u043C', '\u1D0D'},
	'n': {'\u043F', '\u0578', '\u00F1'},
	'o': {'\u043E', '\u03BF', '\u00F8', '\u00F3'},
	'p': {'\u0440', '\u03C1'},
	'q': {'\u051B', '\u0563'},
	'r': {'\u0433', '\u1D26'},
	's': {'\u0455', '\u0282', '\u00DF'},
	't': {'\u0442', '\u03C4'},
	'u': {'\u03C5', '\u028B', '\u00FA'},
	'v': {'\u03BD', '\u0475'},
	'w': {'\u0461', '\u051D'},
	'x': {'\u0445', '\u03C7'},
	'y': {'\u0443', '\u03B3'},
	'z': {'\u0290', '\u03B6'},
}

var asciiSimilar = map[rune][]rune{
	'a': {'4'},
	'b': {'6'},
	'e': {'3'},
	'g': {'9', 'q'},
	'i': {'1', 'l'},
	'l': {'1', 'i'},
	'o': {'0'},
	's': {'5'},
	't': {'7'},
	'z': {'2'},
	'0': {'o'},
	'1': {'l', 'i'},
	'3': {'e'},
	'4': {'a'},
	'5': {'s'},
	'6': {'b'},
	'7': {'t'},
	'9': {'g'},
}

var numeralSwapMap = map[rune][]rune{
	'a': {'4'},
	'e': {'3'},
	'i': {'1'},
	'l': {'1'},
	'o': {'0'},
	's': {'5'},
	't': {'7'},
	'b': {'8'},
	'g': {'9'},
	'0': {'o'},
	'1': {'i', 'l'},
	'3': {'e'},
	'4': {'a'},
	'5': {'s'},
	'7': {'t'},
	'8': {'b'},
	'9': {'g'},
}

var sequenceReplacements = map[string][]string{
	"rn": {"m"},
	"m":  {"rn"},
	"cl": {"d"},
	"d":  {"cl"},
	"vv": {"w"},
	"w":  {"vv"},
	"nn": {"m"},
	"ii": {"u"},
	"ou": {"o", "u"},
}

var allTLDs = []string{
	"com", "net", "org", "info", "biz", "name",
	"io", "ai", "app", "dev", "co", "tech", "cloud",
	"us", "co.us",
	"co.uk", "uk", "ca", "com.au", "au", "de", "fr",
	"es", "it", "nl", "pl", "se", "ch", "at", "be",
	"ru", "cn", "jp", "kr", "in", "com.br", "br",
	"mx", "com.mx", "za", "co.za", "nz", "co.nz",
	"xyz", "top", "online", "site", "store", "web",
	"digital", "global", "group", "live", "media",
	"news", "pro", "services", "solutions", "support",
	"systems", "world", "space", "click", "link",
	"tv", "cc", "ws", "me", "to",
}

var obscureTLDs = map[string]bool{
	"xyz": true, "top": true, "online": true, "site": true, "store": true, "web": true,
	"digital": true, "global": true, "group": true, "live": true, "media": true,
	"news": true, "pro": true, "services": true, "solutions": true, "support": true,
	"systems": true, "world": true, "space": true, "click": true, "link": true,
	"tv": true, "cc": true, "ws": true, "me": true, "to": true, "info": true, "biz": true, "name": true,
}

var allPrefixes = []string{
	"login", "signin", "signup", "auth", "sso", "oauth",
	"account", "accounts", "my", "myaccount", "portal",
	"register", "reset", "verify", "verification", "confirm",
	"secure", "ssl", "safe", "trust", "verified",
	"support", "help", "service", "services", "customer",
	"client", "member", "user", "staff",
	"mail", "email", "webmail", "mail1", "mail2",
	"admin", "api", "app", "apps", "web", "www",
	"cdn", "static", "assets", "dev", "staging",
	"intranet", "vpn", "remote", "gateway",
	"shop", "store", "buy", "order", "checkout",
	"payment", "pay", "billing", "invoice",
	"get", "go", "try", "join", "start", "now",
	"corp", "hq", "team", "official", "real", "legit",
}

var allSuffixes = []string{
	"login", "signin", "auth", "sso", "account", "portal",
	"verify", "verification", "secure", "ssl",
	"app", "web", "site", "online", "platform",
	"support", "help", "service", "services",
	"shop", "store", "pay", "payment", "checkout",
	"inc", "llc", "corp", "ltd", "group", "co",
	"hq", "hub", "central", "global", "official",
	"net", "io", "api", "cloud", "tech",
}

var commonMisspellings = map[string]string{
	"acount":           "account",
	"acound":           "account",
	"authetication":    "authentication",
	"authentification": "authentication",
	"billig":           "billing",
	"busines":          "business",
	"bussiness":        "business",
	"ceck":             "check",
	"checkuot":         "checkout",
	"custmer":          "customer",
	"custommer":        "customer",
	"defualt":          "default",
	"definately":       "definitely",
	"delievry":         "delivery",
	"enviroment":       "environment",
	"fastrack":         "fasttrack",
	"freind":           "friend",
	"gurantee":         "guarantee",
	"guarentee":        "guarantee",
	"helth":            "health",
	"hompage":          "homepage",
	"insurnace":        "insurance",
	"insurrance":       "insurance",
	"legasy":           "legacy",
	"managment":        "management",
	"mesage":           "message",
	"moble":            "mobile",
	"moblie":           "mobile",
	"notifcation":      "notification",
	"onlne":            "online",
	"openning":         "opening",
	"pasword":          "password",
	"passowrd":         "password",
	"patment":          "payment",
	"paymant":          "payment",
	"portle":           "portal",
	"prefered":         "preferred",
	"privcy":           "privacy",
	"regsiter":         "register",
	"registartion":     "registration",
	"regsitration":     "registration",
	"secruity":         "security",
	"securty":          "security",
	"seperaet":         "separate",
	"seperate":         "separate",
	"setings":          "settings",
	"seting":           "settings",
	"shoping":          "shopping",
	"signig":           "signing",
	"sofware":          "software",
	"softawre":         "software",
	"subcribe":         "subscribe",
	"subscrbe":         "subscribe",
	"suport":           "support",
	"trasaction":       "transaction",
	"transacton":       "transaction",
	"upadted":          "updated",
	"upated":           "updated",
	"usre":             "user",
	"uzer":             "user",
	"verfiy":           "verify",
	"verifcation":      "verification",
	"walelt":           "wallet",
	"walllet":          "wallet",
}

// ─────────────────────────────────────────────────────────────────────────────
//  Helpers
// ─────────────────────────────────────────────────────────────────────────────

func splitDomain(domainName string) (string, string) {
	d := strings.ToLower(strings.TrimSpace(domainName))
	parts := strings.SplitN(d, ".", 2)
	if len(parts) < 2 {
		return d, "com"
	}
	return parts[0], parts[1]
}

func normalizeLabel(label string) string {
	label = strings.ToLower(strings.TrimSpace(label))
	label = strings.Trim(label, "-.")
	if label == "" {
		return ""
	}
	for _, ch := range label {
		if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '-' {
			continue
		}
		return label
	}
	return label
}

func nextLetter(ch rune) rune {
	switch {
	case ch >= 'a' && ch < 'z':
		return ch + 1
	case ch == 'z':
		return 'a'
	case ch >= '0' && ch < '9':
		return ch + 1
	case ch == '9':
		return '0'
	default:
		return ch
	}
}

func lexicalSimilarity(s1, s2 string) float64 {
	r1 := []rune(s1)
	r2 := []rune(s2)
	l1, l2 := len(r1), len(r2)
	if l1 == 0 && l2 == 0 {
		return 1.0
	}
	if l1 == 0 || l2 == 0 {
		return 0.0
	}

	matchDist := max(l1, l2)/2 - 1
	if matchDist < 0 {
		matchDist = 0
	}

	m1 := make([]bool, l1)
	m2 := make([]bool, l2)
	matches := 0
	for i := range r1 {
		lo := i - matchDist
		if lo < 0 {
			lo = 0
		}
		hi := i + matchDist + 1
		if hi > l2 {
			hi = l2
		}
		for j := lo; j < hi; j++ {
			if m2[j] || r1[i] != r2[j] {
				continue
			}
			m1[i] = true
			m2[j] = true
			matches++
			break
		}
	}
	if matches == 0 {
		return 0.0
	}

	t := 0
	k := 0
	for i := range r1 {
		if !m1[i] {
			continue
		}
		for k < l2 && !m2[k] {
			k++
		}
		if k < l2 && r1[i] != r2[k] {
			t++
		}
		k++
	}
	jaro := (float64(matches)/float64(l1) +
		float64(matches)/float64(l2) +
		float64(matches-t/2)/float64(matches)) / 3.0

	prefix := 0
	for i := 0; i < min(4, min(l1, l2)); i++ {
		if r1[i] == r2[i] {
			prefix++
		} else {
			break
		}
	}
	return jaro + float64(prefix)*0.1*(1-jaro)
}

// levenshteinDistance returns the minimum number of single-character edits
// (insertions, deletions, substitutions) to transform a into b.
// Used by scoreThreat to give a proximity bonus to close mutations.
func levenshteinDistance(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	la, lb := len(ra), len(rb)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}
	v0 := make([]int, lb+1)
	v1 := make([]int, lb+1)
	for i := 0; i <= lb; i++ {
		v0[i] = i
	}
	for i := 0; i < la; i++ {
		v1[0] = i + 1
		for j := 0; j < lb; j++ {
			cost := 1
			if ra[i] == rb[j] {
				cost = 0
			}
			v1[j+1] = min(v1[j]+1, min(v0[j+1]+1, v0[j]+cost))
		}
		for j := 0; j <= lb; j++ {
			v0[j] = v1[j]
		}
	}
	return v1[lb]
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
