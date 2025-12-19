//go:build !private

package devnet

import "errors"

// ErrPrivateBuildRequired is returned when private build tag is required.
var ErrPrivateBuildRequired = errors.New("this feature requires building with '-tags=private' and access to stablelabs private repositories")

// ExportKeys exports validator and account keys from the devnet.
// This is a stub implementation that returns an error when built without private tag.
func ExportKeys(homeDir string, keyType string) (*KeyExport, error) {
	return nil, ErrPrivateBuildRequired
}
