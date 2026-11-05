package main

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// releaseBody is the shape of a release-please pull request body (taken from mykso/myks#974).
const releaseBody = `:robot: I have created a release *beep* *boop*
---


## [5.17.0](https://github.com/mykso/myks/compare/v5.16.0...v5.17.0) (2026-11-04)


### Features

* describe the pipeline and link the docs in the root help ([9021098](https://github.com/mykso/myks/commit/9021098))
* **init:** add --ci github ([#976](https://github.com/mykso/myks/issues/976)) ([edc289e](https://github.com/mykso/myks/commit/edc289e))
* **init:** scaffold dev and prod ([#975](https://github.com/mykso/myks/issues/975)) ([ca7a384](https://github.com/mykso/myks/commit/ca7a384))


### Bug Fixes

* do not panic on an invalid min-version ([#977](https://github.com/mykso/myks/issues/977)) ([3d70170](https://github.com/mykso/myks/commit/3d70170))

---
This PR was generated with [Release Please](https://github.com/googleapis/release-please).`

func TestCheckReleaseNote(t *testing.T) {
	tests := []struct {
		name, title, author, body, wantProblem string
	}{
		{"feature with a note", "feat(init): add --argocd", "someone", "## Summary\nx\n\n## Release note\n\n`myks init --argocd` adds Argo CD.\n\n## Evidence\ny", ""},
		{"none is enough", "fix: typo", "someone", "## Release note\nNone.", ""},
		{"breaking perf", "perf!: faster", "someone", "no section", "add a"},
		{"missing section", "fix(plugins): x", "someone", "## Summary\nx", "add a"},
		{"template comment only", "feat: x", "someone", "## Release note\n\n<!-- What a user notices -->\n\n## Summary", "is empty"},
		{"details block", "feat: x", "someone", "## Release note\n<details><summary>more</summary>x</details>", "<details>"},
		{"chore needs nothing", "chore(main): release 5.17.0", "mykso-bot", "", ""},
		{"docs need nothing", "docs: fix a link", "someone", "", ""},
		{"renovate is exempt", "fix(deps): update module x to v2", "renovate[bot]", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			problem := checkReleaseNote(tt.title, tt.author, tt.body)
			if tt.wantProblem == "" {
				assert.Empty(t, problem)
			} else {
				assert.Contains(t, problem, tt.wantProblem)
			}
		})
	}
}

func TestReleaseNote(t *testing.T) {
	tests := []struct {
		name, body, want string
		found            bool
	}{
		{"ends at the next section", "## Release note\nOne.\n\nTwo.\n## Evidence\nx", "One.\n\nTwo.", true},
		{"keeps subheadings and code", "## release note\r\nText.\r\n### Example\r\n```sh\r\n## not a heading\r\nmyks render\r\n```\r\n# Other", "Text.\n### Example\n```sh\n## not a heading\nmyks render\n```", true},
		{"drops comments", "## Release note\n<!-- hint\nspanning lines -->\nText.", "Text.", true},
		{"stops at the agent footer", "## Release note\n\nText.\n\n🤖 Generated with [Claude Code](https://claude.com/claude-code)\n\nhttps://claude.ai/code/session_x", "Text.", true},
		{"stops at a thematic break", "## Release note\nText.\n\n---\nSigned-off-by: someone", "Text.", true},
		{"keeps a break inside code", "## Release note\n```yaml\n---\na: 1\n```", "```yaml\n---\na: 1\n```", true},
		{"missing", "## Summary\nx", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, found := releaseNote(tt.body)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.found, found)
		})
	}
}

func TestPRNumbers(t *testing.T) {
	assert.Equal(t, []int{976, 975, 977}, prNumbers(releaseBody+"\n"+releaseBody))
}

func TestRender(t *testing.T) {
	entries := []entry{
		{number: 977, title: "fix: do not panic", note: "Invalid `min-version` values no longer crash myks."},
		{number: 976, title: "feat(init): add --ci github", note: "`myks init --ci github` writes CI workflows.\nThey render pull requests.\n\nSecond paragraph."},
		{number: 975, title: "feat(init): scaffold dev and prod", note: "```shell\nmyks init\n```\nA smaller scaffold.", highlight: true},
		{number: 974, title: "feat: internal", note: "none"},
		{number: 973, title: "refactor: tidy", note: "Rendering is unchanged."},
		{number: 972, title: "feat: no note"},
		{number: 971, title: "fix: sneaky", note: "<details>x</details>"},
	}
	got := render(entries, "https://github.com/mykso/myks")
	want := startMarker + `
### Highlights

` + "```shell\nmyks init\n```\nA smaller scaffold." + `

([#975](https://github.com/mykso/myks/pull/975))

` + "`myks init --ci github` writes CI workflows.\nThey render pull requests. ([#976](https://github.com/mykso/myks/pull/976))\n\nSecond paragraph." + `

` + "Invalid `min-version` values no longer crash myks. ([#977](https://github.com/mykso/myks/pull/977))" + `

Rendering is unchanged. ([#973](https://github.com/mykso/myks/pull/973))
` + endMarker
	assert.Equal(t, want, got)
	assert.Empty(t, render(entries[3:4], "https://github.com/mykso/myks"))
}

func TestApplyHighlights(t *testing.T) {
	block := startMarker + "\n### Highlights\n\nNew. ([#975](https://github.com/mykso/myks/pull/975))\n" + endMarker

	once, err := applyHighlights(releaseBody, block)
	require.NoError(t, err)
	// release-please takes the notes between the first and last "---" line and needs the
	// version heading to be their first line.
	notes := strings.TrimSpace(strings.SplitN(once, "\n---\n", 2)[1])
	assert.True(t, strings.HasPrefix(notes, "## [5.17.0]"), notes)
	assert.Contains(t, once, "(2026-11-04)\n\n"+block+"\n\n\n### Features")

	twice, err := applyHighlights(once, block)
	require.NoError(t, err)
	assert.Equal(t, once, twice, "applying the same block again changes nothing")

	removed, err := applyHighlights(once, "")
	require.NoError(t, err)
	assert.Equal(t, releaseBody, removed)

	_, err = applyHighlights("no heading here", block)
	assert.Error(t, err)
}
