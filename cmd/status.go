package cmd

import (
	"fmt"
	"net/http"
	"net/url"
	"os"

	"github.com/dcasier/cozy-stack/config"
	"github.com/spf13/cobra"
	"strconv"
)

// statusCmd represents the status command
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check if the HTTP server is running",
	Long:  `Check if the HTTP server has been started and answer 200 for /status.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := Configure(); err != nil {
			return err
		}

		url := &url.URL{
			Scheme: "http",
			Host:   config.GetConfig().Host + ":" + strconv.Itoa(config.GetConfig().Port),
			Path:   "status",
		}
		resp, err := http.Get(url.String())
		if err != nil {
			fmt.Println("Error the HTTP server is not running:", err)
			os.Exit(1)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			fmt.Println("Error, unexpected HTTP status code:", resp.Status)
			os.Exit(1)
		}

		fmt.Println("OK, the HTTP server is ready.")
		return nil
	},
}

func init() {
	RootCmd.AddCommand(statusCmd)
}
