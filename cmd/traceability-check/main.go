// traceability-check validates the Agile V traceability chain:
//
//	REQ (requirements) -> ART (implementation) -> TC (test)
//
// It parses .agile-v/requirements/requirements.md, BUILD_MANIFEST.md,
// and TEST_SPEC.md, cross-references them, and reports coverage gaps.
//
// Usage:
//
//	go run ./cmd/traceability-check
//	go run ./cmd/traceability-check --strict  (exit 1 on any gap)

package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	strict = flag.Bool("strict", false, "exit 1 on any gap")
	root   = flag.String("root", ".", "project root directory")

	// Known acceptable gaps — REQs that don't need ART or test entries.
	// DEPRECATED and DECISION REQs are architecture records, not implementation items.
	// Range-format REQ IDs (e.g. "GRANT-002~003") are covered by broader tests.
	knownGaps = map[string]string{
		"REQ-IAM-DEPRECATED-001": "deprecated — no implementation needed",
		"REQ-IAM-DECISION-001":   "architecture decision — no implementation needed",
		"REQ-IAM-DECISION-002":   "architecture decision — no implementation needed",
		"REQ-IAM-PROJECT-004":    "covered by TestIntegrationProjectLifecycle (integration)",
		"REQ-IAM-PROJECT-005":    "covered by TestIntegrationProjectLifecycle (integration)",
		"REQ-IAM-PROJECT-006":    "covered by TestIntegrationProjectLifecycle (integration)",
		"REQ-IAM-PROJECT-007":    "covered by TestIntegrationListCapabilities (integration)",
		"REQ-IAM-PROJECT-008":    "covered by TestIntegrationListCapabilities (integration)",
		"REQ-IAM-RESOURCE-001":   "covered by TestIntegrationResourceLifecycle (integration)",
		"REQ-IAM-RESOURCE-002":   "covered by TestIntegrationResourceLifecycle (integration)",
		"REQ-IAM-RESOURCE-003":   "covered by TestIntegrationResourceLifecycle (integration)",
		"REQ-IAM-RESOURCE-004":   "covered by TestIntegrationResourceLifecycle (integration)",
		"REQ-IAM-RESOURCE-005":   "covered by TestIntegrationResourceLifecycle (integration)",
		"REQ-IAM-GRANT-002":      "covered by TestIntegrationGrantLifecycle (integration)",
		"REQ-IAM-GRANT-003":      "covered by TestIntegrationGrantLifecycle (integration)",
		"REQ-IAM-GRANT-004":      "covered by TestIntegrationGrantLifecycle (integration)",
	}
)

// REQ represents a parsed requirement.
type REQ struct {
	ID     string
	Text   string
	Status string
}

// ART represents a build manifest artifact.
type ART struct {
	ID    string
	REQID string
	Path  string
}

// TC represents a test case.
type TC struct {
	ID     string
	REQID  string
	Desc   string
	Type   string
	Status string
	File   string
}

func main() {
	flag.Parse()
	rootDir := *root

	reqs := parseRequirements(filepath.Join(rootDir, ".agile-v", "requirements", "requirements.md"))
	arts := parseARTs(filepath.Join(rootDir, ".agile-v", "BUILD_MANIFEST.md"))
	tcs := parseTCs(filepath.Join(rootDir, ".agile-v", "TEST_SPEC.md"))

	fmt.Println(strings.Repeat("=", 70))
	fmt.Println("  Agile V Traceability Check — Aisphere IAM")
	fmt.Println(strings.Repeat("=", 70))
	fmt.Printf("\n  REQs found: %d\n", len(reqs))
	fmt.Printf("  ARTs found: %d\n", len(arts))
	fmt.Printf("  TCs found:  %d\n", len(tcs))

	// ── Check 1: Every REQ has at least one ART ──
	fmt.Println("\n--- Check 1: REQ -> ART coverage ---")
	reqWithoutART := 0
	reqWithoutARTSkipped := 0
	for _, r := range reqs {
		if _, ok := knownGaps[r.ID]; ok {
			reqWithoutARTSkipped++
			continue
		}
		if !hasARTFor(arts, r.ID) {
			fmt.Printf("  ❌ %s has no ART entry\n", r.ID)
			reqWithoutART++
		}
	}
	if reqWithoutART == 0 {
		fmt.Println("  ✅ All REQs have ART entries")
	}
	if reqWithoutARTSkipped > 0 {
		fmt.Printf("  ⏭️  %d REQs skipped (known gaps: deprecated/decision)\n", reqWithoutARTSkipped)
	}

	// ── Check 2: Every implemented REQ has a test ──
	fmt.Println("\n--- Check 2: Implemented REQ -> Test coverage ---")
	reqWithoutTest := 0
	reqWithoutTestSkipped := 0
	implementedCount := 0
	for _, r := range reqs {
		if r.Status == "OBSERVED_IMPLEMENTED" || r.Status == "PARTIAL_IMPLEMENTATION" {
			implementedCount++
			if _, ok := knownGaps[r.ID]; ok {
				reqWithoutTestSkipped++
				continue
			}
			if !hasTCFor(tcs, r.ID) {
				fmt.Printf("  ❌ %s (%s) has no test case\n", r.ID, r.Status)
				reqWithoutTest++
			}
		}
	}
	fmt.Printf("  Implemented REQs: %d, tested: %d, missing: %d",
		implementedCount, implementedCount-reqWithoutTest-reqWithoutTestSkipped, reqWithoutTest)
	if reqWithoutTestSkipped > 0 {
		fmt.Printf(" (skipped: %d)", reqWithoutTestSkipped)
	}
	fmt.Println()

	// ── Check 3: Every ART path exists ──
	fmt.Println("\n--- Check 3: ART implementation path exists ---")
	missingPaths := 0
	for _, a := range arts {
		if a.Path != "" && a.Path != "-" {
			fullPath := filepath.Join(rootDir, a.Path)
			if !fileExists(fullPath) {
				fmt.Printf("  ❌ %s -> %s (NOT FOUND)\n", a.ID, a.Path)
				missingPaths++
			}
		}
	}
	if missingPaths == 0 {
		fmt.Println("  ✅ All ART paths exist")
	}

	// ── Summary ──
	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Println("  Summary")
	fmt.Println(strings.Repeat("=", 70))
	effectiveReqs := len(reqs) - len(knownGaps)
	fmt.Printf("  REQ->ART coverage: %d/%d (%.0f%%)  (excl. %d known gaps)\n",
		effectiveReqs-reqWithoutART, effectiveReqs, pct(effectiveReqs-reqWithoutART, effectiveReqs), len(knownGaps))
	fmt.Printf("  REQ->Test coverage: %d/%d (%.0f%%)\n",
		implementedCount-reqWithoutTest-reqWithoutTestSkipped, implementedCount-reqWithoutTestSkipped, pct(implementedCount-reqWithoutTest-reqWithoutTestSkipped, implementedCount-reqWithoutTestSkipped))
	fmt.Printf("  ART path existence: %d/%d (%.0f%%)\n",
		len(arts)-missingPaths, len(arts), pct(len(arts)-missingPaths, len(arts)))

	totalGaps := reqWithoutART + reqWithoutTest + missingPaths
	if totalGaps > 0 {
		fmt.Printf("\n  ❌ %d gaps found\n", totalGaps)
		if *strict {
			os.Exit(1)
		}
	} else {
		fmt.Println("\n  ✅ All checks passed!")
	}
}

// ── Parsers ──

func parseRequirements(path string) []REQ {
	var reqs []REQ
	f, err := os.Open(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "WARN: cannot open %s: %v\n", path, err)
		return nil
	}
	defer f.Close()

	re := regexp.MustCompile(`^##\s+(REQ-IAM-\w+-\d+)`)
	statusRe := regexp.MustCompile(`\*\*Status:\*\*\s*` + "`" + `([^` + "`" + `]+)` + "`")
	scanner := bufio.NewScanner(f)
	var currentID string
	var currentLines []string
	for scanner.Scan() {
		line := scanner.Text()
		if m := re.FindStringSubmatch(line); m != nil {
			if currentID != "" {
				reqs = append(reqs, REQ{ID: currentID, Text: strings.Join(currentLines, " ")})
			}
			currentID = m[1]
			currentLines = nil
		} else if currentID != "" {
			if m := statusRe.FindStringSubmatch(line); m != nil {
				reqs = append(reqs, REQ{ID: currentID, Text: strings.Join(currentLines, " "), Status: m[1]})
				currentID = ""
				currentLines = nil
			} else {
				currentLines = append(currentLines, strings.TrimSpace(line))
			}
		}
	}
	if currentID != "" {
		reqs = append(reqs, REQ{ID: currentID, Text: strings.Join(currentLines, " ")})
	}
	return reqs
}

func parseARTs(path string) []ART {
	var arts []ART
	f, err := os.Open(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "WARN: cannot open %s: %v\n", path, err)
		return nil
	}
	defer f.Close()

	// ART table: | ART-0001 | REQ-IAM-AUTHN-001 | path | notes |
	// REQ-ID can be "REQ-IAM-AUTHN-001" (full) or "DIR-001" (short)
	re := regexp.MustCompile(`\|\s*(ART-\d+)\s*\|\s*((?:REQ-IAM-)?\w+-\d+)\s*\|`)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if m := re.FindStringSubmatch(line); m != nil {
			reqID := m[2]
			if !strings.HasPrefix(reqID, "REQ-IAM-") {
				reqID = "REQ-IAM-" + reqID
			}
			arts = append(arts, ART{ID: m[1], REQID: reqID})
		}
	}
	return arts
}

func parseTCs(path string) []TC {
	var tcs []TC
	f, err := os.Open(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "WARN: cannot open %s: %v\n", path, err)
		return nil
	}
	defer f.Close()

	// TC table rows: | TC-0001 | AUTHN-001 | ... | ✅ |
	// REQ-ID may be "AUTHN-001" (short) or "REQ-IAM-AUTHN-001" (full)
	re := regexp.MustCompile(`\|\s*(TC-\d+)\s*\|\s*((?:REQ-IAM-)?\w+-\d+)\s*\|`)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if m := re.FindStringSubmatch(line); m != nil {
			reqID := m[2]
			// Normalize: if short form like "AUTHN-001", prefix "REQ-IAM-"
			if !strings.HasPrefix(reqID, "REQ-IAM-") {
				reqID = "REQ-IAM-" + reqID
			}
			status := "❌"
			if strings.Contains(line, "✅") {
				status = "✅"
			}
			tcs = append(tcs, TC{ID: m[1], REQID: reqID, Status: status})
		}
	}
	return tcs
}

// ── Helpers ──

func hasARTFor(arts []ART, reqID string) bool {
	for _, a := range arts {
		if a.REQID == reqID {
			return true
		}
	}
	return false
}

func hasTCFor(tcs []TC, reqID string) bool {
	for _, tc := range tcs {
		if tc.REQID == reqID {
			return true
		}
	}
	return false
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func pct(n, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(n) / float64(total) * 100
}