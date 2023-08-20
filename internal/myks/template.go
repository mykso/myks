package myks

import (
	"bytes"
	_ "embed"

	"github.com/rs/zerolog/log"
)

//go:embed templates/vendir/secret.ytt.yaml
var vendirSecretTemplate []byte

func writeSecretFile(secretName string, secretFilePath string, username string, password string) error {
	res, err := runYttWithFilesAndStdin(
		nil,
		bytes.NewReader(vendirSecretTemplate),
		func(name string, args []string) {
			log.Debug().Msg(msgRunCmd("render vendir secret yaml", name, args))
		},
		"--data-value=secret_name="+secretName,
		"--data-value=username="+username,
		"--data-value=password="+password,
	)
	if err != nil {
		log.Error().Err(err).Msg(res.Stderr)
		return err
	}

	err = writeFile(secretFilePath, []byte(res.Stdout))
	if err != nil {
		return err
	}
	return nil
}
