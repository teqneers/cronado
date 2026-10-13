package cmd

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/teqneers/cronado/internal/context"
)

// healthcheckCmd queries the /healthz endpoint of a running cronado. It is
// used as the Docker HEALTHCHECK, so it exits 0 when healthy and 1 otherwise.
var healthcheckCmd = &cobra.Command{
	Use:   "healthcheck",
	Short: "Check the health of a running cronado instance",
	Long:  "Queries /healthz of the running scheduler and exits with 0 when healthy, 1 otherwise.",
	Run: func(cmd *cobra.Command, args []string) {
		cfg := context.AppCtx.Config.ServerConfig
		url := fmt.Sprintf("http://%s/healthz", net.JoinHostPort(localHost(cfg.Host), cfg.Port))

		client := &http.Client{Timeout: 3 * time.Second}
		resp, err := client.Get(url)
		if err != nil {
			fmt.Fprintf(os.Stderr, "unhealthy: %v\n", err)
			os.Exit(1)
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			fmt.Fprintf(os.Stderr, "unhealthy: %s\n", resp.Status)
			os.Exit(1)
		}
		fmt.Println("healthy")
	},
}

// localHost returns an address to reach a server listening on host from the
// same machine; wildcard addresses are reached via loopback.
func localHost(host string) string {
	switch host {
	case "", "0.0.0.0", "::":
		return "127.0.0.1"
	}
	return host
}

func init() {
	rootCmd.AddCommand(healthcheckCmd)
}
