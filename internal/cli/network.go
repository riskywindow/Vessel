package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/vessel/vessel/internal/daemon"
	"github.com/vessel/vessel/internal/network"
	"github.com/vessel/vessel/internal/store"
)

var networkCmd = &cobra.Command{
	Use:   "network",
	Short: "Manage container networking",
	Long: `Manage Vessel container networking.

Commands:
  vessel network status   Show network bridge and routing status
  vessel network list     List container IP assignments`,
}

var networkStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show network status",
	Long: `Display the status of the Vessel network bridge, IP forwarding, and allocated IPs.

Examples:
  vessel network status`,
	RunE: runNetworkStatus,
}

var networkListCmd = &cobra.Command{
	Use:   "list",
	Short: "List container IP assignments",
	Long: `List all containers and their assigned IP addresses.

Examples:
  vessel network list
  vessel network list --json`,
	RunE: runNetworkList,
}

func runNetworkStatus(cmd *cobra.Command, args []string) error {
	// Check bridge exists
	bridgeExists := false
	if _, err := os.Stat("/sys/class/net/vessel0"); err == nil {
		bridgeExists = true
	}

	fmt.Println("Network Status")
	fmt.Println(strings.Repeat("-", 40))

	if bridgeExists {
		fmt.Println("Bridge:         vessel0 (up)")
	} else {
		fmt.Println("Bridge:         not configured")
	}

	// Check IP forwarding
	data, _ := os.ReadFile("/proc/sys/net/ipv4/ip_forward")
	forwarding := strings.TrimSpace(string(data))
	if forwarding == "1" {
		fmt.Println("IP Forwarding:  enabled")
	} else {
		fmt.Println("IP Forwarding:  disabled")
	}

	// Get container count from daemon
	client := daemon.NewClient("")
	var containers []*store.Container
	if err := client.Call("containers.list", nil, &containers); err != nil {
		fmt.Println("Daemon:         not running")
		return nil
	}

	ipCount := 0
	for _, c := range containers {
		if c.IP != "" {
			ipCount++
		}
	}
	fmt.Printf("Allocated IPs:  %d\n", ipCount)
	fmt.Printf("Subnet:         10.88.0.0/16\n")
	fmt.Printf("Gateway:        10.88.0.1\n")

	// Fetch routes from daemon
	var routes map[string][]network.RouteTarget
	if err := client.Call("network.routes", nil, &routes); err != nil {
		fmt.Println("\nRoutes:         error fetching")
	} else {
		fmt.Println("\nRoutes:")
		if len(routes) == 0 {
			fmt.Println("  (none)")
		}
		for hostname, targets := range routes {
			for _, t := range targets {
				shortID := t.ContainerID
				if len(shortID) > 12 {
					shortID = shortID[:12]
				}
				fmt.Printf("  %s → %s:%d (container %s)\n", hostname, t.IP, t.Port, shortID)
			}
		}
	}

	return nil
}

func runNetworkList(cmd *cobra.Command, args []string) error {
	client := daemon.NewClient("")
	var containers []*store.Container
	if err := client.Call("containers.list", nil, &containers); err != nil {
		return fmt.Errorf("daemon not running: %w", err)
	}

	if jsonOutput {
		type netEntry struct {
			ContainerID string `json:"container_id"`
			AppID       string `json:"app_id"`
			IP          string `json:"ip"`
			State       string `json:"state"`
		}
		var entries []netEntry
		for _, c := range containers {
			ip := c.IP
			if ip == "" {
				ip = "-"
			}
			entries = append(entries, netEntry{
				ContainerID: c.ID,
				AppID:       c.AppID,
				IP:          ip,
				State:       string(c.State),
			})
		}
		data, err := json.MarshalIndent(entries, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	if len(containers) == 0 {
		fmt.Println("No containers found.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "CONTAINER\tAPP\tIP\tSTATE")
	fmt.Fprintln(w, "---------\t---\t--\t-----")

	for _, c := range containers {
		ip := c.IP
		if ip == "" {
			ip = "-"
		}
		shortID := c.ID
		if len(shortID) > 12 {
			shortID = shortID[:12]
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", shortID, c.AppID, ip, c.State)
	}

	w.Flush()
	return nil
}
