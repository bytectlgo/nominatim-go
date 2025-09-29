package cmd

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/spf13/cobra"
)

var (
	waitURL     string
	waitTimeout time.Duration
)

// waitreadyCmd waits until nominatim-go reports healthy status
var waitreadyCmd = &cobra.Command{
	Use:   "waitready",
	Short: "等待 nominatim-go /status 就绪（部署编排用）",
	RunE: func(cmd *cobra.Command, args []string) error {
		deadline := time.Now().Add(waitTimeout)
		for time.Now().Before(deadline) {
			req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, waitURL, nil)
			resp, err := http.DefaultClient.Do(req)
			if err == nil && resp.StatusCode == 200 {
				_ = resp.Body.Close()
				return nil
			}
			if resp != nil {
				_ = resp.Body.Close()
			}
			time.Sleep(2 * time.Second)
		}
		return fmt.Errorf("waitready 超时：%s", waitURL)
	},
}

func init() {
	waitreadyCmd.Flags().StringVar(&waitURL, "url", "http://127.0.0.1:8000/status", "就绪探针 URL")
	waitreadyCmd.Flags().DurationVar(&waitTimeout, "timeout", 10*time.Minute, "等待超时")
	rootCmd.AddCommand(waitreadyCmd)
}
