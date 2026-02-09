package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/vessel/vessel/internal/cli/progress"
	"github.com/vessel/vessel/internal/cli/styles"
	"github.com/vessel/vessel/internal/config"
	"github.com/vessel/vessel/internal/daemon"
	vesselSSH "github.com/vessel/vessel/internal/ssh"
)

var sshCmd = &cobra.Command{
	Use:   "ssh <user@host> [command] [args...]",
	Short: "Run vessel commands on a remote server",
	Long: `Execute Vessel commands on a remote server via SSH.

The remote server must have Vessel installed and the daemon running.
An SSH tunnel is established to forward commands to the remote daemon.

Examples:
  vessel ssh root@prod.example.com ps                        # List apps on remote
  vessel ssh root@prod.example.com deploy --config app.toml  # Deploy to remote
  vessel ssh root@prod.example.com logs my-app               # View remote logs
  vessel ssh deploy@server exec my-app -- /bin/sh            # Shell into remote container

Connection options:
  vessel ssh -i ~/.ssh/prod_key root@server ps               # Use specific SSH key
  vessel ssh -p 2222 root@server ps                          # Custom SSH port

Named remotes (from vessel.toml [[remote]] sections):
  vessel ssh prod ps                                         # Use named remote "prod"
  vessel ssh staging deploy --all                            # Deploy to staging`,
	Args:               cobra.MinimumNArgs(1),
	RunE:               runSSH,
	DisableFlagParsing: false,
}

var (
	sshPort    int
	sshKeyFile string
)

func init() {
	sshCmd.Flags().IntVarP(&sshPort, "port", "p", 22, "SSH port")
	sshCmd.Flags().StringVarP(&sshKeyFile, "identity", "i", "", "path to SSH private key")
}

func runSSH(cmd *cobra.Command, args []string) error {
	target := args[0]

	// Check if target is a named remote from config
	sshCfg, err := resolveSSHTarget(target)
	if err != nil {
		return err
	}

	// CLI flags override config values
	if cmd.Flags().Changed("port") {
		sshCfg.Port = sshPort
	}
	if cmd.Flags().Changed("identity") {
		sshCfg.KeyFile = sshKeyFile
	}

	// Get remaining args (vessel command)
	vesselArgs := args[1:]
	if len(vesselArgs) == 0 {
		vesselArgs = []string{"ps"} // Default to ps
	}

	// Create SSH client
	sshClient := vesselSSH.NewClient(sshCfg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle Ctrl+C
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
		sshClient.Close()
	}()

	// For commands that need direct TTY access, run via SSH directly
	if needsDirectSSH(vesselArgs) {
		return runDirectSSH(ctx, sshClient, vesselArgs)
	}

	// For commands that can use the daemon socket, tunnel it
	return runTunneledCommand(ctx, cmd, sshClient, vesselArgs)
}

// resolveSSHTarget resolves a target string to an SSH config.
// Supports "user@host", "host", or named remotes from vessel.toml.
func resolveSSHTarget(target string) (vesselSSH.Config, error) {
	// Try to load config for named remotes
	cfg, err := config.Load(configFile)
	if err == nil && len(cfg.Remotes) > 0 {
		for _, r := range cfg.Remotes {
			if r.Name == target {
				port := r.Port
				if port == 0 {
					port = 22
				}
				return vesselSSH.Config{
					Host:    r.Host,
					User:    r.User,
					Port:    port,
					KeyFile: r.KeyFile,
				}, nil
			}
		}
	}

	// Parse as user@host
	user, host := vesselSSH.ParseTarget(target)
	return vesselSSH.Config{
		Host: host,
		User: user,
		Port: 22,
	}, nil
}

// needsDirectSSH returns true if the command needs direct SSH execution
// (e.g., interactive commands with TTY, streaming commands).
func needsDirectSSH(args []string) bool {
	if len(args) == 0 {
		return false
	}
	switch args[0] {
	case "exec":
		return true // Needs TTY support
	case "logs":
		// Check for -f/--follow flag
		for _, arg := range args[1:] {
			if arg == "-f" || arg == "--follow" {
				return true
			}
		}
		return false
	case "health":
		// "health watch" needs direct SSH for streaming
		for _, arg := range args[1:] {
			if arg == "watch" {
				return true
			}
		}
		return false
	default:
		return false
	}
}

// runDirectSSH runs a vessel command directly on the remote server via SSH.
func runDirectSSH(ctx context.Context, client *vesselSSH.Client, vesselArgs []string) error {
	remoteCmd := "vessel " + strings.Join(vesselArgs, " ")

	if !jsonOutput {
		fmt.Printf("%s Running on %s@%s: vessel %s\n",
			styles.InfoIcon(),
			client.User(), client.Host(),
			strings.Join(vesselArgs, " "))
	}

	return client.RunCommand(ctx, remoteCmd)
}

// runTunneledCommand runs a vessel command through an SSH-tunneled daemon socket.
func runTunneledCommand(ctx context.Context, cmd *cobra.Command, sshClient *vesselSSH.Client, vesselArgs []string) error {
	if !jsonOutput {
		spin := progress.NewSpinner(fmt.Sprintf("Connecting to %s@%s...", sshClient.User(), sshClient.Host()))
		spin.Start()

		// Check Vessel is installed on remote
		if err := sshClient.CheckVessel(ctx); err != nil {
			spin.Fail("Remote server check failed: vessel not found")
			return fmt.Errorf("remote server check failed: %w", err)
		}

		// If deploying with a config file, sync it first
		if shouldSyncConfig(vesselArgs) {
			configPath := getConfigFromArgs(vesselArgs)
			if configPath != "" {
				spin.UpdateMessage(fmt.Sprintf("Syncing config: %s", configPath))
				remotePath := "/tmp/vessel-deploy-config.toml"
				if err := sshClient.CopyFile(ctx, configPath, remotePath); err != nil {
					spin.Fail("Failed to sync config file")
					return fmt.Errorf("failed to sync config: %w", err)
				}
				vesselArgs = replaceConfigPath(vesselArgs, remotePath)
			}
		}

		// Establish SSH tunnel
		spin.UpdateMessage("Establishing SSH tunnel...")
		localSocket, err := sshClient.Connect(ctx)
		if err != nil {
			spin.Fail("Failed to establish SSH tunnel")
			return fmt.Errorf("failed to connect: %w", err)
		}
		defer sshClient.Close()

		spin.Success(fmt.Sprintf("Connected to %s@%s", sshClient.User(), sshClient.Host()))

		// Execute via tunneled socket
		return executeTunneledCommand(localSocket, vesselArgs)
	}

	// JSON mode: no spinner, direct execution
	if err := sshClient.CheckVessel(ctx); err != nil {
		return fmt.Errorf("remote server check failed: %w", err)
	}

	if shouldSyncConfig(vesselArgs) {
		configPath := getConfigFromArgs(vesselArgs)
		if configPath != "" {
			remotePath := "/tmp/vessel-deploy-config.toml"
			if err := sshClient.CopyFile(ctx, configPath, remotePath); err != nil {
				return fmt.Errorf("failed to sync config: %w", err)
			}
			vesselArgs = replaceConfigPath(vesselArgs, remotePath)
		}
	}

	localSocket, err := sshClient.Connect(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer sshClient.Close()

	return executeTunneledCommand(localSocket, vesselArgs)
}

// executeTunneledCommand runs a vessel daemon command through the tunneled socket.
func executeTunneledCommand(socketPath string, vesselArgs []string) error {
	client := daemon.NewClient(socketPath)

	if len(vesselArgs) == 0 {
		return fmt.Errorf("no command specified")
	}

	switch vesselArgs[0] {
	case "ps":
		return runRemotePS(client)
	case "deploy":
		return runRemoteDeploy(client, vesselArgs[1:])
	case "stop":
		return runRemoteStop(client, vesselArgs[1:])
	case "rm":
		return runRemoteRm(client, vesselArgs[1:])
	case "health":
		return runRemoteHealth(client, vesselArgs[1:])
	case "stats":
		return runRemoteStats(client, vesselArgs[1:])
	case "history":
		return runRemoteHistory(client, vesselArgs[1:])
	case "logs":
		return runRemoteLogs(client, vesselArgs[1:])
	case "rollback":
		return runRemoteRollback(client, vesselArgs[1:])
	default:
		return fmt.Errorf("unsupported tunneled command %q — use direct SSH for this command", vesselArgs[0])
	}
}

// runRemotePS lists apps on the remote server.
func runRemotePS(client *daemon.Client) error {
	var apps []map[string]interface{}
	if err := client.Call("apps.list", nil, &apps); err != nil {
		return fmt.Errorf("failed to list apps: %w", err)
	}

	if len(apps) == 0 {
		fmt.Println("No applications found on remote server.")
		return nil
	}

	// Print using the same format as local ps
	fmt.Printf("%-16s %-12s %-12s %-16s %-24s\n",
		styles.TableHeader.Render("NAME"),
		styles.TableHeader.Render("STATUS"),
		styles.TableHeader.Render("INSTANCES"),
		styles.TableHeader.Render("IP"),
		styles.TableHeader.Render("IMAGE"))
	fmt.Println(strings.Repeat("-", 80))

	for _, app := range apps {
		name, _ := app["name"].(string)
		state, _ := app["state"].(string)
		image, _ := app["image"].(string)
		instances := 0
		if v, ok := app["instances"].(float64); ok {
			instances = int(v)
		}

		var statusStr string
		switch state {
		case "running":
			statusStr = styles.Success.Render(state)
		case "stopped":
			statusStr = styles.Muted.Render(state)
		case "failed":
			statusStr = styles.Error.Render(state)
		default:
			statusStr = state
		}

		if len(image) > 24 {
			image = image[:21] + "..."
		}

		fmt.Printf("%-16s %-12s %-12d %-16s %-24s\n",
			styles.AppName.Render(truncate(name, 16)),
			statusStr,
			instances,
			"-",
			image)
	}

	return nil
}

// runRemoteDeploy deploys an app on the remote server.
func runRemoteDeploy(client *daemon.Client, args []string) error {
	// Parse config path from args
	configPath := ""
	appName := ""
	for i, arg := range args {
		if (arg == "--config" || arg == "-c") && i+1 < len(args) {
			configPath = args[i+1]
		}
		if arg == "--app" && i+1 < len(args) {
			appName = args[i+1]
		}
	}

	if configPath == "" {
		configPath = configFile
	}

	// Load and parse the local config
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	for _, appCfg := range cfg.Apps {
		if appName != "" && appCfg.Name != appName {
			continue
		}

		spin := progress.NewSpinner(fmt.Sprintf("Deploying %s remotely...", appCfg.Name))
		if !jsonOutput {
			spin.Start()
		}

		var result map[string]interface{}
		if err := client.Call("apps.deploy_config", &appCfg, &result); err != nil {
			if !jsonOutput {
				spin.Fail(fmt.Sprintf("Deploy failed: %v", err))
			}
			return err
		}

		if !jsonOutput {
			spin.Success(fmt.Sprintf("%s deployed successfully", styles.AppName.Render(appCfg.Name)))
		}
	}

	return nil
}

// runRemoteStop stops an app on the remote server.
func runRemoteStop(client *daemon.Client, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("app name required")
	}
	return client.Call("apps.stop", map[string]string{"name": args[0]}, nil)
}

// runRemoteRm removes an app on the remote server.
func runRemoteRm(client *daemon.Client, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("app name required")
	}
	return client.Call("apps.remove", map[string]string{"name": args[0]}, nil)
}

// runRemoteHealth shows health status on the remote server.
func runRemoteHealth(client *daemon.Client, args []string) error {
	var result map[string]interface{}
	if err := client.Call("health.all", nil, &result); err != nil {
		return fmt.Errorf("failed to get health status: %w", err)
	}

	if len(result) == 0 {
		fmt.Println("No monitored containers.")
		return nil
	}

	fmt.Printf("%-14s %-10s %-6s %-24s\n",
		styles.TableHeader.Render("CONTAINER"),
		styles.TableHeader.Render("STATUS"),
		styles.TableHeader.Render("FAILS"),
		styles.TableHeader.Render("MESSAGE"))
	fmt.Println(strings.Repeat("-", 56))

	for id, info := range result {
		infoMap, ok := info.(map[string]interface{})
		if !ok {
			continue
		}
		status, _ := infoMap["status"].(string)
		fails := 0
		if v, ok := infoMap["consecutive_failures"].(float64); ok {
			fails = int(v)
		}
		msg, _ := infoMap["last_message"].(string)

		var statusStr string
		switch status {
		case "healthy":
			statusStr = styles.Success.Render(status)
		case "unhealthy":
			statusStr = styles.Error.Render(status)
		default:
			statusStr = styles.Muted.Render(status)
		}

		shortID := id
		if len(id) > 12 {
			shortID = id[:12]
		}

		fmt.Printf("%-14s %-10s %-6d %-24s\n",
			styles.ContainerID.Render(shortID),
			statusStr,
			fails,
			truncate(msg, 24))
	}

	return nil
}

// runRemoteStats shows stats on the remote server.
func runRemoteStats(client *daemon.Client, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("app name required")
	}

	var containers []map[string]interface{}
	if err := client.Call("containers.list", map[string]string{"app_name": args[0]}, &containers); err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	fmt.Printf("%-14s %-8s %-12s %-12s\n",
		styles.TableHeader.Render("CONTAINER"),
		styles.TableHeader.Render("CPU %"),
		styles.TableHeader.Render("MEMORY"),
		styles.TableHeader.Render("PIDS"))
	fmt.Println(strings.Repeat("-", 48))

	for _, c := range containers {
		id, _ := c["id"].(string)
		if len(id) > 12 {
			id = id[:12]
		}

		// Try to get metrics for each container
		var metrics map[string]interface{}
		client.Call("metrics.latest", map[string]string{"container_id": id}, &metrics)

		cpu := "-"
		mem := "-"
		pids := "-"
		if metrics != nil {
			if v, ok := metrics["cpu_percent"].(float64); ok {
				cpu = fmt.Sprintf("%.1f%%", v)
			}
			if v, ok := metrics["memory_bytes"].(float64); ok {
				mem = humanBytes(int64(v))
			}
			if v, ok := metrics["pids_count"].(float64); ok {
				pids = fmt.Sprintf("%d", int(v))
			}
		}

		fmt.Printf("%-14s %-8s %-12s %-12s\n",
			styles.ContainerID.Render(id), cpu, mem, pids)
	}

	return nil
}

// runRemoteHistory shows deploy history on the remote server.
func runRemoteHistory(client *daemon.Client, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("app name required")
	}

	var deploys []map[string]interface{}
	if err := client.Call("apps.history", map[string]string{"name": args[0]}, &deploys); err != nil {
		return fmt.Errorf("failed to get history: %w", err)
	}

	if len(deploys) == 0 {
		fmt.Printf("No deploy history for %s.\n", args[0])
		return nil
	}

	fmt.Printf("%-8s %-10s %-30s %-12s\n",
		styles.TableHeader.Render("VERSION"),
		styles.TableHeader.Render("STATUS"),
		styles.TableHeader.Render("IMAGE"),
		styles.TableHeader.Render("STRATEGY"))
	fmt.Println(strings.Repeat("-", 62))

	for _, d := range deploys {
		ver := 0
		if v, ok := d["version"].(float64); ok {
			ver = int(v)
		}
		status, _ := d["status"].(string)
		image, _ := d["image"].(string)
		strategy, _ := d["strategy"].(string)

		var statusStr string
		switch status {
		case "active":
			statusStr = styles.Success.Render(status)
		case "failed":
			statusStr = styles.Error.Render(status)
		case "rolled_back":
			statusStr = styles.Warning.Render(status)
		default:
			statusStr = styles.Muted.Render(status)
		}

		if len(image) > 30 {
			image = image[:27] + "..."
		}

		fmt.Printf("%-8s %-10s %-30s %-12s\n",
			styles.Version.Render(fmt.Sprintf("v%d", ver)),
			statusStr,
			image,
			strategy)
	}

	return nil
}

// runRemoteLogs shows logs from the remote server (non-follow mode).
func runRemoteLogs(client *daemon.Client, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("app name required")
	}

	var logs string
	params := map[string]interface{}{
		"name": args[0],
		"tail": 50,
	}
	if err := client.Call("containers.logs", params, &logs); err != nil {
		return fmt.Errorf("failed to get logs: %w", err)
	}

	fmt.Print(logs)
	return nil
}

// runRemoteRollback rolls back an app on the remote server.
func runRemoteRollback(client *daemon.Client, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("app name required")
	}

	params := map[string]interface{}{
		"name":    args[0],
		"version": 0, // Previous version
	}

	// Check for --version flag
	for i, arg := range args {
		if arg == "--version" && i+1 < len(args) {
			params["version"] = args[i+1]
		}
	}

	return client.Call("apps.rollback", params, nil)
}

// shouldSyncConfig checks if the command requires syncing a config file.
func shouldSyncConfig(args []string) bool {
	for i, arg := range args {
		if arg == "deploy" {
			for j := i + 1; j < len(args); j++ {
				if args[j] == "--config" || args[j] == "-c" {
					return true
				}
			}
		}
	}
	return false
}

// getConfigFromArgs extracts the config file path from command args.
func getConfigFromArgs(args []string) string {
	for i, arg := range args {
		if (arg == "--config" || arg == "-c") && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

// replaceConfigPath replaces the config file path in args with a new path.
func replaceConfigPath(args []string, newPath string) []string {
	result := make([]string, len(args))
	copy(result, args)
	for i, arg := range result {
		if (arg == "--config" || arg == "-c") && i+1 < len(result) {
			result[i+1] = newPath
		}
	}
	return result
}

