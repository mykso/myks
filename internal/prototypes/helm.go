package prototypes

/*
Allows to interact with helm to improve myks src add completion.
Uunfortunately the response time is slow for name and version completion as we cannot cache the context between completions
	> time helm search repo oncall -o yaml|yq ".[0].version"
	1.10.0
	helm search repo oncall -o yaml  4,00s user 0,24s system 130% cpu 3,254 total
	yq ".[0].version"  0,01s user 0,01s system 0% cpu 3,254 total

So it's either better to read helm cache directory (a bit faster)
	> time yq e ".entries.oncall[0].version" $(grep -l "oncall:" /home/kbudde/.cache/helm/repository/*-index.yaml)
	1.10.0
	yq e ".entries.oncall[0].version"   0,98s user 0,11s system 138% cpu 0,779 total

or don't use completions:
   allow incomplete add command "myks proto src add _PROTO_ -n grafana"
   load the missing parameters from helm -> faster as we have better context, still slow but not so annoying
*/
import (
	"encoding/json"
	"os/exec"
	"strings"
)

type HelmClient struct {
}

// RepoUrls returns a list of helm repository URLs
// executes `helm repo list -o json` and returns the list of URLs
func (hc *HelmClient) RepoUrls() ([]string, error) {
	repos := []string{}

	cmd := exec.Command("helm", "repo", "list", "-o", "json")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var repoList []struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(output, &repoList); err != nil {
		return nil, err
	}

	for _, repo := range repoList {
		repos = append(repos, repo.URL)
	}

	return repos, nil
}

// Charts returns a list of helm charts
// executes `helm search repo -o json` and returns the list of charts
func (hc *HelmClient) Charts(search string) ([]string, error) {
	charts := []string{}

	cmd := exec.Command("helm", "search", "repo", "-o", "json", search)
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var chartList []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(output, &chartList); err != nil {
		return nil, err
	}

	for _, chart := range chartList {
		if _, chart, found := strings.Cut(chart.Name, "/"); found { // drop leading repo name
			charts = append(charts, chart)
		}
	}

	return charts, nil
}

func (hc *HelmClient) RepoName(url string) (string, error) {
	cmd := exec.Command("helm", "repo", "list", "-o", "json")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	var repoList []struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	}
	if err := json.Unmarshal(output, &repoList); err != nil {
		return "", err
	}

	for _, repo := range repoList {
		if repo.URL == url {
			return repo.Name, nil
		}
	}

	return "", nil
}

// ChartVersion returns helm chart version. Usually it should be only one result if url is specified
// executes `helm search repo -o json _chart_` and returns the list of versions
func (hc *HelmClient) ChartVersion(url string, chart string) ([]string, error) {
	var repo string
	var err error
	if len(url) > 0 {
		repo, err = hc.RepoName(url)
		if err != nil {
			return nil, err
		}
	}

	versions := []string{}

	search := chart
	if repo != "" {
		search = repo + "/" + chart
	}

	cmd := exec.Command("helm", "search", "repo", "-o", "json", search)
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var versionList []struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(output, &versionList); err != nil {
		return nil, err
	}

	for _, version := range versionList {
		versions = append(versions, version.Version)
	}
	return versions, nil
}
