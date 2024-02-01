package splitcarfetcher

import (
	"fmt"
	"os"

	"github.com/anjor/carlet"
	"gopkg.in/yaml.v2"
)

type Metadata struct {
	CarPieces *carlet.CarPiecesAndMetadata `yaml:"car_pieces_meta"`
}

func MetadataFromYaml(path string) (*Metadata, error) {
	var meta Metadata

	metadataFileContent, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read pieces metadata file: %w", err)
	}

	// read the yaml file
	err = yaml.Unmarshal(metadataFileContent, &meta)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal pieces metadata: %w", err)
	}
	return &meta, nil
}
