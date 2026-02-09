package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/vessel/vessel/internal/cli/styles"
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
  vessel deploy --app my-api --strategy blue-green    # Override strategy
  vessel deploy --env FOO=bar --env BAZ=qux           # Set env vars
  vessel deploy --env-file .env                        # Load env from file`,
	RunE: runDeploy,
}

func init() {
	deployCmd.Flags().Bool("all", false, "deploy all apps in config")
	deployCmd.Flags().String("app", "", "deploy only this app")
	deployCmd.Flags().String("image", "", "override the image from config")
	deployCmd.Flags().String("strategy", "", "override deploy strategy (rolling, blue-green)")
	deployCmd.Flags().StringArray("env", nil, "set environment variable (KEY=VALUE, can be repeated)")
	deployCmd.Flags().String("env-file", "", "path to .env file")
	deployCmd.Flags().Bool("continue-on-error", false, "continue deploying remaining apps if one fails")
}

func runDeploy(cmd *cobra.Command, args []string) error {
	appName, _ := cmd.Flags().GetString("app")
	imageOverride, _ := cmd.Flags().GetString("image")
	strategyOverride, _ := cmd.Flags().GetString("strategy")
	envFlags, _ := cmd.Flags().GetStringArray("env")
	envFile, _ := cmd.Flags().GetString("env-file")
	continueOnError, _ := cmd.Flags().GetBool("continue-on-error")

	// Parse config
	cfg, err := config.Load(configFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if len(cfg.Apps) == 0 {
		return fmt.Errorf("no apps defined in %s", configFile)
	}

	// Parse --env-file if provided
	var envFileVars map[string]string
	if envFile != "" {
		envFileVars, err = config.ParseEnvFile(envFile)
		if err != nil {
			return fmt.Errorf("failed to parse env file: %w", err)
		}
	}

	// Parse --env flags
	var cliEnvVars map[string]string
	if len(envFlags) > 0 {
		cliEnvVars, err = config.ParseCLIEnvFlags(envFlags)
		if err != nil {
			return fmt.Errorf("invalid --env flag: %w", err)
		}
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

	// Apply overrides and merge env vars
	for i := range appsToDeploy {
		if imageOverride != "" {
			appsToDeploy[i].Image = imageOverride
		}
		if strategyOverride != "" {
			appsToDeploy[i].Deploy.Strategy = strategyOverride
		}

		// Merge env: config env < .env file < CLI flags
		// (image env is handled by the runtime layer)
		appsToDeploy[i].Env = config.MergeEnv(
			appsToDeploy[i].Env,
			envFileVars,
			cliEnvVars,
		)
	}

	client := daemon.NewClient("")

	// Deploy each app
	type deployResult struct {
		name     string
		oldVer   int
		newVer   int
		duration time.Duration
		err      error
	}
	var results []deployResult

	for _, appCfg := range appsToDeploy {
		start := time.Now()
		oldVer := getAppVersion(client, appCfg.Name)
		err := deployOneApp(client, &appCfg)
		dur := time.Since(start)

		newVer := getAppVersion(client, appCfg.Name)
		results = append(results, deployResult{
			name:     appCfg.Name,
			oldVer:   oldVer,
			newVer:   newVer,
			duration: dur,
			err:      err,
		})

		if err != nil && !continueOnError {
			break
		}
	}

	// Print summary if deploying multiple apps
	if len(appsToDeploy) > 1 {
		fmt.Println()
		fmt.Println(styles.Header.Render("Deploy Summary:"))
		for _, r := range results {
			if r.err != nil {
				fmt.Printf("  %s %-15s v%d -> v%d  %s (%v)\n",
					styles.ErrorIcon(), styles.AppName.Render(r.name),
					r.oldVer, r.newVer, styles.Error.Render("FAILED"), r.err)
			} else {
				fmt.Printf("  %s %-15s v%d -> v%d  (%s)\n",
					styles.SuccessIcon(), styles.AppName.Render(r.name),
					r.oldVer, r.newVer, r.duration.Round(time.Second))
			}
		}
	}

	// Return error if any deploy failed
	for _, r := range results {
		if r.err != nil {
			return fmt.Errorf("one or more deploys failed")
		}
	}

	return nil
}

// getAppVersion returns the current deploy version for an app, or 0 if not found.
func getAppVersion(client *daemon.Client, appName string) int {
	var deploys []store.Deploy
	err := client.Call("apps.history", map[string]string{"name": appName}, &deploys)
	if err != nil || len(deploys) == 0 {
		return 0
	}
	maxVer := 0
	for _, d := range deploys {
		if d.Version > maxVer {
			maxVer = d.Version
		}
	}
	return maxVer
}

func deployOneApp(client *daemon.Client, appCfg *config.AppConfig) error {
	fmt.Printf("Deploying %s (image: %s, strategy: %s, instances: %d)...\n",
		appCfg.Name, appCfg.Image, appCfg.Deploy.Strategy, appCfg.Instances)

	var deploy store.Deploy
	err := client.Call("apps.deploy_config", appCfg, &deploy)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  %s %s deploy failed: %v\n",
			styles.ErrorIcon(), styles.AppName.Render(appCfg.Name), err)
		return err
	}

	if jsonOutput {
		data, _ := json.MarshalIndent(deploy, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	if deploy.Status == store.DeployStatusFailed {
		fmt.Fprintf(os.Stderr, "  %s %s deploy failed: %s. Old version still running.\n",
			styles.ErrorIcon(), styles.AppName.Render(appCfg.Name), deploy.Error)
		return fmt.Errorf("deploy failed for %s", appCfg.Name)
	}

	fmt.Printf("  %s %s deployed successfully (%s)\n",
		styles.SuccessIcon(), styles.AppName.Render(appCfg.Name),
		styles.Version.Render(fmt.Sprintf("v%d", deploy.Version)))
	return nil
}
