package manage

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/altuslabsxyz/devnet-builder/internal/application"
	"github.com/altuslabsxyz/devnet-builder/internal/output"
	"github.com/altuslabsxyz/devnet-builder/types"
	"github.com/altuslabsxyz/devnet-builder/types/ctxconfig"
	"github.com/spf13/cobra"
)

var (
	pfAddress string
)

// PortMapping represents a port forward mapping.
type PortMapping struct {
	LocalPort     int    `json:"local_port"`
	ContainerPort int    `json:"container_port"`
	NodeIndex     int    `json:"node_index"`
	Protocol      string `json:"protocol"`
}

// NewPortForwardCmd creates the port-forward command.
func NewPortForwardCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "port-forward [node] [LOCAL_PORT:]CONTAINER_PORT",
		Short: "Forward local ports to node container ports",
		Long: `Forward one or more local ports to a devnet node container.

Specify the node by index (0, 1, 2, 3) or name (node0, node1, node2, node3).
Port mappings can be specified as LOCAL_PORT:CONTAINER_PORT or just CONTAINER_PORT
(which uses the same port number for both).

Common Cosmos SDK ports:
  26656 - P2P port
  26657 - RPC port
  26660 - Prometheus metrics
  1317  - REST API
  9090  - gRPC

Examples:
  # Forward local port 8080 to container port 26657 on node0
  devnet-builder port-forward 0 8080:26657

  # Forward using the same local and container port
  devnet-builder port-forward node0 26657

  # Forward multiple ports
  devnet-builder port-forward 0 26657 1317 9090

  # Bind to a specific address
  devnet-builder port-forward 0 26657 --address 0.0.0.0`,
		Args: cobra.MinimumNArgs(2),
		RunE: runPortForward,
	}

	cmd.Flags().StringVar(&pfAddress, "address", "127.0.0.1",
		"Address to bind to (default: localhost only)")

	return cmd
}

func runPortForward(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	cfg := ctxconfig.FromContext(ctx)
	homeDir := cfg.HomeDir()

	svc, err := application.GetService(homeDir)
	if err != nil {
		return fmt.Errorf("failed to initialize service: %w", err)
	}

	// Check if devnet exists
	if !svc.DevnetExists() {
		return fmt.Errorf("no devnet found at %s", homeDir)
	}

	// Check if running in docker mode
	isDocker, err := svc.IsDockerMode(ctx)
	if err != nil {
		return err
	}
	if !isDocker {
		return fmt.Errorf("port-forward command only works in docker mode")
	}

	// Get number of validators
	numValidators, err := svc.GetNumValidators(ctx)
	if err != nil {
		return err
	}

	// Parse node argument
	nodeIndex, err := parseNodeArgPF(args[0], numValidators)
	if err != nil {
		return err
	}

	// Parse port mappings
	portMappings, err := parsePortMappings(args[1:], nodeIndex)
	if err != nil {
		return err
	}

	// Get container name
	modeInfo, err := svc.GetExecutionModeInfo(ctx, nodeIndex)
	if err != nil {
		return err
	}

	if modeInfo.Mode != types.ExecutionModeDocker {
		return fmt.Errorf("node%d is not running in docker mode", nodeIndex)
	}

	containerName := modeInfo.ContainerName
	if containerName == "" {
		return fmt.Errorf("container name not found for node%d", nodeIndex)
	}

	// Start port forwarding
	return startPortForwarding(ctx, containerName, portMappings)
}

// parseNodeArgPF parses a node argument into an index.
func parseNodeArgPF(arg string, numNodes int) (int, error) {
	if arg == "" {
		return 0, fmt.Errorf("node argument cannot be empty")
	}

	var index int
	var err error

	if strings.HasPrefix(arg, "node") {
		indexStr := strings.TrimPrefix(arg, "node")
		index, err = strconv.Atoi(indexStr)
	} else {
		index, err = strconv.Atoi(arg)
	}

	if err != nil {
		return 0, fmt.Errorf("invalid node: %s (expected 0-%d or node0-node%d)", arg, numNodes-1, numNodes-1)
	}

	if index < 0 || index >= numNodes {
		return 0, fmt.Errorf("invalid node: %s (expected 0-%d or node0-node%d)", arg, numNodes-1, numNodes-1)
	}

	return index, nil
}

// parsePortMappings parses port mapping arguments.
func parsePortMappings(args []string, nodeIndex int) ([]PortMapping, error) {
	var mappings []PortMapping

	for _, arg := range args {
		mapping, err := parsePortMapping(arg, nodeIndex)
		if err != nil {
			return nil, err
		}
		mappings = append(mappings, mapping)
	}

	return mappings, nil
}

// parsePortMapping parses a single port mapping (e.g., "8080:26657" or "26657").
func parsePortMapping(arg string, nodeIndex int) (PortMapping, error) {
	mapping := PortMapping{
		NodeIndex: nodeIndex,
		Protocol:  "tcp",
	}

	parts := strings.Split(arg, ":")
	switch len(parts) {
	case 1:
		// Just container port, use same for local
		port, err := strconv.Atoi(parts[0])
		if err != nil || port <= 0 || port > 65535 {
			return mapping, fmt.Errorf("invalid port: %s", arg)
		}
		mapping.LocalPort = port
		mapping.ContainerPort = port
	case 2:
		// local:container format
		localPort, err := strconv.Atoi(parts[0])
		if err != nil || localPort <= 0 || localPort > 65535 {
			return mapping, fmt.Errorf("invalid local port: %s", parts[0])
		}
		containerPort, err := strconv.Atoi(parts[1])
		if err != nil || containerPort <= 0 || containerPort > 65535 {
			return mapping, fmt.Errorf("invalid container port: %s", parts[1])
		}
		mapping.LocalPort = localPort
		mapping.ContainerPort = containerPort
	default:
		return mapping, fmt.Errorf("invalid port mapping format: %s (expected LOCAL:CONTAINER or PORT)", arg)
	}

	return mapping, nil
}

// startPortForwarding starts TCP port forwarding using socat or direct Go implementation.
func startPortForwarding(ctx context.Context, containerName string, mappings []PortMapping) error {
	// Create a context that can be cancelled
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Handle interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		output.Info("Stopping port forwarding...")
		cancel()
	}()

	output.Info("Forwarding ports to container %s:", containerName)
	for _, m := range mappings {
		output.Info("  %s:%d -> %d/%s", pfAddress, m.LocalPort, m.ContainerPort, m.Protocol)
	}
	fmt.Println()
	output.Info("Press Ctrl+C to stop")

	// Get container IP
	containerIP, err := getContainerIP(ctx, containerName)
	if err != nil {
		return fmt.Errorf("failed to get container IP: %w", err)
	}

	// Start listeners for each port
	errCh := make(chan error, len(mappings))
	for _, mapping := range mappings {
		go func(m PortMapping) {
			errCh <- forwardPort(ctx, m, containerIP)
		}(mapping)
	}

	// Wait for context cancellation or error
	select {
	case <-ctx.Done():
		return nil
	case err := <-errCh:
		return err
	}
}

// getContainerIP gets the IP address of a container.
func getContainerIP(ctx context.Context, containerName string) (string, error) {
	cmd := exec.CommandContext(ctx, "docker", "inspect",
		"--format", "{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}",
		containerName)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	ip := strings.TrimSpace(string(out))
	if ip == "" {
		return "", fmt.Errorf("container %s has no IP address", containerName)
	}
	return ip, nil
}

// forwardPort handles TCP port forwarding for a single port mapping.
func forwardPort(ctx context.Context, mapping PortMapping, containerIP string) error {
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", pfAddress, mapping.LocalPort))
	if err != nil {
		return fmt.Errorf("failed to listen on %s:%d: %w", pfAddress, mapping.LocalPort, err)
	}
	defer listener.Close()

	// Close listener when context is cancelled
	go func() {
		<-ctx.Done()
		listener.Close()
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return nil
			default:
				return err
			}
		}

		go handleConnection(ctx, conn, containerIP, mapping.ContainerPort)
	}
}

// handleConnection forwards a single TCP connection.
func handleConnection(ctx context.Context, clientConn net.Conn, containerIP string, containerPort int) {
	defer clientConn.Close()

	// Connect to container
	dialer := net.Dialer{Timeout: 5 * time.Second}
	serverConn, err := dialer.DialContext(ctx, "tcp", fmt.Sprintf("%s:%d", containerIP, containerPort))
	if err != nil {
		return
	}
	defer serverConn.Close()

	// Bidirectional copy
	done := make(chan struct{})
	go func() {
		copyConn(clientConn, serverConn)
		done <- struct{}{}
	}()
	go func() {
		copyConn(serverConn, clientConn)
		done <- struct{}{}
	}()

	// Wait for either direction to finish or context to cancel
	select {
	case <-done:
	case <-ctx.Done():
	}
}

// copyConn copies data from src to dst until EOF or error.
func copyConn(dst, src net.Conn) {
	buf := make([]byte, 32*1024)
	for {
		n, err := src.Read(buf)
		if n > 0 {
			if _, werr := dst.Write(buf[:n]); werr != nil {
				return
			}
		}
		if err != nil {
			return
		}
	}
}

// NewPortsCmd creates the ports command to list exposed ports.
func NewPortsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ports [node]",
		Short: "List exposed ports for nodes",
		Long: `List the exposed ports for devnet nodes.

Shows the port mappings for all nodes or a specific node.

Examples:
  # List all node ports
  devnet-builder ports

  # List ports for node0 only
  devnet-builder ports 0
  devnet-builder ports node0`,
		Args: cobra.MaximumNArgs(1),
		RunE: runPorts,
	}

	return cmd
}

func runPorts(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	cfg := ctxconfig.FromContext(ctx)
	homeDir := cfg.HomeDir()
	jsonMode := cfg.JSONMode()

	svc, err := application.GetService(homeDir)
	if err != nil {
		return fmt.Errorf("failed to initialize service: %w", err)
	}

	if !svc.DevnetExists() {
		return fmt.Errorf("no devnet found at %s", homeDir)
	}

	isDocker, err := svc.IsDockerMode(ctx)
	if err != nil {
		return err
	}
	if !isDocker {
		return fmt.Errorf("ports command only works in docker mode")
	}

	numValidators, err := svc.GetNumValidators(ctx)
	if err != nil {
		return err
	}

	// Determine which nodes to show
	var nodeIndices []int
	if len(args) > 0 {
		index, err := parseNodeArgPF(args[0], numValidators)
		if err != nil {
			return err
		}
		nodeIndices = []int{index}
	} else {
		for i := 0; i < numValidators; i++ {
			nodeIndices = append(nodeIndices, i)
		}
	}

	// Collect port info for each node
	type NodePorts struct {
		Node  int               `json:"node"`
		Ports map[string]string `json:"ports"`
	}

	var allPorts []NodePorts
	for _, idx := range nodeIndices {
		modeInfo, err := svc.GetExecutionModeInfo(ctx, idx)
		if err != nil {
			continue
		}

		ports, err := getContainerPorts(ctx, modeInfo.ContainerName)
		if err != nil {
			continue
		}

		allPorts = append(allPorts, NodePorts{
			Node:  idx,
			Ports: ports,
		})
	}

	if jsonMode {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(allPorts)
	}

	// Text output
	for _, np := range allPorts {
		output.Info("node%d:", np.Node)
		if len(np.Ports) == 0 {
			fmt.Println("  No exposed ports")
		} else {
			for containerPort, hostBinding := range np.Ports {
				fmt.Printf("  %s -> %s\n", containerPort, hostBinding)
			}
		}
		fmt.Println()
	}

	return nil
}

// getContainerPorts gets the port mappings for a container.
func getContainerPorts(ctx context.Context, containerName string) (map[string]string, error) {
	cmd := exec.CommandContext(ctx, "docker", "port", containerName)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	ports := make(map[string]string)
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		// Format: "26657/tcp -> 0.0.0.0:26657"
		parts := strings.Split(line, " -> ")
		if len(parts) == 2 {
			ports[parts[0]] = parts[1]
		}
	}

	return ports, nil
}
