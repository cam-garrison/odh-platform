package resource

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/opendatahub-io/odh-platform/pkg/spi"
)

type capabilities struct {
	Authorization [][]spi.AuthorizationComponent `json:"authorization"`
}

func LoadConfig(path string) ([]spi.AuthorizationComponent, error) {
	components := []spi.AuthorizationComponent{}

	/*
		dir, err := os.Open(path)
		if err != nil {
			return []spi.AuthorizationComponent{}, errors.Wrap(err, "could not read config directory")
		}

		files, err := dir.ReadDir(-1)
		if err != nil {
			return []spi.AuthorizationComponent{}, errors.Wrap(err, "could not read files from config directory")
		}

		for _, file := range files {
			if file.IsDir() {
				continue
			}
	*/
	content, err := os.ReadFile(path)
	if err != nil {
		return []spi.AuthorizationComponent{}, fmt.Errorf("could not read config file [%s]: %w", path, err)
	}

	var caps capabilities

	err = json.Unmarshal(content, &caps)
	if err != nil {
		return []spi.AuthorizationComponent{}, fmt.Errorf("could not parse json content of [%s]: %w", path, err)
	}

	for _, v := range caps.Authorization {
		components = append(components, v...)
	}

	return components, nil
}