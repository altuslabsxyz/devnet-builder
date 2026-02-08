// cmd/dvb/errors.go
package main

import "fmt"

// errDaemonNotRunning is the standard error returned when daemon connection is required but unavailable.
var errDaemonNotRunning = fmt.Errorf("daemon not running - start with: devnetd")

// requireDaemon returns errDaemonNotRunning if the daemon client is not connected.
// Usage: if err := requireDaemon(); err != nil { return err }
func requireDaemon() error {
	if daemonClient == nil {
		return errDaemonNotRunning
	}
	return nil
}
