package devnet

import (
	"context"

	"github.com/b-harvest/devnet-builder/internal/node"
	"github.com/b-harvest/devnet-builder/internal/output"
)

// HealthService handles health checking operations for devnet nodes.
type HealthService struct {
	logger *output.Logger
}

// NewHealthService creates a new HealthService.
func NewHealthService(logger *output.Logger) *HealthService {
	if logger == nil {
		logger = output.DefaultLogger
	}
	return &HealthService{logger: logger}
}

// CheckHealth returns the health status of all nodes.
func (s *HealthService) CheckHealth(ctx context.Context, nodes []*node.Node) []*node.NodeHealth {
	return node.CheckAllNodesHealth(ctx, nodes)
}

// PrintFailedNodeLogs checks all nodes and prints log files for any that failed health checks.
func (s *HealthService) PrintFailedNodeLogs(ctx context.Context, nodes []*node.Node) {
	printFailedNodeLogs(ctx, nodes, s.logger)
}

// GetHealth returns the health status of all nodes in a devnet.
func (d *Devnet) GetHealth(ctx context.Context) []*node.NodeHealth {
	return node.CheckAllNodesHealth(ctx, d.Nodes)
}

// printFailedNodeLogs checks all nodes and prints log files for any that failed health checks.
func printFailedNodeLogs(ctx context.Context, nodes []*node.Node, logger *output.Logger) {
	healthResults := node.CheckAllNodesHealth(ctx, nodes)

	for i, health := range healthResults {
		// Only print logs for unhealthy nodes
		if health.Status == node.NodeStatusRunning || health.Status == node.NodeStatusSyncing {
			continue
		}

		n := nodes[i]
		logPath := n.LogFilePath()

		// Read last N lines from log file
		logLines, err := output.ReadLastLines(logPath, output.DefaultLogLines)

		errorInfo := &output.NodeErrorInfo{
			NodeName: n.Name,
			NodeDir:  n.HomeDir,
			LogPath:  logPath,
			LogLines: logLines,
		}

		// Add PID if available (for verbose mode)
		if n.PID != nil {
			errorInfo.PID = *n.PID
		}

		// Handle read errors gracefully - still show what we can
		if err != nil {
			switch err.(type) {
			case *output.FileNotFoundError:
				errorInfo.LogLines = []string{"(Log file not found: " + logPath + ")"}
			case *output.EmptyFileError:
				errorInfo.LogLines = []string{"(Log file is empty)"}
			case *output.PermissionDeniedError:
				errorInfo.LogLines = []string{"(Cannot read log file: permission denied)"}
			default:
				errorInfo.LogLines = []string{"(Error reading log file: " + err.Error() + ")"}
			}
		}

		logger.PrintNodeError(errorInfo)
	}
}
