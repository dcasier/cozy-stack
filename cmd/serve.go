package cmd

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"

	"github.com/dcasier/cozy-stack/config"
	"github.com/dcasier/cozy-stack/web"
)

// serveCmd represents the serve command
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Starts the stack and listens for HTTP calls",
	Long: `Starts the stack and listens for HTTP calls
It will accept HTTP requests on localhost:8080 by default.
Use the --port and --host flags to change the listening option.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := Configure(); err != nil {
			return err
		}

		router := getGin()
		web.SetupRoutes(router)

		addr := config.GetConfig().Host + ":" + strconv.Itoa(config.GetConfig().Port)
		return router.Run(addr)
	},
}

func init() {
	RootCmd.AddCommand(serveCmd)
}

func getGin() *gin.Engine {
	if config.GetConfig().Mode == config.Production {
		gin.SetMode(gin.ReleaseMode)
	}

	return gin.Default()
}
