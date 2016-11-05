// Package web Cozy Stack API.
//
// Cozy is a personal platform as a service with a focus on data.
//
// Terms Of Service:
//
// there are no TOS at this moment, use at your own risk we take no responsibility
//
//     Schemes: https
//     Host: localhost
//     BasePath: /
//     Version: 0.0.1
//     License: AGPL-3.0 https://opensource.org/licenses/agpl-3.0
//     Contact: Bruno Michel <bruno@cozycloud.cc> https://cozy.io/
//
//     Consumes:
//     - application/json
//
//     Produces:
//     - application/json
//
// swagger:meta
package web

import (
	"github.com/dcasier/cozy-stack/web/apps"
	"github.com/dcasier/cozy-stack/web/data"
	"github.com/dcasier/cozy-stack/web/files"
	"github.com/dcasier/cozy-stack/web/middlewares"
	"github.com/dcasier/cozy-stack/web/status"
	"github.com/dcasier/cozy-stack/web/version"
	"github.com/gin-gonic/gin"
)

// SetupRoutes sets the routing for HTTP endpoints to the Go methods
func SetupRoutes(router *gin.Engine) {
	router.Use(middlewares.SetInstance())
	router.Use(middlewares.ErrorHandler())
	apps.Routes(router.Group("/apps"))
	data.Routes(router.Group("/data"))
	files.Routes(router.Group("/files"))
	status.Routes(router.Group("/status"))
	version.Routes(router.Group("/version"))
}
