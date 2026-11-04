// Command release-highlights keeps the "Highlights" of the release-please pull request in step
// with the "## Release note" sections of the pull requests it releases.
//
//	release-highlights check            # PR_TITLE, PR_AUTHOR, PR_BODY: fail without a release note
//	release-highlights apply [-dry-run] # rewrite the highlights of the open release pull request
//
// apply runs gh, so it needs GH_TOKEN (or a gh login) and runs inside a clone of the repository.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// releaseBranch is the branch release-please opens its pull request from.
const releaseBranch = "release-please--branches--main"

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: release-highlights check | apply [-dry-run]")
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "check":
		err = check()
	case "apply":
		fs := flag.NewFlagSet("apply", flag.ExitOnError)
		dryRun := fs.Bool("dry-run", false, "print the new body instead of editing the pull request")
		_ = fs.Parse(os.Args[2:])
		err = apply(*dryRun)
	default:
		err = fmt.Errorf("unknown command %q", os.Args[1])
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "release-highlights:", err)
		os.Exit(1)
	}
}

func check() error {
	if problem := checkReleaseNote(os.Getenv("PR_TITLE"), os.Getenv("PR_AUTHOR"), os.Getenv("PR_BODY")); problem != "" {
		fmt.Printf("::error title=Release note::%s\n", problem)
		return errors.New(problem)
	}
	return nil
}

type pullRequest struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	Body   string `json:"body"`
	Labels []struct {
		Name string `json:"name"`
	} `json:"labels"`
}

func apply(dryRun bool) error {
	var open []pullRequest
	if err := ghJSON(&open, "pr", "list", "--head", releaseBranch, "--state", "open", "--json", "number,body"); err != nil {
		return err
	}
	if len(open) == 0 {
		fmt.Println("no open release pull request")
		return nil
	}
	release := open[0]

	var repo struct {
		URL string `json:"url"`
	}
	if err := ghJSON(&repo, "repo", "view", "--json", "url"); err != nil {
		return err
	}

	var entries []entry
	for _, n := range prNumbers(release.Body) {
		var pr pullRequest
		if err := ghJSON(&pr, "pr", "view", fmt.Sprint(n), "--json", "number,title,body,labels"); err != nil {
			return err
		}
		note, _ := releaseNote(pr.Body)
		e := entry{number: n, title: pr.Title, note: note}
		for _, l := range pr.Labels {
			e.highlight = e.highlight || l.Name == "highlight"
		}
		entries = append(entries, e)
	}

	body, err := applyHighlights(release.Body, render(entries, repo.URL))
	if err != nil {
		return err
	}
	switch {
	case dryRun:
		fmt.Print(body)
		return nil
	case body == release.Body:
		fmt.Printf("release pull request #%d is up to date\n", release.Number)
		return nil
	}
	cmd := exec.Command("gh", "pr", "edit", fmt.Sprint(release.Number), "--body-file", "-") // #nosec G204 -- fixed arguments and a PR number
	cmd.Stdin, cmd.Stdout, cmd.Stderr = strings.NewReader(body), os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("editing #%d: %w", release.Number, err)
	}
	fmt.Printf("updated the highlights of release pull request #%d\n", release.Number)
	return nil
}

func ghJSON(v any, args ...string) error {
	var stderr bytes.Buffer
	cmd := exec.Command("gh", args...) // #nosec G204 -- callers pass fixed arguments and PR numbers
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("gh %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	if err := json.Unmarshal(out, v); err != nil {
		return fmt.Errorf("gh %s: %w", strings.Join(args, " "), err)
	}
	return nil
}
