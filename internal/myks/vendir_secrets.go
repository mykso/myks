package myks

import (
	"bytes"
	_ "embed"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/rs/zerolog/log"
)

//go:embed templates/vendir/secret.ytt.yaml
var vendirSecretTemplate []byte

func (g *Globe) collectVendirSecrets() map[string]*VendirCredentials {
	vendirCredentials := make(map[string]*VendirCredentials)

	usrRgx := regexp.MustCompile("^" + g.VendirSecretEnvPrefix + "(.+)_USERNAME=(.*)$")
	pswRgx := regexp.MustCompile("^" + g.VendirSecretEnvPrefix + "(.+)_PASSWORD=(.*)$")

	envvars := os.Environ()
	// Sort envvars to produce deterministic output for testing
	sort.Strings(envvars)
	for _, envPair := range envvars {
		if usrRgx.MatchString(envPair) {
			match := usrRgx.FindStringSubmatch(envPair)
			secretName := strings.ToLower(match[1])
			username := match[2]
			if vendirCredentials[secretName] == nil {
				vendirCredentials[secretName] = &VendirCredentials{}
			}
			vendirCredentials[secretName].Username = username
		} else if pswRgx.MatchString(envPair) {
			match := pswRgx.FindStringSubmatch(envPair)
			secretName := strings.ToLower(match[1])
			password := match[2]
			if vendirCredentials[secretName] == nil {
				vendirCredentials[secretName] = &VendirCredentials{}
			}
			vendirCredentials[secretName].Password = password
		}
	}

	for secretName, credentials := range vendirCredentials {
		if credentials.Username == "" || credentials.Password == "" {
			log.Warn().Msg("Incomplete credentials for secret: " + secretName)
			delete(vendirCredentials, secretName)
		}
	}

	var secretNames []string
	for secretName := range vendirCredentials {
		secretNames = append(secretNames, secretName)
	}
	log.Debug().Msg(g.Msg("Found vendir secrets: " + strings.Join(secretNames, ", ")))

	return vendirCredentials
}

func (g *Globe) generateVendirSecretYamls() (string, error) {
	vendirCredentials := g.collectVendirSecrets()

	// sort secret names to produce deterministic output for testing
	var secretNames []string
	for secretName := range vendirCredentials {
		secretNames = append(secretNames, secretName)
	}
	sort.Strings(secretNames)

	var secretYamls string
	for _, secretName := range secretNames {
		credentials := vendirCredentials[secretName]
		secretYaml, err := g.generateVendirSecretYaml(secretName, credentials.Username, credentials.Password)
		if err != nil {
			return secretYamls, err
		}
		secretYamls += "---\n" + secretYaml
	}

	return secretYamls, nil
}

func (g *Globe) generateVendirSecretYaml(secretName string, username string, password string) (string, error) {
	res, err := runYttWithFilesAndStdin(
		nil,
		bytes.NewReader(vendirSecretTemplate),
		func(name string, args []string) {
			log.Debug().Msg(g.Msg(msgRunCmd("render vendir secret yaml", name, args)))
		},
		"--data-value=secret_name="+secretName,
		"--data-value=username="+username,
		"--data-value=password="+password,
	)
	if err != nil {
		log.Error().Err(err).Msg(g.Msg(res.Stderr))
		return "", err
	}

	return res.Stdout, nil
}
