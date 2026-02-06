package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/vessel/vessel/internal/daemon"
)

var certCmd = &cobra.Command{
	Use:   "cert",
	Short: "Manage TLS certificates",
	Long: `Manage TLS certificates for Vessel's reverse proxy.

Commands:
  vessel cert list                     List loaded certificates
  vessel cert import <hostname>        Import a custom certificate`,
}

var certListCmd = &cobra.Command{
	Use:   "list",
	Short: "List loaded certificates",
	Long: `List all TLS certificates currently loaded in the reverse proxy.

Examples:
  vessel cert list
  vessel cert list --json`,
	RunE: runCertList,
}

var certImportCmd = &cobra.Command{
	Use:   "import <hostname>",
	Short: "Import a custom TLS certificate",
	Long: `Import a custom TLS certificate and key for a hostname.

The certificate and key files must be in PEM format.

Examples:
  vessel cert import example.com --cert cert.pem --key key.pem`,
	Args: cobra.ExactArgs(1),
	RunE: runCertImport,
}

func init() {
	certImportCmd.Flags().String("cert", "", "path to certificate file (PEM format)")
	certImportCmd.Flags().String("key", "", "path to private key file (PEM format)")
	certImportCmd.MarkFlagRequired("cert")
	certImportCmd.MarkFlagRequired("key")

	certCmd.AddCommand(certListCmd, certImportCmd)
}

func runCertList(cmd *cobra.Command, args []string) error {
	client := daemon.NewClient("")

	var result struct {
		TLSEnabled bool     `json:"tls_enabled"`
		ACME       bool     `json:"acme"`
		Certs      []string `json:"certs"`
	}

	if err := client.Call("cert.list", nil, &result); err != nil {
		return fmt.Errorf("failed to list certificates: %w", err)
	}

	if jsonOutput {
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	fmt.Println("TLS Certificates")
	fmt.Println("-----------------")
	fmt.Printf("TLS Enabled: %v\n", result.TLSEnabled)
	fmt.Printf("ACME:        %v\n", result.ACME)
	fmt.Println()

	if len(result.Certs) == 0 {
		fmt.Println("No custom certificates loaded.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "HOSTNAME\tTYPE")
	fmt.Fprintln(w, "--------\t----")
	for _, hostname := range result.Certs {
		fmt.Fprintf(w, "%s\tcustom\n", hostname)
	}
	w.Flush()
	return nil
}

func runCertImport(cmd *cobra.Command, args []string) error {
	hostname := args[0]
	certPath, _ := cmd.Flags().GetString("cert")
	keyPath, _ := cmd.Flags().GetString("key")

	// Validate files exist
	if _, err := os.Stat(certPath); err != nil {
		return fmt.Errorf("certificate file not found: %s", certPath)
	}
	if _, err := os.Stat(keyPath); err != nil {
		return fmt.Errorf("key file not found: %s", keyPath)
	}

	client := daemon.NewClient("")

	params := map[string]string{
		"hostname":  hostname,
		"cert_path": certPath,
		"key_path":  keyPath,
	}

	var result struct {
		Status string `json:"status"`
	}

	if err := client.Call("cert.import", params, &result); err != nil {
		return fmt.Errorf("failed to import certificate: %w", err)
	}

	fmt.Printf("Certificate imported for %s\n", hostname)
	return nil
}
