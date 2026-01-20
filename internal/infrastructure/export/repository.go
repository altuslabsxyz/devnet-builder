package export

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	domainExport "github.com/altuslabsxyz/devnet-builder/internal/domain/export"
	"github.com/altuslabsxyz/devnet-builder/internal/paths"
	"github.com/altuslabsxyz/devnet-builder/types"
)

// Repository implements ports.ExportRepository for file-based persistence.
type Repository struct {
	baseDir string // Base directory for exports (e.g., ~/.stable-devnet/devnet-xyz/exports)
}

// NewRepository creates a new export repository.
func NewRepository(baseDir string) *Repository {
	return &Repository{
		baseDir: baseDir,
	}
}

// Save persists export metadata to disk.
func (r *Repository) Save(ctx context.Context, exp interface{}) error {
	export, ok := exp.(*domainExport.Export)
	if !ok {
		return fmt.Errorf("expected *export.Export, got %T", exp)
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Validate export before saving
	if err := export.Validate(); err != nil {
		return fmt.Errorf("export validation failed: %w", err)
	}

	// Ensure export directory exists
	if err := os.MkdirAll(export.DirectoryPath, 0755); err != nil {
		return fmt.Errorf("failed to create export directory: %w", err)
	}

	// Serialize metadata to JSON
	metadataJSON, err := export.Metadata.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize metadata: %w", err)
	}

	// Write metadata.json
	metadataPath := export.GetMetadataPath()
	if err := os.WriteFile(metadataPath, metadataJSON, 0644); err != nil {
		return fmt.Errorf("failed to write metadata file: %w", err)
	}

	return nil
}

// Load retrieves export metadata from a directory.
func (r *Repository) Load(ctx context.Context, exportPath string) (interface{}, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	if exportPath == "" {
		return nil, fmt.Errorf("export path cannot be empty")
	}

	// Read metadata.json
	metadataPath := filepath.Join(exportPath, paths.MetadataFile)
	metadataJSON, err := os.ReadFile(metadataPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, domainExport.ErrExportNotFound
		}
		return nil, fmt.Errorf("failed to read metadata: %w", err)
	}

	// Deserialize metadata
	metadata, err := domainExport.FromJSON(metadataJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to parse metadata: %w", err)
	}

	// Reconstruct BinaryInfo
	binaryInfo, err := domainExport.NewBinaryInfo(
		metadata.BinaryPath,
		metadata.DockerImage,
		metadata.BinaryHash,
		metadata.BinaryVersion,
		types.ExecutionMode(metadata.ExecutionMode),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create BinaryInfo: %w", err)
	}

	// Determine genesis file path
	genesisFileName := fmt.Sprintf("genesis-%d-%s.json", metadata.BlockHeight, metadata.BinaryHashPrefix)
	genesisPath := filepath.Join(exportPath, genesisFileName)

	// Create Export entity
	export, err := domainExport.NewExport(
		exportPath,
		metadata.ExportTimestamp,
		metadata.BlockHeight,
		metadata.NetworkSource,
		binaryInfo,
		metadata,
		genesisPath,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Export: %w", err)
	}

	return export, nil
}

// ListForDevnet lists all exports for a given devnet home directory.
func (r *Repository) ListForDevnet(ctx context.Context, devnetHomeDir string) (interface{}, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	exportsDir := paths.ExportsPath(devnetHomeDir)

	// Check if exports directory exists
	if _, err := os.Stat(exportsDir); os.IsNotExist(err) {
		// No exports directory means no exports
		return []*domainExport.Export{}, nil
	}

	// Read all entries in exports directory
	entries, err := os.ReadDir(exportsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read exports directory: %w", err)
	}

	var exports []*domainExport.Export

	// Load each export
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		exportPath := filepath.Join(exportsDir, entry.Name())

		// Load export metadata
		exp, err := r.Load(ctx, exportPath)
		if err != nil {
			// Skip invalid exports
			continue
		}

		export, ok := exp.(*domainExport.Export)
		if !ok {
			continue
		}

		exports = append(exports, export)
	}

	// Sort by timestamp (newest first)
	sort.Slice(exports, func(i, j int) bool {
		return exports[i].ExportTimestamp.After(exports[j].ExportTimestamp)
	})

	return exports, nil
}

// Delete removes an export directory and all its contents.
func (r *Repository) Delete(ctx context.Context, exportPath string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if exportPath == "" {
		return fmt.Errorf("export path cannot be empty")
	}

	// Verify it's a valid export directory
	if _, err := r.Load(ctx, exportPath); err != nil {
		if err == domainExport.ErrExportNotFound {
			return err
		}
		// Still try to delete even if metadata is invalid
	}

	// Remove directory and all contents
	if err := os.RemoveAll(exportPath); err != nil {
		return fmt.Errorf("failed to delete export: %w", err)
	}

	return nil
}

// Validate checks if an export is complete and valid.
func (r *Repository) Validate(ctx context.Context, exportPath string) (interface{}, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Load the export
	exp, err := r.Load(ctx, exportPath)
	if err != nil {
		return nil, err
	}

	export, ok := exp.(*domainExport.Export)
	if !ok {
		return nil, fmt.Errorf("invalid export type")
	}

	// Check completeness
	var missingFiles []string

	// Check metadata.json
	metadataPath := export.GetMetadataPath()
	if _, err := os.Stat(metadataPath); os.IsNotExist(err) {
		missingFiles = append(missingFiles, "metadata.json")
	}

	// Check genesis file
	if _, err := os.Stat(export.GenesisFilePath); os.IsNotExist(err) {
		missingFiles = append(missingFiles, filepath.Base(export.GenesisFilePath))
	}

	result := &ValidationResult{
		Export:       export,
		IsComplete:   len(missingFiles) == 0,
		MissingFiles: missingFiles,
	}

	if !result.IsComplete {
		return result, domainExport.ErrExportIncomplete
	}

	return result, nil
}

// ValidationResult contains the result of export validation.
type ValidationResult struct {
	Export       *domainExport.Export
	IsComplete   bool
	MissingFiles []string
}

// GetExportsDirectory returns the exports directory path for a devnet.
func (r *Repository) GetExportsDirectory(devnetHomeDir string) string {
	return paths.ExportsPath(devnetHomeDir)
}

// GenerateExportPath generates the full export directory path.
func (r *Repository) GenerateExportPath(devnetHomeDir string, export *domainExport.Export) string {
	exportsDir := r.GetExportsDirectory(devnetHomeDir)
	return filepath.Join(exportsDir, export.DirectoryName())
}

// ParseDirectoryName attempts to parse an export directory name.
// Returns height, timestamp, and hash prefix if valid.
func ParseDirectoryName(dirName string) (network string, hashPrefix string, height int64, timestamp time.Time, err error) {
	// Expected format: {network}-{hash}-{height}-{timestamp}
	// Example: mainnet-abc12345-1000000-20240115120000
	parts := strings.Split(dirName, "-")
	if len(parts) < 4 {
		err = fmt.Errorf("invalid directory name format: %s", dirName)
		return
	}

	network = parts[0]
	hashPrefix = parts[1]

	// Parse height
	if _, scanErr := fmt.Sscanf(parts[2], "%d", &height); scanErr != nil {
		err = fmt.Errorf("invalid height in directory name: %s", parts[2])
		return
	}

	// Parse timestamp
	timestampStr := parts[3]
	timestamp, err = time.Parse("20060102150405", timestampStr)
	if err != nil {
		err = fmt.Errorf("invalid timestamp in directory name: %s", timestampStr)
		return
	}

	return
}
