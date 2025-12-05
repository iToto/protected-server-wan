# protect-wan

A Go utility that automatically ensures your WAN connection is protected by a Tailscale exit node, with automatic Mullvad VPN exit node selection.

## Features

- Automatically checks if a Tailscale exit node is active
- Auto-selects the best Mullvad VPN exit node based on priority and availability
- Full CLI control with flags for checking, listing, and setting exit nodes
- Filters by country code for region-specific exit nodes
- Built using the official Tailscale Go SDK

## Prerequisites

- **Tailscale** installed and running (`tailscaled` daemon must be active)
- **Mullvad VPN add-on** subscription ($5/month per 5 devices) - [Subscribe here](https://tailscale.com/kb/1258/mullvad-exit-nodes)
- **Go 1.21+** (for building from source)
- Appropriate permissions to access the Tailscale daemon socket (typically requires running as the same user as `tailscaled` or root)

## Installation

### Build from Source

#### Using Make (Recommended)

```bash
# Clone the repository
cd /path/to/protected-server-wan

# Build the binary
make build

# Or simply
make
```

#### Using Go directly

```bash
# Build the binary
go build -o protect-wan

# Optionally, install to your PATH
sudo mv protect-wan /usr/local/bin/
```

### Using Make Targets

The project includes a Makefile with convenient targets:

```bash
# Build the binary
make build

# Build and run with default behavior
make run

# Build and check exit node status
make check

# Build and list Mullvad nodes
make list

# Build and auto-select exit node
make auto

# Build and disable exit node
make disable

# Build and run with verbose output
make verbose

# Clean build artifacts
make clean

# Install to /usr/local/bin (requires sudo)
make install

# Uninstall from /usr/local/bin
make uninstall

# Show all available targets
make help
```

### Cross-Platform Builds

#### Using Make

```bash
# Build for Linux (amd64)
make build-linux

# Build for macOS (arm64 and amd64)
make build-darwin

# Build for Windows (amd64)
make build-windows

# Build for all platforms
make build-all
```

#### Using Go directly

```bash
# Linux (amd64)
GOOS=linux GOARCH=amd64 go build -o protect-wan-linux

# macOS (Apple Silicon)
GOOS=darwin GOARCH=arm64 go build -o protect-wan-macos-arm64

# macOS (Intel)
GOOS=darwin GOARCH=amd64 go build -o protect-wan-macos-amd64

# Windows (amd64)
GOOS=windows GOARCH=amd64 go build -o protect-wan.exe
```

## Usage

### Default Behavior

Run without flags to automatically check and protect your WAN:

```bash
./protect-wan
```

**Behavior:**
- If an exit node is already active: Prints "WAN is protected" and exits with code 0
- If no exit node is active: Auto-selects the best Mullvad exit node and activates it

### Available Flags

```
--check              Only check current exit node status and exit
--list               List all available Mullvad exit nodes
--set <hostname>     Set specific exit node by hostname or ID
--country <code>     Filter Mullvad nodes by country code (e.g., US, CH, SE)
--auto               Auto-select and set the best Mullvad exit node
--disable            Disable/clear the current exit node
--verbose            Enable detailed logging
```

### Examples

#### Check if WAN is Protected

```bash
./protect-wan --check
```

Output:
- `WAN is protected` (exit code 0) if an exit node is active
- `No exit node active` (exit code 1) if no exit node is active

#### List Available Mullvad Exit Nodes

```bash
./protect-wan --list
```

Example output:
```
Available Mullvad Exit Nodes (47):
--------------------------------------------------------------------------------
HOSTNAME                                 LOCATION             ONLINE   PRIORITY
--------------------------------------------------------------------------------
us-nyc-wg-301.mullvad.ts.net            New York City, US    Yes      10
us-lax-wg-102.mullvad.ts.net            Los Angeles, US      Yes      10
ch-zrh-wg-001.mullvad.ts.net            Zurich, CH           Yes      11
se-sto-wg-005.mullvad.ts.net            Stockholm, SE        Yes      12
...
```

#### List Exit Nodes for Specific Country

```bash
./protect-wan --list --country US
./protect-wan --list --country CH
./protect-wan --list --country SE
```

#### Auto-Select Best Mullvad Node

```bash
./protect-wan --auto
```

Output:
```
WAN is now protected via us-nyc-wg-301.mullvad.ts.net (New York City, US)
```

#### Auto-Select Best Node in Specific Country

```bash
./protect-wan --auto --country CH
```

This will select the best (lowest priority, online) Mullvad exit node in Switzerland.

#### Set Specific Exit Node

```bash
./protect-wan --set us-lax-wg-102.mullvad.ts.net
```

Or with verbose output:

```bash
./protect-wan --set ch-zrh-wg-001.mullvad.ts.net --verbose
```

#### Disable Exit Node

```bash
./protect-wan --disable
```

Output:
```
Exit node disabled successfully
```

#### Verbose Mode

Add `--verbose` to any command for detailed logging:

```bash
./protect-wan --verbose
./protect-wan --auto --verbose
./protect-wan --check --verbose
```

## How It Works

1. **Connection**: Uses the Tailscale Go SDK to connect to the local `tailscaled` daemon via Unix socket (or named pipe on Windows)

2. **Exit Node Check**: Queries the daemon status to check if `ExitNodeStatus` is present and online

3. **Mullvad Node Discovery**: Retrieves all peers from Tailscale status and filters for nodes with DNS names ending in `.mullvad.ts.net.`

4. **Best Node Selection**:
   - Filters for **online nodes only**
   - Sorts by **priority** (lower number = better)
   - Optionally filters by **country code**
   - Selects the first matching node

5. **Exit Node Activation**: Uses `EditPrefs` with `MaskedPrefs` to set the `ExitNodeID` preference

## Exit Codes

- `0` - Success (exit node active or successfully set)
- `1` - Error or no exit node active (when using `--check`)

## Permissions

The program requires appropriate permissions to access and modify Tailscale daemon preferences.

### Permission Requirements

- **Linux/macOS**: Must run as the same user as `tailscaled` or as root
- **Windows**: Must run with Administrator privileges

### Common Permission Error

If you see this error:
```
Error auto-selecting Mullvad node: failed to set exit node: Access denied: prefs write access denied
```

This means your user doesn't have permission to modify Tailscale preferences.

### Solutions

#### 1. Run with sudo (Recommended for servers)

```bash
sudo ./protect-wan
sudo make auto
```

#### 2. Add your user to the tailscale group (Linux)

```bash
# Add user to tailscale group
sudo usermod -a -G tailscale $USER

# Logout and login again for changes to take effect
# Or use newgrp:
newgrp tailscale
```

#### 3. Run as the tailscale user (Linux)

```bash
sudo -u tailscale ./protect-wan
```

#### 4. Use make targets with sudo

```bash
sudo make auto
sudo make disable
sudo make run
```

#### 5. Install to /usr/local/bin and create a wrapper script

```bash
# Install the binary
make install

# Create a wrapper script that runs with appropriate permissions
echo '#!/bin/bash' | sudo tee /usr/local/bin/protect-wan-sudo
echo 'sudo /usr/local/bin/protect-wan "$@"' | sudo tee -a /usr/local/bin/protect-wan-sudo
sudo chmod +x /usr/local/bin/protect-wan-sudo
```

### Checking Current Permissions

To check if you have access without modifying anything:

```bash
# This only reads, doesn't require write permissions
./protect-wan --check
./protect-wan --list
```

## Troubleshooting

### No Mullvad Exit Nodes Found

If you see "No Mullvad exit nodes found", ensure:
1. You have an active Mullvad VPN add-on subscription
2. Your Tailscale client is up-to-date
3. Run `tailscale exit-node list` to verify Mullvad nodes are visible

### Permission Denied

See the [Permissions](#permissions) section above for detailed solutions to permission-related errors.

### Exit Node Set But Not Working

If the exit node is set but traffic isn't routing through it:
1. Check Tailscale status: `tailscale status`
2. Verify exit node is online: `./protect-wan --list`
3. Check Tailscale logs: `sudo journalctl -u tailscaled -f` (Linux)

## Development

### Project Structure

```
protected-server-wan/
├── main.go          # Main program logic
├── go.mod           # Go module definition
├── go.sum           # Dependency checksums
├── Makefile         # Build automation
├── README.md        # This file
└── .gitignore       # Git ignore patterns
```

### Building

```bash
# Build
go build -o protect-wan

# Run tests (if any)
go test ./...

# Format code
go fmt ./...

# Vet code
go vet ./...
```

### Dependencies

- `tailscale.com/client/tailscale` - Tailscale LocalClient for daemon communication
- `tailscale.com/ipn` - Tailscale preferences and configuration structures
- `tailscale.com/ipn/ipnstate` - Tailscale status structures
- `tailscale.com/tailcfg` - Tailscale configuration types

## References

- [Tailscale Exit Nodes Documentation](https://tailscale.com/kb/1103/exit-nodes)
- [Mullvad Exit Nodes](https://tailscale.com/kb/1258/mullvad-exit-nodes)
- [Tailscale Go SDK Documentation](https://pkg.go.dev/tailscale.com)
- [Tailscale CLI Documentation](https://tailscale.com/kb/1080/cli)

## License

This is an open source project. Feel free to use, modify, and distribute as needed.

## Contributing

Contributions are welcome! Please feel free to submit issues or pull requests.
