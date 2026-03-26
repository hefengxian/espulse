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
			clusters.GET("/:id", handlers.GetCluster)
			clusters.DELETE("/:id", handlers.DeleteCluster)
		}
	}

	return r
}
