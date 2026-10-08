package router

import (
	"github.com/gin-gonic/gin"
	"github.com/hefengxian/espulse/internal/handlers"
)

func SetupRouter() *gin.Engine {
	r := gin.Default()

	api := r.Group("/api")
	{
		clusters := api.Group("/clusters")
		{
			clusters.GET("", handlers.ListClusters)
			clusters.POST("", handlers.CreateCluster)
			clusters.POST("/probe", handlers.ProbeCluster)
			clusters.POST("/refresh", handlers.RefreshClusters)
			clusters.GET("/:id", handlers.GetCluster)
			clusters.PUT("/:id", handlers.UpdateCluster)
			clusters.DELETE("/:id", handlers.DeleteCluster)

			clusters.GET("/:id/overview", handlers.GetOverview)
			clusters.GET("/:id/nodes", handlers.ListNodes)
			clusters.GET("/:id/indices", handlers.ListIndices)
			clusters.GET("/:id/shards", handlers.ListShards)
		}

		api.Any("/proxy/*path", handlers.ProxyES)
	}

	return r
}
