package main

import (
	"fmt"
	"log"

	// "github.com/gin-gonic/gin"
	"github.com/hefengxian/espulse/internal/database"
	"github.com/hefengxian/espulse/internal/router"
)

func main() {
	// 初始化数据库 (开发环境下放在当前目录 data 文件夹)
	if err := database.InitDB("./data"); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	r := router.SetupRouter()

	addr := "0.0.0.0:18080"
	fmt.Printf("ESPulse backend starting on http://%v ...\n", addr)
	r.Run(addr)
}
