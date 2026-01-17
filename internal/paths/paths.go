// Package paths provides centralized path management for devnet-builder.
package paths

import (
	"fmt"
	"os"
	"path/filepath"
)

// Directory constants relative to home directory.
const (
	DevnetDir      = "devnet"
	BinDir         = "bin"
	CacheDir       = "cache"
	BinaryCacheDir = "cache/binaries"
	SnapshotsDir   = "snapshots"
	ExportsDir     = "exports"
	PluginsDir     = "plugins"
)

// Node subdirectory constants.
const (
	ConfigDir  = "config"
	DataDir    = "data"
	KeyringDir = "keyring-test"
)

// File name constants.
const (
	ConfigFile        = "config.toml"
	AppConfigFile     = "app.toml"
	GenesisFile       = "genesis.json"
	MetadataFile      = "metadata.json"
	SnapshotMetaFile  = "snapshot.meta.json"
	DefaultBinaryName = "binary"
)

// Cache key format constants.
const (
	CommitHashLength  = 40
	ConfigHashLength  = 8
	CacheKeySeparator = "-"
)

const DefaultHomeDirName = ".devnet-builder"

// DefaultHomeDir returns $HOME/.devnet-builder or falls back to current directory.
func DefaultHomeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return DefaultHomeDirName
	}
	return filepath.Join(home, DefaultHomeDirName)
}

// Devnet paths

func DevnetPath(homeDir string) string {
	return filepath.Join(homeDir, DevnetDir)
}

func NodePath(homeDir string, index int) string {
	return filepath.Join(DevnetPath(homeDir), fmt.Sprintf("node%d", index))
}

func NodeConfigPath(homeDir string, index int) string {
	return filepath.Join(NodePath(homeDir, index), ConfigDir)
}

func NodeDataPath(homeDir string, index int) string {
	return filepath.Join(NodePath(homeDir, index), DataDir)
}

func NodeKeyringPath(homeDir string, index int) string {
	return filepath.Join(NodePath(homeDir, index), KeyringDir)
}

func NodeConfigTomlPath(homeDir string, index int) string {
	return filepath.Join(NodeConfigPath(homeDir, index), ConfigFile)
}

func NodeAppTomlPath(homeDir string, index int) string {
	return filepath.Join(NodeConfigPath(homeDir, index), AppConfigFile)
}

func NodeGenesisPath(homeDir string, index int) string {
	return filepath.Join(NodeConfigPath(homeDir, index), GenesisFile)
}

func DevnetGenesisPath(homeDir string) string {
	return filepath.Join(DevnetPath(homeDir), GenesisFile)
}

func DevnetMetadataPath(homeDir string) string {
	return filepath.Join(DevnetPath(homeDir), MetadataFile)
}

func DevnetAccountsPath(homeDir string) string {
	return filepath.Join(DevnetPath(homeDir), "accounts")
}

func NodePIDPath(nodeHomeDir, binaryName string) string {
	return filepath.Join(nodeHomeDir, binaryName+".pid")
}

func NodeLogPath(nodeHomeDir, binaryName string) string {
	return filepath.Join(nodeHomeDir, binaryName+".log")
}

// Binary cache paths

func BinaryCachePath(homeDir string) string {
	return filepath.Join(homeDir, BinaryCacheDir)
}

func BinaryCacheNetworkPath(homeDir, networkType string) string {
	return filepath.Join(BinaryCachePath(homeDir), networkType)
}

func BinaryCacheKeyPath(homeDir, networkType, cacheKey string) string {
	return filepath.Join(BinaryCacheNetworkPath(homeDir, networkType), cacheKey)
}

func CachedBinaryPath(homeDir, networkType, cacheKey, binaryName string) string {
	return filepath.Join(BinaryCacheKeyPath(homeDir, networkType, cacheKey), binaryName)
}

func CachedBinaryMetadataPath(homeDir, networkType, cacheKey string) string {
	return filepath.Join(BinaryCacheKeyPath(homeDir, networkType, cacheKey), MetadataFile)
}

func BuildCacheKey(commitHash, configHash string) string {
	return commitHash + CacheKeySeparator + configHash
}

// Binary symlink paths

func BinPath(homeDir string) string {
	return filepath.Join(homeDir, BinDir)
}

func BinarySymlinkPath(homeDir, binaryName string) string {
	return filepath.Join(BinPath(homeDir), binaryName)
}

// RelativeCacheTargetPath returns relative path from bin dir to cache entry.
// cacheKey format: {networkType}/{commitHash}-{configHash}
func RelativeCacheTargetPath(cacheKey, binaryName string) string {
	return filepath.Join("..", BinaryCacheDir, cacheKey, binaryName)
}

// Snapshot paths

func SnapshotCachePath(homeDir string) string {
	return filepath.Join(homeDir, SnapshotsDir)
}

func SnapshotCacheKeyPath(homeDir, cacheKey string) string {
	return filepath.Join(SnapshotCachePath(homeDir), cacheKey)
}

func SnapshotFilePath(homeDir, cacheKey, extension string) string {
	return filepath.Join(SnapshotCacheKeyPath(homeDir, cacheKey), "snapshot"+extension)
}

func SnapshotMetadataPath(homeDir, cacheKey string) string {
	return filepath.Join(SnapshotCacheKeyPath(homeDir, cacheKey), SnapshotMetaFile)
}

// Export paths

func ExportsPath(homeDir string) string {
	return filepath.Join(homeDir, ExportsDir)
}

func ExportPath(homeDir, exportName string) string {
	return filepath.Join(ExportsPath(homeDir), exportName)
}

func ExportMetadataPath(homeDir, exportName string) string {
	return filepath.Join(ExportPath(homeDir, exportName), MetadataFile)
}

func ExportGenesisPath(homeDir, exportName, genesisFileName string) string {
	return filepath.Join(ExportPath(homeDir, exportName), genesisFileName)
}

func BuildExportName(networkType, commitHash string, blockHeight int64, timestamp string) string {
	return fmt.Sprintf("%s-%s-%d-%s", networkType, commitHash[:8], blockHeight, timestamp)
}

func BuildGenesisFileName(blockHeight int64, commitHash string) string {
	return fmt.Sprintf("genesis-%d-%s.json", blockHeight, commitHash[:8])
}

// Config path

func ConfigPath(homeDir string) string {
	return filepath.Join(homeDir, ConfigFile)
}

// Plugin paths

func PluginsPath(homeDir string) string {
	return filepath.Join(homeDir, PluginsDir)
}

// Validator/Account key paths

func ValidatorKeyPath(homeDir string, index int) string {
	return filepath.Join(NodePath(homeDir, index), fmt.Sprintf("validator%d.json", index))
}

func AccountKeyPath(homeDir string, index int) string {
	return filepath.Join(DevnetPath(homeDir), fmt.Sprintf("account%d.json", index))
}

// IsValidCacheKey validates cache key format: {commitHash(40hex)}-{configHash(8hex)}
func IsValidCacheKey(cacheKey string) bool {
	expectedLen := CommitHashLength + len(CacheKeySeparator) + ConfigHashLength
	if len(cacheKey) != expectedLen {
		return false
	}
	if cacheKey[CommitHashLength:CommitHashLength+1] != CacheKeySeparator {
		return false
	}
	return isHexString(cacheKey[:CommitHashLength]) &&
		isHexString(cacheKey[CommitHashLength+1:])
}

func isHexString(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

// Path existence helpers

func Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func IsDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func IsFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func IsSymlink(path string) bool {
	info, err := os.Lstat(path)
	return err == nil && info.Mode()&os.ModeSymlink != 0
}

func EnsureDir(path string) error {
	return os.MkdirAll(path, 0755)
}
