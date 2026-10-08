package handlers

import (
	"database/sql"
	"io"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/hefengxian/espulse/internal/database"
	"github.com/hefengxian/espulse/internal/es"
	"github.com/hefengxian/espulse/internal/models"
)

// ProxyES 将请求透明转发到目标 Elasticsearch 集群，不做任何解析或封装。
func ProxyES(c *gin.Context) {
	clusterID := c.GetHeader("X-Cluster-ID")
	if clusterID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "X-Cluster-ID header is required"})
		return
	}

	// 1. 取出集群连接信息
	var cluster models.Cluster
	err := database.DB.QueryRow(
		"SELECT id, hosts, auth_type, username, password, api_key FROM clusters WHERE id = ?",
		clusterID,
	).Scan(
		&cluster.ID, &cluster.Hosts, &cluster.AuthType,
		&cluster.Username, &cluster.Password, &cluster.APIKey,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Cluster not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error: " + err.Error()})
		return
	}

	// 2. 构造转发请求（认证由请求层统一处理）
	// 使用不设超时的共享客户端：reindex、长查询不能被打断
	target := es.TargetFromCluster(cluster)
	proxyReq, err := target.NewRequest(c.Request.Method, c.Param("path"), c.Request.URL.Query(), c.Request.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 3. 透传客户端请求头，但不转发 X-Cluster-ID、Host 与 Authorization
	// （Authorization 必须由本服务按集群配置生成，避免被浏览器请求头覆盖）
	for name, values := range c.Request.Header {
		if name == "X-Cluster-ID" || name == "Host" || name == "Authorization" {
			continue
		}
		for _, value := range values {
			proxyReq.Header.Add(name, value)
		}
	}

	// 4. 执行请求
	resp, err := es.SharedClient.Do(proxyReq)
	if err != nil {
		log.Printf("Proxy error: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to connect to Elasticsearch"})
		return
	}
	defer resp.Body.Close()

	// 5. 原样回传响应
	for name, values := range resp.Header {
		for _, value := range values {
			c.Header(name, value)
		}
	}
	c.Status(resp.StatusCode)
	io.Copy(c.Writer, resp.Body)
}
