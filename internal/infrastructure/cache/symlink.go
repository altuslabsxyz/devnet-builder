package cache

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/altuslabsxyz/devnet-builder/internal/paths"
	"github.com/altuslabsxyz/devnet-builder/pkg/network"
)

type SymlinkManager struct {
	homeDir     string
	binaryName  string
	symlinkPath string
}

func NewSymlinkManager(homeDir, binaryName string) *SymlinkManager {
	if binaryName == "" {
		binaryName = paths.DefaultBinaryName
	}
	return &SymlinkManager{
		homeDir:     homeDir,
		binaryName:  binaryName,
		symlinkPath: paths.BinarySymlinkPath(homeDir, binaryName),
	}
}

func (m *SymlinkManager) SymlinkPath() string {
	return m.symlinkPath
}

// Switch atomically switches the symlink using the atomic rename pattern.
func (m *SymlinkManager) Switch(targetPath string) error {
	binDir := filepath.Dir(m.symlinkPath)
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return fmt.Errorf("failed to create bin directory: %w", err)
	}

	tempLink := m.symlinkPath + ".tmp"
	os.Remove(tempLink)

	if err := os.Symlink(targetPath, tempLink); err != nil {
		return fmt.Errorf("failed to create temporary symlink: %w", err)
	}

	if err := os.Rename(tempLink, m.symlinkPath); err != nil {
		os.Remove(tempLink)
		return fmt.Errorf("failed to atomic rename symlink: %w", err)
	}

	return nil
}

func (m *SymlinkManager) SwitchToCache(cache *BinaryCache, cacheKey string) error {
	relativePath := paths.RelativeCacheTargetPath(cacheKey, m.binaryName)
	return m.Switch(relativePath)
}

func (m *SymlinkManager) SwitchToCacheWithConfig(cache *BinaryCache, networkType, commitHash string, buildConfig *network.BuildConfig) error {
	cacheKey := MakeCacheKey(networkType, commitHash, buildConfig)
	return m.SwitchToCache(cache, cacheKey)
}
