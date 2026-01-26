package config

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// YAMLLoader loads and validates YAML devnet definitions
type YAMLLoader struct{}

// NewYAMLLoader creates a new YAML loader
func NewYAMLLoader() *YAMLLoader {
	return &YAMLLoader{}
}

// LoadFile loads devnet definitions from a YAML file
// Supports multi-document YAML (separated by ---)
func (l *YAMLLoader) LoadFile(path string) ([]YAMLDevnet, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()

	return l.LoadReader(f, path)
}

// LoadReader loads devnet definitions from a reader
func (l *YAMLLoader) LoadReader(r io.Reader, source string) ([]YAMLDevnet, error) {
	decoder := yaml.NewDecoder(r)
	var devnets []YAMLDevnet

	docIndex := 0
	for {
		var devnet YAMLDevnet
		err := decoder.Decode(&devnet)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to decode YAML document %d in %s: %w", docIndex, source, err)
		}

		// Validate each document
		if err := devnet.Validate(); err != nil {
			return nil, fmt.Errorf("validation failed for document %d in %s: %w", docIndex, source, err)
		}

		devnets = append(devnets, devnet)
		docIndex++
	}

	if len(devnets) == 0 {
		return nil, fmt.Errorf("no devnet definitions found in %s", source)
	}

	return devnets, nil
}

// LoadDirectory loads all YAML files from a directory
func (l *YAMLLoader) LoadDirectory(dir string) ([]YAMLDevnet, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	var allDevnets []YAMLDevnet

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		ext := filepath.Ext(name)
		if ext != ".yaml" && ext != ".yml" {
			continue
		}

		path := filepath.Join(dir, name)
		devnets, err := l.LoadFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to load %s: %w", path, err)
		}

		allDevnets = append(allDevnets, devnets...)
	}

	if len(allDevnets) == 0 {
		return nil, fmt.Errorf("no devnet definitions found in %s", dir)
	}

	return allDevnets, nil
}

// Load loads from a path (file or directory)
func (l *YAMLLoader) Load(path string) ([]YAMLDevnet, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to stat path: %w", err)
	}

	if info.IsDir() {
		return l.LoadDirectory(path)
	}
	return l.LoadFile(path)
}
