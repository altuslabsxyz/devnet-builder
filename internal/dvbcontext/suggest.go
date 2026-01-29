package dvbcontext

import (
	"context"
	"fmt"
	"strings"

	"github.com/altuslabsxyz/devnet-builder/internal/client"
)

// SuggestUsage returns formatted suggestion text based on available devnets.
// Returns empty string if client is nil or if there's an error fetching devnets.
func SuggestUsage(c *client.Client) string {
	if c == nil {
		return ""
	}

	devnets, err := c.ListDevnets(context.Background(), "")
	if err != nil {
		return ""
	}

	if len(devnets) == 0 {
		return "No devnets found. Create one first:\n  dvb init my-devnet"
	}

	if len(devnets) == 1 {
		d := devnets[0]
		name := d.Metadata.Name
		if d.Metadata.Namespace != "" && d.Metadata.Namespace != "default" {
			name = d.Metadata.Namespace + "/" + d.Metadata.Name
		}
		return fmt.Sprintf("Found 1 devnet: %s\n\nRun: dvb use %s", name, name)
	}

	// Multiple devnets
	var sb strings.Builder
	sb.WriteString("Available devnets:\n")
	for _, d := range devnets {
		name := d.Metadata.Name
		if d.Metadata.Namespace != "" && d.Metadata.Namespace != "default" {
			name = d.Metadata.Namespace + "/" + d.Metadata.Name
		}
		phase := ""
		if d.Status != nil && d.Status.Phase != "" {
			phase = fmt.Sprintf(" (%s)", strings.ToLower(d.Status.Phase))
		}
		sb.WriteString(fmt.Sprintf("  %s%s\n", name, phase))
	}
	sb.WriteString("\nRun: dvb use <devnet>")
	return sb.String()
}
