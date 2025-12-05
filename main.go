package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"

	"tailscale.com/client/tailscale"
	"tailscale.com/ipn"
	"tailscale.com/tailcfg"
)

var (
	checkFlag   = flag.Bool("check", false, "Only check current exit node status and exit")
	setFlag     = flag.String("set", "", "Set specific exit node by ID or hostname")
	listFlag    = flag.Bool("list", false, "List all available Mullvad exit nodes")
	countryFlag = flag.String("country", "", "Filter Mullvad nodes by country code (e.g., US, CH, SE)")
	autoFlag    = flag.Bool("auto", false, "Auto-select best Mullvad exit node")
	disableFlag = flag.Bool("disable", false, "Disable exit node")
	verboseFlag = flag.Bool("verbose", false, "Enable detailed logging")
)

type MullvadNode struct {
	ID          tailcfg.StableNodeID
	DNSName     string
	Country     string
	CountryCode string
	City        string
	CityCode    string
	Priority    int
	Online      bool
}

func main() {
	flag.Parse()

	ctx := context.Background()
	lc := &tailscale.LocalClient{}

	// Handle explicit flags first
	if *checkFlag {
		exitNodeActive, err := checkExitNode(ctx, lc)
		if err != nil {
			log.Fatalf("Error checking exit node: %v", err)
		}
		if exitNodeActive {
			fmt.Println("WAN is protected")
			os.Exit(0)
		} else {
			fmt.Println("No exit node active")
			os.Exit(1)
		}
	}

	if *listFlag {
		if err := listMullvadNodes(ctx, lc); err != nil {
			log.Fatalf("Error listing Mullvad nodes: %v", err)
		}
		os.Exit(0)
	}

	if *disableFlag {
		if err := clearExitNode(ctx, lc); err != nil {
			log.Fatalf("Error disabling exit node: %v", err)
		}
		fmt.Println("Exit node disabled successfully")
		os.Exit(0)
	}

	if *setFlag != "" {
		if err := setExitNodeByName(ctx, lc, *setFlag); err != nil {
			log.Fatalf("Error setting exit node: %v", err)
		}
		fmt.Printf("Exit node set to: %s\n", *setFlag)
		os.Exit(0)
	}

	if *autoFlag {
		if err := autoSelectMullvad(ctx, lc); err != nil {
			log.Fatalf("Error auto-selecting Mullvad node: %v", err)
		}
		os.Exit(0)
	}

	// Default behavior: check if exit node is active, if not, auto-select
	exitNodeActive, err := checkExitNode(ctx, lc)
	if err != nil {
		log.Fatalf("Error checking exit node: %v", err)
	}

	if exitNodeActive {
		fmt.Println("WAN is protected")
		os.Exit(0)
	}

	// No exit node active, auto-select best Mullvad node
	if *verboseFlag {
		fmt.Println("No exit node active. Auto-selecting best Mullvad node...")
	}

	if err := autoSelectMullvad(ctx, lc); err != nil {
		log.Fatalf("Error auto-selecting Mullvad node: %v", err)
	}
}

// checkExitNode checks if an exit node is currently active
// Returns true if active, false otherwise
func checkExitNode(ctx context.Context, lc *tailscale.LocalClient) (bool, error) {
	status, err := lc.StatusWithoutPeers(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to get status: %w", err)
	}

	if status.ExitNodeStatus != nil && status.ExitNodeStatus.Online {
		if *verboseFlag {
			fmt.Printf("Exit node active:\n")
			fmt.Printf("  ID: %s\n", status.ExitNodeStatus.ID)
			fmt.Printf("  Online: %v\n", status.ExitNodeStatus.Online)
			fmt.Printf("  IPs: %v\n", status.ExitNodeStatus.TailscaleIPs)
		}
		return true, nil
	}

	return false, nil
}

// listMullvadNodes lists all available Mullvad exit nodes
func listMullvadNodes(ctx context.Context, lc *tailscale.LocalClient) error {
	nodes, err := getMullvadNodes(ctx, lc)
	if err != nil {
		return err
	}

	if len(nodes) == 0 {
		fmt.Println("No Mullvad exit nodes found.")
		fmt.Println("Note: Mullvad VPN add-on requires a subscription ($5/month per 5 devices)")
		return nil
	}

	// Apply country filter if specified
	if *countryFlag != "" {
		filtered := make([]MullvadNode, 0)
		for _, node := range nodes {
			if strings.EqualFold(node.CountryCode, *countryFlag) {
				filtered = append(filtered, node)
			}
		}
		nodes = filtered
	}

	fmt.Printf("Available Mullvad Exit Nodes (%d):\n", len(nodes))
	fmt.Println(strings.Repeat("-", 80))
	fmt.Printf("%-40s %-20s %-8s %s\n", "HOSTNAME", "LOCATION", "ONLINE", "PRIORITY")
	fmt.Println(strings.Repeat("-", 80))

	for _, node := range nodes {
		location := fmt.Sprintf("%s, %s", node.City, node.CountryCode)
		onlineStr := "Yes"
		if !node.Online {
			onlineStr = "No"
		}
		fmt.Printf("%-40s %-20s %-8s %d\n",
			strings.TrimSuffix(node.DNSName, "."),
			location,
			onlineStr,
			node.Priority)
	}

	return nil
}

// getMullvadNodes retrieves all Mullvad exit nodes from Tailscale status
func getMullvadNodes(ctx context.Context, lc *tailscale.LocalClient) ([]MullvadNode, error) {
	status, err := lc.Status(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get status: %w", err)
	}

	var nodes []MullvadNode

	for _, peer := range status.Peer {
		// Check if this is a Mullvad exit node
		if peer.ExitNodeOption && strings.HasSuffix(peer.DNSName, ".mullvad.ts.net.") {
			node := MullvadNode{
				ID:      peer.ID,
				DNSName: peer.DNSName,
				Online:  peer.Online,
			}

			if peer.Location != nil {
				node.Country = peer.Location.Country
				node.CountryCode = peer.Location.CountryCode
				node.City = peer.Location.City
				node.CityCode = peer.Location.CityCode
				node.Priority = peer.Location.Priority
			}

			nodes = append(nodes, node)
		}
	}

	// Sort by priority (lower is better), then by online status, then by name
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Priority != nodes[j].Priority {
			return nodes[i].Priority < nodes[j].Priority
		}
		if nodes[i].Online != nodes[j].Online {
			return nodes[i].Online
		}
		return nodes[i].DNSName < nodes[j].DNSName
	})

	return nodes, nil
}

// autoSelectMullvad automatically selects and sets the best Mullvad exit node
func autoSelectMullvad(ctx context.Context, lc *tailscale.LocalClient) error {
	nodes, err := getMullvadNodes(ctx, lc)
	if err != nil {
		return err
	}

	if len(nodes) == 0 {
		return fmt.Errorf("no Mullvad exit nodes found. Mullvad VPN add-on subscription required")
	}

	// Apply country filter if specified
	if *countryFlag != "" {
		filtered := make([]MullvadNode, 0)
		for _, node := range nodes {
			if strings.EqualFold(node.CountryCode, *countryFlag) {
				filtered = append(filtered, node)
			}
		}
		if len(filtered) == 0 {
			return fmt.Errorf("no Mullvad exit nodes found for country: %s", *countryFlag)
		}
		nodes = filtered
	}

	// Filter for online nodes only
	onlineNodes := make([]MullvadNode, 0)
	for _, node := range nodes {
		if node.Online {
			onlineNodes = append(onlineNodes, node)
		}
	}

	if len(onlineNodes) == 0 {
		return fmt.Errorf("no online Mullvad exit nodes found")
	}

	// Select the best node (already sorted by priority)
	bestNode := onlineNodes[0]

	if *verboseFlag {
		fmt.Printf("Selected Mullvad node:\n")
		fmt.Printf("  Hostname: %s\n", strings.TrimSuffix(bestNode.DNSName, "."))
		fmt.Printf("  Location: %s, %s\n", bestNode.City, bestNode.CountryCode)
		fmt.Printf("  Priority: %d\n", bestNode.Priority)
		fmt.Printf("  Online: %v\n", bestNode.Online)
	}

	// Set the exit node
	if err := setExitNode(ctx, lc, bestNode.ID); err != nil {
		return err
	}

	fmt.Printf("WAN is now protected via %s (%s, %s)\n",
		strings.TrimSuffix(bestNode.DNSName, "."),
		bestNode.City,
		bestNode.CountryCode)

	return nil
}

// setExitNode sets the exit node by StableNodeID
func setExitNode(ctx context.Context, lc *tailscale.LocalClient, nodeID tailcfg.StableNodeID) error {
	mp := &ipn.MaskedPrefs{
		Prefs: ipn.Prefs{
			ExitNodeID: nodeID,
		},
		ExitNodeIDSet: true,
	}

	_, err := lc.EditPrefs(ctx, mp)
	if err != nil {
		return handlePermissionError(err, "set exit node")
	}

	if *verboseFlag {
		fmt.Printf("Exit node set to ID: %s\n", nodeID)
	}

	return nil
}

// setExitNodeByName sets the exit node by hostname or ID string
func setExitNodeByName(ctx context.Context, lc *tailscale.LocalClient, name string) error {
	nodes, err := getMullvadNodes(ctx, lc)
	if err != nil {
		return err
	}

	// Try to find by hostname (with or without trailing dot)
	nameWithDot := name
	if !strings.HasSuffix(name, ".") {
		nameWithDot = name + "."
	}
	nameWithoutDot := strings.TrimSuffix(name, ".")

	for _, node := range nodes {
		if node.DNSName == nameWithDot || strings.TrimSuffix(node.DNSName, ".") == nameWithoutDot {
			return setExitNode(ctx, lc, node.ID)
		}
		// Also try matching by ID string
		if string(node.ID) == name {
			return setExitNode(ctx, lc, node.ID)
		}
	}

	return fmt.Errorf("exit node not found: %s", name)
}

// clearExitNode disables the exit node
func clearExitNode(ctx context.Context, lc *tailscale.LocalClient) error {
	mp := &ipn.MaskedPrefs{
		Prefs: ipn.Prefs{
			ExitNodeID: "",
		},
		ExitNodeIDSet: true,
	}

	_, err := lc.EditPrefs(ctx, mp)
	if err != nil {
		return handlePermissionError(err, "clear exit node")
	}

	if *verboseFlag {
		fmt.Println("Exit node preference cleared")
	}

	return nil
}

// handlePermissionError checks if the error is permission-related and provides helpful guidance
func handlePermissionError(err error, operation string) error {
	errMsg := err.Error()

	// Check for common permission-related error messages
	if strings.Contains(errMsg, "Access denied") ||
	   strings.Contains(errMsg, "permission denied") ||
	   strings.Contains(errMsg, "prefs write access denied") {
		return fmt.Errorf(`failed to %s: %w

Permission denied. Tailscale preferences require elevated access.

Try one of these solutions:

1. Run with sudo:
   sudo %s

2. Run as the tailscale user (Linux):
   sudo -u tailscale %s

3. Grant your user access to Tailscale (Linux):
   sudo usermod -a -G tailscale $USER
   (then logout and login again)

4. On macOS, ensure you're running as an admin user or use sudo

5. Use the tailscale CLI directly as an alternative:
   tailscale set --exit-node=<node-hostname>

For more information, see: https://tailscale.com/kb/1103/exit-nodes`,
			operation, err, os.Args[0], os.Args[0])
	}

	// Return the original error with context if it's not a permission error
	return fmt.Errorf("failed to %s: %w", operation, err)
}
