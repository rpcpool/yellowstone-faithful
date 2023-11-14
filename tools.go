package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

func isDirectory(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	return info.IsDir(), nil
}

// exists checks whether a file or directory exists.
func exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		// file does not exist
		return false, nil
	}
	// other error
	return false, err
}

// isFile checks whether a path is a file.
func isFile(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	return !info.IsDir(), nil
}

// isJSONFile checks whether a path is a JSON file.
func isJSONFile(filepath string) bool {
	return filepath[len(filepath)-5:] == ".json"
}

// isYAMLFile checks whether a path is a YAML file.
func isYAMLFile(filepath string) bool {
	return filepath[len(filepath)-5:] == ".yaml" || filepath[len(filepath)-4:] == ".yml"
}

// loadFromJSON loads a JSON file into dst (which must be a pointer).
func loadFromJSON(configFilepath string, dst any) error {
	file, err := os.Open(configFilepath)
	if err != nil {
		return fmt.Errorf("failed to open config file: %w", err)
	}
	defer file.Close()
	return json.NewDecoder(file).Decode(dst)
}

// loadFromYAML loads a YAML file into dst (which must be a pointer).
func loadFromYAML(configFilepath string, dst any) error {
	file, err := os.Open(configFilepath)
	if err != nil {
		return fmt.Errorf("failed to open config file: %w", err)
	}
	defer file.Close()

	return yaml.NewDecoder(file).Decode(dst)
}

// btoi converts a byte slice of length 8 to a uint64.
func btoi(b []byte) uint64 {
	return binary.LittleEndian.Uint64(b)
}

// itob converts a uint64 to a byte slice of length 8.
func itob(v uint64) []byte {
	var buf [8]byte
	binary.LittleEndian.PutUint64(buf[:], v)
	return buf[:]
}
