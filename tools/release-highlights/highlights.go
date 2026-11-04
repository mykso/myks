package main

import (
	"cmp"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

const (
	startMarker = "<!-- release-highlights:start -->"
	endMarker   = "<!-- release-highlights:end -->"
)

var (
	// titleTypeRe matches the Conventional Commits type of a pull request title.
	titleTypeRe = regexp.MustCompile(`^(\w+)(\([^)]*\))?!?:`)
	sectionRe   = regexp.MustCompile(`(?i)^##\s+release note\s*$`)
	// otherSectionRe ends the release note: the next level-one or level-two heading.
	otherSectionRe = regexp.MustCompile(`^#{1,2}\s`)
	commentRe      = regexp.MustCompile(`(?s)<!--.*?-->`)
	// prRefRe matches the pull request links release-please writes into changelog lines.
	prRefRe = regexp.MustCompile(`\[#(\d+)\]\(https://github\.com/[^/]+/[^/]+/(?:issues|pull)/\d+\)`)
	// versionHeadingRe is release-please's own pattern for the heading that starts the notes.
	versionHeadingRe = regexp.MustCompile(`^#{2,} \[?\d+\.\d+\.\d+`)
)

// typeRank orders highlights by type; titles of other types never need a release note.
var typeRank = map[string]int{"feat": 0, "fix": 1, "perf": 2}

// botAuthors open pull requests whose titles carry a type but no human-written release note.
var botAuthors = []string{"renovate[bot]", "dependabot[bot]", "mykso-bot"}

func titleType(title string) string {
	if m := titleTypeRe.FindStringSubmatch(title); m != nil {
		return m[1]
	}
	return ""
}

// needsReleaseNote reports whether a pull request must carry a "## Release note" section.
func needsReleaseNote(title, author string) bool {
	_, ranked := typeRank[titleType(title)]
	return ranked && !slices.Contains(botAuthors, author)
}

// releaseNote returns the "## Release note" section of a pull request description, without
// HTML comments. found is false when the description has no such section.
func releaseNote(body string) (text string, found bool) {
	var lines []string
	inSection, inFence := false, false
	for line := range strings.SplitSeq(strings.ReplaceAll(body, "\r\n", "\n"), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inFence = !inFence
		}
		switch {
		case !inFence && sectionRe.MatchString(line):
			inSection, found = true, true
		case inSection && !inFence && otherSectionRe.MatchString(line):
			inSection = false
		case inSection:
			lines = append(lines, line)
		}
	}
	return strings.TrimSpace(commentRe.ReplaceAllString(strings.Join(lines, "\n"), "")), found
}

func rank(title string) int {
	if r, ok := typeRank[titleType(title)]; ok {
		return r
	}
	return len(typeRank)
}

// hasDetails reports a <details> element, which release-please reads in its notes as a
// per-component release block.
func hasDetails(note string) bool {
	return strings.Contains(strings.ToLower(note), "<details")
}

func isNone(note string) bool {
	return strings.EqualFold(strings.TrimSuffix(strings.TrimSpace(note), "."), "none")
}

// checkReleaseNote returns why a pull request description fails the release note rule, or "".
func checkReleaseNote(title, author, body string) string {
	if !needsReleaseNote(title, author) {
		return ""
	}
	note, found := releaseNote(body)
	switch {
	case !found:
		return `add a "## Release note" section: what a user notices, or "none"`
	case note == "":
		return `the "## Release note" section is empty: write what a user notices, or "none"`
	case hasDetails(note):
		return `the "## Release note" section must not use <details>`
	}
	return ""
}

// prNumbers returns the pull requests a release-please changelog links to, in order, once each.
func prNumbers(changelog string) []int {
	var numbers []int
	for _, m := range prRefRe.FindAllStringSubmatch(changelog, -1) {
		n, err := strconv.Atoi(m[1])
		if err == nil && !slices.Contains(numbers, n) {
			numbers = append(numbers, n)
		}
	}
	return numbers
}

type entry struct {
	number    int
	title     string
	note      string
	highlight bool
}

// render builds the highlights block: entries labelled "highlight" first, then features, fixes,
// performance improvements and the rest, each by pull request number. It returns "" when no
// entry has a note.
func render(entries []entry, repoURL string) string {
	var kept []entry
	for _, e := range entries {
		if e.note != "" && !isNone(e.note) && !hasDetails(e.note) {
			kept = append(kept, e)
		}
	}
	if len(kept) == 0 {
		return ""
	}
	slices.SortStableFunc(kept, func(a, b entry) int {
		if a.highlight != b.highlight {
			if a.highlight {
				return -1
			}
			return 1
		}
		return cmp.Or(cmp.Compare(rank(a.title), rank(b.title)), cmp.Compare(a.number, b.number))
	})
	parts := []string{startMarker + "\n### Highlights"}
	for _, e := range kept {
		parts = append(parts, withReference(e.note, fmt.Sprintf("([#%d](%s/pull/%d))", e.number, repoURL, e.number)))
	}
	return strings.Join(parts, "\n\n") + "\n" + endMarker
}

// withReference appends the pull request link to the first paragraph of a note, or on a line of
// its own when that paragraph is a code block.
func withReference(note, ref string) string {
	lines := strings.Split(note, "\n")
	end := len(lines)
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			end = i
			break
		}
	}
	for _, line := range lines[:end] {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			return note + "\n\n" + ref
		}
	}
	lines[end-1] += " " + ref
	return strings.Join(lines, "\n")
}

// applyHighlights replaces the highlights block of a release-please pull request body, placing it
// right after the version heading, which release-please needs to stay the first line of the notes.
// An empty block removes it. Removal undoes insertion exactly, so reapplying changes nothing.
func applyHighlights(body, block string) (string, error) {
	if start := strings.Index(body, startMarker); start >= 0 {
		end := strings.Index(body[start:], endMarker)
		if end < 0 {
			return "", fmt.Errorf("%q without %q", startMarker, endMarker)
		}
		body = strings.TrimSuffix(body[:start], "\n\n") + body[start+end+len(endMarker):]
	}
	if block == "" {
		return body, nil
	}
	lines := strings.Split(body, "\n")
	for i, line := range lines {
		if versionHeadingRe.MatchString(line) {
			lines[i] += "\n\n" + block
			return strings.Join(lines, "\n"), nil
		}
	}
	return "", errors.New("no version heading in the release pull request body")
}
