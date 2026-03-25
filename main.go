package main

import (
	"fmt"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/hefengxian/espulse/internal/database"
)

func main() {
	// 初始化数据库 (开发环境下放在当前目录 data 文件夹)
	if err := database.InitDB("./data"); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	r := gin.Default()
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})
	fmt.Println("ESPulse backend starting on http://localhost:18080...")
	r.Run(":18080")
}
