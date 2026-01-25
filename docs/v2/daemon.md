# Daemon Operations

Complete guide to devnetd daemon operations, configuration, and troubleshooting.

## Table of Contents

- [Daemon Lifecycle](#daemon-lifecycle)
- [Configuration](#configuration)
- [Process Management](#process-management)
- [Monitoring and Logs](#monitoring-and-logs)
- [Performance Tuning](#performance-tuning)
- [Backup and Recovery](#backup-and-recovery)
- [Security](#security)
- [Troubleshooting](#troubleshooting)

## Daemon Lifecycle

### Starting the Daemon

#### Foreground Mode (Development)

```bash
# Start with default config
devnetd start

# With custom config
devnetd start --config /path/to/config.toml

# With debug logging
devnetd start --log-level debug

# Custom data directory
devnetd start --data-dir /data/devnet-builder
```

#### Background Mode (Production)

```bash
# Start as daemon
devnetd start --daemon

# With pid file
devnetd start --daemon --pid-file /var/run/devnetd.pid

# Check status
devnetd status
```

####

 Systemd Service

```bash
# Install systemd service
sudo tee /etc/systemd/system/devnetd.service <<EOF
[Unit]
Description=Devnet Builder Daemon
Documentation=https://github.com/altuslabsxyz/devnet-builder
After=docker.service
Requires=docker.service

[Service]
Type=simple
User=devnet
Group=devnet
ExecStart=/usr/local/bin/devnetd start
ExecStop=/usr/local/bin/devnetd shutdown
Restart=on-failure
RestartSec=5s
StandardOutput=journal
StandardError=journal
SyslogIdentifier=devnetd

# Security hardening
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/devnet-builder

[Install]
WantedBy=multi-user.target
EOF

# Reload and enable
sudo systemctl daemon-reload
sudo systemctl enable devnetd
sudo systemctl start devnetd

# View logs
sudo journalctl -u devnetd -f
```

### Stopping the Daemon

```bash
# Graceful shutdown (stops all devnets first)
devnetd shutdown

# Force shutdown (immediate)
devnetd shutdown --force

# Via systemd
sudo systemctl stop devnetd
```

### Restarting

```bash
# Restart daemon (preserves devnets)
devnetd restart

# Via systemd
sudo systemctl restart devnetd
```

## Configuration

### Configuration File

Default location: `~/.devnet-builder/config.toml`

```toml
# config.toml - Daemon configuration

[daemon]
# Data directory for all daemon state
data_dir = "/home/user/.devnet-builder"

# Unix socket path for gRPC
socket_path = "/home/user/.devnet-builder/devnetd.sock"

# TCP address (optional, use for remote access)
# address = "0.0.0.0:50051"

# Log level: debug, info, warn, error
log_level = "info"

# Log format: text, json
log_format = "text"

# Enable metrics collection
enable_metrics = true

# Metrics port
metrics_port = 9090

[controller]
# How often controllers reconcile resources
reconcile_interval = "5s"

# Health check interval for nodes
health_check_interval = "10s"

# Max concurrent reconciliations per controller
max_concurrent_reconciles = 5

# Max node restart attempts before marking failed
max_restart_attempts = 3

# Restart backoff base duration
restart_backoff_base = "5s"

[storage]
# BoltDB file path
db_path = "/home/user/.devnet-builder/devnetd.db"

# Sync writes to disk (slower but safer)
sync_writes = true

# Auto-compact DB on startup
auto_compact = true

# Event retention (days)
event_retention_days = 30

[network]
# Docker network mode: bridge, host
docker_network = "bridge"

# Network isolation per devnet
isolated_networks = true

# Port range for RPC services
rpc_port_start = 26650
rpc_port_end = 26750

# Port range for REST API
rest_port_start = 1300
rest_port_end = 1400

# Port range for gRPC
grpc_port_start = 9080
grpc_port_end = 9180

[cache]
# Binary cache directory
cache_dir = "/home/user/.devnet-builder/cache"

# Max cache size (GB)
max_cache_size_gb = 10

# Cache retention (days)
retention_days = 30

# Auto-cleanup on startup
auto_cleanup = true

[plugins]
# Plugin directory
plugin_dir = "/home/user/.devnet-builder/plugins"

# Plugin auto-discovery
auto_discover = true

# Plugin load timeout
load_timeout = "30s"

[security]
# Enable TLS for gRPC (remote access)
enable_tls = false
tls_cert_file = ""
tls_key_file = ""

# Enable authentication
enable_auth = false
auth_token_file = ""

# Allow insecure operations (local socket only)
allow_insecure_local = true
```

### Environment Variables

Override config with environment variables:

```bash
# Data directory
export DEVNETD_DATA_DIR=/data/devnet-builder

# Log level
export DEVNETD_LOG_LEVEL=debug

# Socket path
export DEVNETD_SOCKET_PATH=/tmp/devnetd.sock

# Docker host
export DOCKER_HOST=unix:///var/run/docker.sock

# Start with overrides
devnetd start
```

### Runtime Configuration Updates

Some settings can be updated at runtime:

```bash
# Update log level
dvb daemon config set log_level=debug

# Update reconcile interval
dvb daemon config set controller.reconcile_interval=10s

# View current config
dvb daemon config show
```

## Process Management

### Status Check

```bash
# Daemon status
dvb daemon status

# Output:
# Status: Running
# PID: 12345
# Uptime: 2h 15m 30s
# Version: v2.0.0
# gRPC Socket: /home/user/.devnet-builder/devnetd.sock
# Active Devnets: 3
# Total Nodes: 12
# Memory Usage: 245 MB
# Open File Descriptors: 125
```

### Health Check

```bash
# Health endpoint
curl http://localhost:9090/health

# Response:
# {
#   "status": "healthy",
#   "checks": {
#     "database": "ok",
#     "docker": "ok",
#     "plugins": "ok",
#     "controllers": "ok"
#   }
# }
```

### Resource Usage

```bash
# View daemon metrics
dvb daemon metrics

# Metrics exposed via Prometheus
curl http://localhost:9090/metrics
```

Example metrics:

```
# HELP devnetd_devnets_total Total number of devnets
# TYPE devnetd_devnets_total gauge
devnetd_devnets_total 3

# HELP devnetd_nodes_total Total number of nodes
# TYPE devnetd_nodes_total gauge
devnetd_nodes_total{phase="running"} 10
devnetd_nodes_total{phase="stopped"} 2

# HELP devnetd_reconcile_duration_seconds Controller reconciliation duration
# TYPE devnetd_reconcile_duration_seconds histogram
devnetd_reconcile_duration_seconds{controller="devnet"} 0.052

# HELP devnetd_reconcile_errors_total Total reconciliation errors
# TYPE devnetd_reconcile_errors_total counter
devnetd_reconcile_errors_total{controller="node"} 2
```

## Monitoring and Logs

### Log Files

```bash
# View logs (systemd)
journalctl -u devnetd -f

# View logs (foreground)
tail -f ~/.devnet-builder/devnetd.log

# Filter by level
journalctl -u devnetd -p err

# JSON logs for parsing
devnetd start --log-format json | jq '.msg'
```

### Structured Logging

Logs use structured format:

```json
{
  "time": "2026-01-25T10:15:30Z",
  "level": "INFO",
  "msg": "devnet provisioning started",
  "devnet": "osmosis-test",
  "validators": 4,
  "controller": "DevnetController"
}
```

### Event Stream

Watch all daemon events:

```bash
# Stream events
dvb daemon events --follow

# Filter by type
dvb daemon events --type devnet --follow

# Filter by resource
dvb daemon events --resource osmosis-test --follow
```

### Audit Trail

All resource modifications are logged:

```bash
# View audit log
dvb daemon audit

# Filter by user (future feature)
dvb daemon audit --user admin

# Export audit log
dvb daemon audit --output json > audit.json
```

## Performance Tuning

### Controller Tuning

```toml
[controller]
# Increase concurrency for large deployments
max_concurrent_reconciles = 10

# Reduce polling for stable environments
reconcile_interval = "30s"
health_check_interval = "60s"

# Aggressive restart attempts
max_restart_attempts = 5
restart_backoff_base = "2s"
```

### Database Optimization

```toml
[storage]
# Enable write-through cache
sync_writes = false  # Faster but less safe

# Increase compact threshold
auto_compact = true

# Batch writes (future feature)
batch_size = 100
```

### Resource Limits

```toml
[daemon]
# Limit memory usage
max_memory_mb = 2048

# Limit concurrent operations
max_concurrent_operations = 50

# Connection pool sizes
max_docker_connections = 20
max_grpc_connections = 100
```

### Profiling

```bash
# Enable CPU profiling
devnetd start --cpuprofile /tmp/cpu.prof

# Enable memory profiling
devnetd start --memprofile /tmp/mem.prof

# Analyze with pprof
go tool pprof -http=:8080 /tmp/cpu.prof
```

## Backup and Recovery

### Database Backup

```bash
# Stop daemon (recommended)
dvb daemon shutdown

# Copy database
cp ~/.devnet-builder/devnetd.db ~/.devnet-builder/devnetd.db.backup

# Restart daemon
devnetd start

# Hot backup (online, may be inconsistent)
cp ~/.devnet-builder/devnetd.db /backup/devnetd-$(date +%Y%m%d).db
```

### State Export

```bash
# Export all resources to JSON
dvb export --output /backup/state.json

# Export specific devnet
dvb export --devnet osmosis-test --output /backup/osmosis.json

# Includes:
# - Devnet specs
# - Node configs
# - Transaction history
# - Upgrade records
```

### State Import

```bash
# Import from backup
dvb import --input /backup/state.json

# Dry-run first
dvb import --input /backup/state.json --dry-run

# Import specific resources
dvb import --input /backup/osmosis.json --resources devnet,nodes
```

### Disaster Recovery

```bash
# 1. Stop daemon
dvb daemon shutdown

# 2. Remove corrupted database
rm ~/.devnet-builder/devnetd.db

# 3. Restore from backup
cp /backup/devnetd.db ~/.devnet-builder/devnetd.db

# 4. Restart daemon
devnetd start

# 5. Verify state
dvb list
dvb nodes health --all
```

## Security

### Unix Socket Permissions

```bash
# Restrict socket access
chmod 600 ~/.devnet-builder/devnetd.sock

# Or use group permissions
chown :devnet ~/.devnet-builder/devnetd.sock
chmod 660 ~/.devnet-builder/devnetd.sock
```

### TLS Configuration

For remote access, enable TLS:

```toml
[security]
enable_tls = true
tls_cert_file = "/etc/devnetd/tls/server.crt"
tls_key_file = "/etc/devnetd/tls/server.key"
```

Generate certificates:

```bash
# Self-signed for testing
openssl req -x509 -newkey rsa:4096 \
  -keyout server.key \
  -out server.crt \
  -days 365 -nodes \
  -subj "/CN=devnetd"

# Client connects with TLS
dvb --tls --ca-cert server.crt daemon status
```

### Authentication

```toml
[security]
enable_auth = true
auth_token_file = "/etc/devnetd/tokens"
```

Token file format:

```
# /etc/devnetd/tokens
admin:secret-token-1234
readonly:public-token-5678
```

Client usage:

```bash
# Set token
export DEVNETD_TOKEN=secret-token-1234

# Or pass explicitly
dvb --token secret-token-1234 list
```

## Troubleshooting

### Daemon Won't Start

**Symptom**: `devnetd start` fails immediately

**Diagnosis**:

```bash
# Check for stale socket
ls -la ~/.devnet-builder/devnetd.sock

# Check for stale PID file
cat ~/.devnet-builder/devnetd.pid
ps aux | grep <pid>

# Check database integrity
file ~/.devnet-builder/devnetd.db
```

**Solutions**:

```bash
# Remove stale files
rm ~/.devnet-builder/devnetd.sock
rm ~/.devnet-builder/devnetd.pid

# Reset database (destructive)
mv ~/.devnet-builder/devnetd.db ~/.devnet-builder/devnetd.db.old
devnetd start

# Start with debug logging
devnetd start --log-level debug
```

### High Memory Usage

**Symptom**: Daemon using excessive memory

**Diagnosis**:

```bash
# Check memory usage
ps aux | grep devnetd

# View metrics
dvb daemon metrics | grep memory

# Check for goroutine leaks
curl http://localhost:9090/debug/pprof/goroutine?debug=1
```

**Solutions**:

```bash
# Set memory limit
devnetd start --max-memory 1024

# Reduce cache size
dvb daemon config set cache.max_cache_size_gb=5

# Reduce concurrent operations
dvb daemon config set controller.max_concurrent_reconciles=3

# Restart daemon periodically (cron)
0 2 * * * /usr/local/bin/dvb daemon restart
```

### Database Corruption

**Symptom**: Errors reading/writing database

**Diagnosis**:

```bash
# Check database file
file ~/.devnet-builder/devnetd.db

# Try opening with bolt
bolt check ~/.devnet-builder/devnetd.db
```

**Solutions**:

```bash
# Backup corrupted DB
mv ~/.devnet-builder/devnetd.db ~/.devnet-builder/devnetd.db.corrupt

# Restore from backup
cp /backup/devnetd.db ~/.devnet-builder/devnetd.db

# Or start fresh
devnetd start
# Re-deploy devnets from configs
```

### Controller Stuck

**Symptom**: Resources not reconciling

**Diagnosis**:

```bash
# Check controller status
dvb daemon status --verbose

# View work queue
dvb daemon queue

# Check for deadlocks
curl http://localhost:9090/debug/pprof/goroutine?debug=1
```

**Solutions**:

```bash
# Restart daemon
dvb daemon restart

# Force reconciliation
dvb reconcile --resource devnet/osmosis-test

# Increase timeout
dvb daemon config set controller.reconcile_timeout=60s
```

### Permission Errors

**Symptom**: Docker operations failing

**Diagnosis**:

```bash
# Check Docker access
docker ps

# Check user groups
groups $(whoami)
```

**Solutions**:

```bash
# Add user to docker group
sudo usermod -aG docker $USER
newgrp docker

# Or run daemon as root (not recommended)
sudo devnetd start
```

## Advanced Operations

### Database Compaction

```bash
# Manual compact (daemon must be stopped)
dvb daemon shutdown
bolt compact -o devnetd-compact.db ~/.devnet-builder/devnetd.db
mv devnetd-compact.db ~/.devnet-builder/devnetd.db
devnetd start
```

### Plugin Management

```bash
# List loaded plugins
dvb plugins list

# Reload plugins
dvb plugins reload

# Install new plugin
dvb plugins install /path/to/plugin-binary

# Uninstall plugin
dvb plugins uninstall cosmos-v050
```

### State Inspection

```bash
# Dump entire state
dvb debug dump-state --output /tmp/state.json

# Inspect specific bucket
dvb debug inspect-bucket devnets

# View work queue
dvb debug work-queue

# Controller statistics
dvb debug controllers
```

## Best Practices

1. **Run as systemd service** - Automatic restart on failure
2. **Regular backups** - Daily database backups
3. **Monitor metrics** - Set up Prometheus + Grafana
4. **Log rotation** - Use journald or logrotate
5. **Resource limits** - Prevent runaway resource usage
6. **Security** - Use TLS for remote access
7. **Separate data directory** - Use dedicated volume/partition
8. **Test upgrades** - Always test daemon upgrades in staging

## Next Steps

- **[Client Guide](client.md)** - Learn dvb CLI commands
- **[Architecture](architecture.md)** - Understand daemon internals
- **[API Reference](api-reference.md)** - gRPC API documentation
