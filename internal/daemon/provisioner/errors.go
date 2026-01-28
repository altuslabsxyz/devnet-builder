package provisioner

import "fmt"

// SnapshotVersionRequiredError is returned when snapshot mode is used without
// an explicit binary version. Snapshot forking requires the binary version to
// match the chain state schema exactly, otherwise genesis export will panic.
type SnapshotVersionRequiredError struct {
	DevnetName string
}

// Error implements the error interface.
func (e *SnapshotVersionRequiredError) Error() string {
	return fmt.Sprintf(`snapshot mode requires explicit binary version for devnet %q

When forking from a snapshot, the binary version must match the chain state schema.
Using the wrong version will cause the node to panic during genesis export.

To fix:
  1. Find compatible versions: dvb network versions <network-name>
  2. Set version in your devnet spec:
     binarySource:
       type: cache
       version: "v1.0.0"`, e.DevnetName)
}

// Is implements errors.Is interface for comparing error types.
func (e *SnapshotVersionRequiredError) Is(target error) bool {
	_, ok := target.(*SnapshotVersionRequiredError)
	return ok
}
