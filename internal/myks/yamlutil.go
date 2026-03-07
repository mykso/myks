package myks

import (
	"bytes"
	"os"

	yaml "gopkg.in/yaml.v3"
)

func unmarshalYamlToMap(filePath string) (map[string]any, error) {
	ok, err := isExist(filePath)
	if err != nil {
		return nil, err
	}
	if !ok {
		return make(map[string]any), nil
	}

	file, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var config map[string]any
	err = yaml.Unmarshal(file, &config)
	if err != nil {
		return nil, err
	}
	return config, nil
}

func sortYaml(content []byte) ([]byte, error) {
	var obj map[string]any
	if err := yaml.Unmarshal(content, &obj); err != nil {
		return nil, err
	}

	var data bytes.Buffer
	enc := yaml.NewEncoder(&data)
	enc.SetIndent(2)
	err := enc.Encode(obj)
	if err != nil {
		return nil, err
	}
	return data.Bytes(), nil
}
