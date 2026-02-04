package cli

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/vessel/vessel/internal/runtime"
)

var imagesCmd = &cobra.Command{
	Use:   "images",
	Short: "List locally cached images",
	Long: `List all locally cached container images.

Examples:
  vessel images`,
	RunE: listImages,
}

func init() {
	rootCmd.AddCommand(imagesCmd)
}

func listImages(cmd *cobra.Command, args []string) error {
	// Create runtime to get the image store
	rt, err := runtime.NewLinuxRuntime()
	if err != nil {
		return fmt.Errorf("failed to create runtime: %w", err)
	}

	images, err := rt.GetImageStore().ListImages()
	if err != nil {
		return fmt.Errorf("failed to list images: %w", err)
	}

	if len(images) == 0 {
		fmt.Println("No images found.")
		return nil
	}

	// Print as a table
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "DIGEST\tREFERENCE\tSIZE\tLAYERS\tPULLED")

	for _, img := range images {
		// Truncate digest for display
		shortDigest := img.Digest
		if len(shortDigest) > 19 {
			shortDigest = shortDigest[:19] + "..."
		}

		// Format size
		sizeStr := formatBytes(img.Size)

		// Format time ago
		timeAgo := formatTimeAgo(img.PulledAt)

		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\n",
			shortDigest,
			img.Reference,
			sizeStr,
			img.Layers,
			timeAgo,
		)
	}
	w.Flush()

	return nil
}

// formatBytes formats bytes into a human-readable string.
func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

// formatTimeAgo formats a time as a human-readable "time ago" string.
func formatTimeAgo(t time.Time) string {
	d := time.Since(t)

	if d < time.Minute {
		return "just now"
	}
	if d < time.Hour {
		mins := int(d.Minutes())
		if mins == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", mins)
	}
	if d < 24*time.Hour {
		hours := int(d.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	}
	days := int(d.Hours() / 24)
	if days == 1 {
		return "1 day ago"
	}
	return fmt.Sprintf("%d days ago", days)
}
