package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/vessel/vessel/internal/config"
	"github.com/vessel/vessel/internal/daemon"
	"github.com/vessel/vessel/internal/store"
)

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy an application",
	Long: `Deploy an application from the vessel.toml configuration file.

If --app is specified, only that app is deployed.
If --all is specified, all apps in the config are deployed.
Without flags, deploys all apps in the config.

Examples:
  vessel deploy                          # Deploy all apps in vessel.toml
  vessel deploy --app my-api             # Deploy only 'my-api'
  vessel deploy --all                    # Explicitly deploy all apps
  vessel deploy --app my-api --image myorg/api:v2.0  # Override image
  vessel deploy --app my-api --strategy blue-green    # Override strategy`,
	RunE: runDeploy,
}

func init() {
	deployCmd.Flags().Bool("all", false, "deploy all apps in config")
	deployCmd.Flags().String("app", "", "deploy only this app")
	deployCmd.Flags().String("image", "", "override the image from config")
	deployCmd.Flags().String("strategy", "", "override deploy strategy (rolling, blue-green)")
}

func runDeploy(cmd *cobra.Command, args []string) error {
	appName, _ := cmd.Flags().GetString("app")
	imageOverride, _ := cmd.Flags().GetString("image")
	strategyOverride, _ := cmd.Flags().GetString("strategy")

	// Parse config
	cfg, err := config.Load(configFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if len(cfg.Apps) == 0 {
		return fmt.Errorf("no apps defined in %s", configFile)
	}

	// Determine which apps to deploy
	var appsToDeploy []config.AppConfig
	if appName != "" {
		// Deploy a specific app
		for _, app := range cfg.Apps {
			if app.Name == appName {
				appsToDeploy = append(appsToDeploy, app)
				break
			}
		}
		if len(appsToDeploy) == 0 {
			return fmt.Errorf("app %q not found in %s", appName, configFile)
		}
	} else {
		// Deploy all apps
		appsToDeploy = cfg.Apps
	}

	// Apply overrides
	for i := range appsToDeploy {
		if imageOverride != "" {
			appsToDeploy[i].Image = imageOverride
		}
		if strategyOverride != "" {
			appsToDeploy[i].Deploy.Strategy = strategyOverride
		}
	}

	client := daemon.NewClient("")

	// Deploy each app
	var lastErr error
	for _, appCfg := range appsToDeploy {
		if err := deployOneApp(client, &appCfg); err != nil {
			lastErr = err
		}
	}

	return lastErr
}

func deployOneApp(client *daemon.Client, appCfg *config.AppConfig) error {
	fmt.Printf("Deploying %s (image: %s, strategy: %s, instances: %d)...\n",
		appCfg.Name, appCfg.Image, appCfg.Deploy.Strategy, appCfg.Instances)

	var deploy store.Deploy
	err := client.Call("apps.deploy_config", appCfg, &deploy)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  x %s deploy failed: %v\n", appCfg.Name, err)
		return err
	}

	if jsonOutput {
		data, _ := json.MarshalIndent(deploy, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	if deploy.Status == store.DeployStatusFailed {
		fmt.Fprintf(os.Stderr, "  x %s deploy failed: %s. Old version still running.\n",
			appCfg.Name, deploy.Error)
		return fmt.Errorf("deploy failed for %s", appCfg.Name)
	}

	fmt.Printf("  > %s deployed successfully (v%d)\n", appCfg.Name, deploy.Version)
	return nil
}
