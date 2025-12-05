package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/netip"
	"os"
	"sort"
	"strings"
	"time"

	"tailscale.com/client/tailscale"
	"tailscale.com/ipn"
	"tailscale.com/tailcfg"
)

var (
	checkFlag       = flag.Bool("check", false, "Only check current exit node status and exit")
	setFlag         = flag.String("set", "", "Set specific exit node by ID or hostname")
	listFlag        = flag.Bool("list", false, "List all available Mullvad exit nodes")
	countryFlag     = flag.String("country", "", "Filter Mullvad nodes by country code (e.g., US, CH, SE)")
	autoFlag        = flag.Bool("auto", false, "Auto-select best Mullvad exit node")
	disableFlag     = flag.Bool("disable", false, "Disable exit node")
	verboseFlag     = flag.Bool("verbose", false, "Enable detailed logging")
	preferPriority  = flag.Bool("prefer-priority", false, "Select by Tailscale priority instead of latency (faster but may not be optimal)")
)

type MullvadNode struct {
	ID           tailcfg.StableNodeID
	DNSName      string
	Country      string
	CountryCode  string
	City         string
	CityCode     string
	Priority     int
	Online       bool
	TailscaleIPs []netip.Addr // Tailscale IP addresses for pinging
	Latency      time.Duration // Measured latency (0 if not tested)
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
				ID:           peer.ID,
				DNSName:      peer.DNSName,
				Online:       peer.Online,
				TailscaleIPs: peer.TailscaleIPs,
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

	var bestNode MullvadNode

	// Test latency unless --prefer-priority is specified
	if !*preferPriority {
		// Use smart two-phase latency selection
		testedNodes := smartLatencySelection(ctx, lc, onlineNodes)

		if len(testedNodes) == 0 {
			return fmt.Errorf("no nodes responded to latency tests")
		}

		bestNode = testedNodes[0]
	} else {
		// Use priority-based selection (no latency testing)
		bestNode = onlineNodes[0]
	}

	if *verboseFlag {
		fmt.Printf("\nSelected Mullvad node:\n")
		fmt.Printf("  Hostname: %s\n", strings.TrimSuffix(bestNode.DNSName, "."))
		fmt.Printf("  Location: %s, %s\n", bestNode.City, bestNode.CountryCode)
		fmt.Printf("  Priority: %d\n", bestNode.Priority)
		if bestNode.Latency > 0 {
			fmt.Printf("  Latency: %v\n", bestNode.Latency.Round(time.Millisecond))
		}
		fmt.Printf("  Online: %v\n", bestNode.Online)
	}

	// Set the exit node
	if err := setExitNode(ctx, lc, bestNode.ID); err != nil {
		return err
	}

	// Show latency in output if available
	if bestNode.Latency > 0 {
		fmt.Printf("WAN is now protected via %s (%s, %s) - Latency: %v\n",
			strings.TrimSuffix(bestNode.DNSName, "."),
			bestNode.City,
			bestNode.CountryCode,
			bestNode.Latency.Round(time.Millisecond))
	} else {
		fmt.Printf("WAN is now protected via %s (%s, %s)\n",
			strings.TrimSuffix(bestNode.DNSName, "."),
			bestNode.City,
			bestNode.CountryCode)
	}

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

// pingNode measures the latency to a Mullvad exit node
func pingNode(ctx context.Context, lc *tailscale.LocalClient, node *MullvadNode) time.Duration {
	if len(node.TailscaleIPs) == 0 {
		return time.Duration(0) // No IP available
	}

	// Use the first Tailscale IP
	targetIP := node.TailscaleIPs[0]

	// Create a timeout context (2 seconds for each ping)
	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	// Perform disco ping (tests connectivity)
	result, err := lc.Ping(pingCtx, targetIP, tailcfg.PingDisco)
	if err != nil {
		if *verboseFlag {
			fmt.Printf("  Ping to %s failed: %v\n", strings.TrimSuffix(node.DNSName, "."), err)
		}
		return time.Duration(0) // Failed ping
	}

	// Check for ping errors
	if result.Err != "" {
		if *verboseFlag {
			fmt.Printf("  Ping to %s error: %s\n", strings.TrimSuffix(node.DNSName, "."), result.Err)
		}
		return time.Duration(0)
	}

	// Convert latency from seconds to duration
	latency := time.Duration(result.LatencySeconds * float64(time.Second))

	if *verboseFlag {
		fmt.Printf("  Ping to %s: %v\n", strings.TrimSuffix(node.DNSName, "."), latency.Round(time.Millisecond))
	}

	return latency
}

// testLatencyForNodes tests latency for the top N nodes
func testLatencyForNodes(ctx context.Context, lc *tailscale.LocalClient, nodes []MullvadNode, maxNodes int) []MullvadNode {
	if len(nodes) == 0 {
		return nodes
	}

	// Limit to maxNodes
	testCount := len(nodes)
	if testCount > maxNodes {
		testCount = maxNodes
	}

	if *verboseFlag {
		fmt.Printf("Testing latency for top %d nodes by priority...\n", testCount)
	}

	// Test latency for each node
	for i := 0; i < testCount; i++ {
		nodes[i].Latency = pingNode(ctx, lc, &nodes[i])
	}

	return nodes
}

// CountryGroup represents nodes grouped by country
type CountryGroup struct {
	CountryCode string
	Country     string
	Nodes       []MullvadNode
	BestLatency time.Duration // Best latency from this country
}

// groupNodesByCountry groups Mullvad nodes by country code
func groupNodesByCountry(nodes []MullvadNode) map[string]*CountryGroup {
	groups := make(map[string]*CountryGroup)

	for _, node := range nodes {
		if _, exists := groups[node.CountryCode]; !exists {
			groups[node.CountryCode] = &CountryGroup{
				CountryCode: node.CountryCode,
				Country:     node.Country,
				Nodes:       []MullvadNode{},
			}
		}
		groups[node.CountryCode].Nodes = append(groups[node.CountryCode].Nodes, node)
	}

	return groups
}

// testCountryLatency tests one representative node from each country
// Returns a slice of countries sorted by their best latency
func testCountryLatency(ctx context.Context, lc *tailscale.LocalClient, countryGroups map[string]*CountryGroup) []*CountryGroup {
	if *verboseFlag {
		fmt.Printf("\nPhase 1: Testing one node from each country (%d countries)...\n", len(countryGroups))
	}

	var countries []*CountryGroup

	for _, group := range countryGroups {
		// Test the highest priority (first) node from this country
		if len(group.Nodes) > 0 {
			testNode := &group.Nodes[0]
			latency := pingNode(ctx, lc, testNode)
			testNode.Latency = latency
			group.BestLatency = latency

			if *verboseFlag && latency > 0 {
				fmt.Printf("  %s (%s): %v\n", group.Country, group.CountryCode, latency.Round(time.Millisecond))
			}
		}
		countries = append(countries, group)
	}

	// Sort countries by best latency
	sort.Slice(countries, func(i, j int) bool {
		// Countries with 0 latency (failed) go to the end
		if countries[i].BestLatency == 0 && countries[j].BestLatency != 0 {
			return false
		}
		if countries[i].BestLatency != 0 && countries[j].BestLatency == 0 {
			return true
		}
		// Both have valid latency
		if countries[i].BestLatency != 0 && countries[j].BestLatency != 0 {
			return countries[i].BestLatency < countries[j].BestLatency
		}
		// Both failed, sort by country code
		return countries[i].CountryCode < countries[j].CountryCode
	})

	return countries
}

// testTopCountriesInDepth tests the top N nodes in each of the top M countries
func testTopCountriesInDepth(ctx context.Context, lc *tailscale.LocalClient, countries []*CountryGroup, topCountries int, nodesPerCountry int) []MullvadNode {
	var allNodes []MullvadNode

	// Limit to topCountries
	testCountryCount := len(countries)
	if testCountryCount > topCountries {
		testCountryCount = topCountries
	}

	if *verboseFlag {
		fmt.Printf("\nPhase 2: Testing top %d nodes in each of the top %d countries...\n", nodesPerCountry, testCountryCount)
	}

	for i := 0; i < testCountryCount; i++ {
		country := countries[i]

		// Skip countries that failed initial test
		if country.BestLatency == 0 {
			continue
		}

		if *verboseFlag {
			fmt.Printf("\nTesting nodes in %s (%s):\n", country.Country, country.CountryCode)
		}

		// Test up to nodesPerCountry nodes from this country
		testCount := len(country.Nodes)
		if testCount > nodesPerCountry {
			testCount = nodesPerCountry
		}

		for j := 0; j < testCount; j++ {
			node := &country.Nodes[j]

			// Skip if already tested (first node was tested in phase 1)
			if node.Latency == 0 {
				node.Latency = pingNode(ctx, lc, node)
			} else if *verboseFlag {
				fmt.Printf("  %s: %v (from Phase 1)\n",
					strings.TrimSuffix(node.DNSName, "."),
					node.Latency.Round(time.Millisecond))
			}

			allNodes = append(allNodes, *node)
		}
	}

	// Sort all tested nodes by latency
	sort.Slice(allNodes, func(i, j int) bool {
		if allNodes[i].Latency == 0 && allNodes[j].Latency != 0 {
			return false
		}
		if allNodes[i].Latency != 0 && allNodes[j].Latency == 0 {
			return true
		}
		if allNodes[i].Latency != 0 && allNodes[j].Latency != 0 {
			return allNodes[i].Latency < allNodes[j].Latency
		}
		return allNodes[i].Priority < allNodes[j].Priority
	})

	return allNodes
}

// smartLatencySelection performs two-phase latency testing:
// Phase 1: Test one node per country
// Phase 2: Deep test top nodes in fastest countries
func smartLatencySelection(ctx context.Context, lc *tailscale.LocalClient, nodes []MullvadNode) []MullvadNode {
	// Group nodes by country
	countryGroups := groupNodesByCountry(nodes)

	// Phase 1: Test one node from each country
	sortedCountries := testCountryLatency(ctx, lc, countryGroups)

	// Phase 2: Deep test top 5 nodes in top 5 countries
	testedNodes := testTopCountriesInDepth(ctx, lc, sortedCountries, 5, 5)

	return testedNodes
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
