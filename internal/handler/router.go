package handler

import (
	"net/http"
	"runtime/debug"

	"bot-go/internal/controller"
	"bot-go/pkg/mcp"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func SetupRouter(repoController *controller.RepoController, mcpServer *mcp.CodeGraphServer, logger *zap.Logger) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()
	router.Use(CustomRecoveryMiddleware(logger))
	router.Use(LoggerMiddleware(logger))

	v1 := router.Group("/api/v1")
	{
		v1.POST("/processRepo", repoController.ProcessRepo)
		//v1.POST("/getFunctionsInFile", repoController.GetFunctionsInFile)
		//v1.POST("/getFunctionDetails", repoController.GetFunctionDetails)
		v1.POST("/functionDependencies", repoController.GetFunctionDependencies)
		v1.POST("/processDirectory", repoController.ProcessDirectory)
		v1.POST("/searchSimilarCode", repoController.SearchSimilarCode)
		v1.GET("/health", func(c *gin.Context) {
			c.JSON(200, gin.H{
				"status": "healthy",
			})
		})
	}

	// Setup MCP routes
	mcpServer.SetupHTTPRoutes(router)

	return router
}

func LoggerMiddleware(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		logger.Info("HTTP Request",
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.String("client_ip", c.ClientIP()),
		)
		c.Next()
	}
}

func CustomRecoveryMiddleware(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				logger.Error("Panic recovered",
					zap.Any("error", err),
					zap.String("stack", string(debug.Stack())),
					zap.String("path", c.Request.URL.Path),
					zap.String("method", c.Request.Method),
				)
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": "Internal server error",
				})
				c.Abort()
			}
		}()
		c.Next()
	}
}
