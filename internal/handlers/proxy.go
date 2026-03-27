package handlers

import (
	"crypto/tls"
	"database/sql"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/hefengxian/espulse/internal/database"
	"github.com/hefengxian/espulse/internal/models"
)

var httpClient = &http.Client{
	Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // Allow self-signed certs for ES
	},
}

// ProxyES forwards requests to the target Elasticsearch cluster
func ProxyES(c *gin.Context) {
	clusterID := c.GetHeader("X-Cluster-ID")
	if clusterID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "X-Cluster-ID header is required"})
		return
	}

	// 1. Fetch cluster info from DB
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
		// 真正的数据库异常
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error: " + err.Error()})
		return
	}

	if len(cluster.Hosts) == 0 {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Cluster has no hosts configured"})
		return
	}

	// 2. Select host (currently just the first one)
	targetHost := cluster.Hosts[0]
	if !strings.HasPrefix(targetHost, "http://") && !strings.HasPrefix(targetHost, "https://") {
		targetHost = "http://" + targetHost
	}

	// 3. Construct target URL
	// The path in Gin will be like "/proxy/_search" if registered as "/proxy/*path"
	// and path variable will be "/_search"
	path := c.Param("path")
	targetURL, err := url.Parse(targetHost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid cluster host URL"})
		return
	}
	// 使用更安全的方式拼接路径和查询参数
	targetURL.Path, err = url.JoinPath(targetURL.Path, path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to join path"})
		return
	}
	targetURL.RawQuery = c.Request.URL.RawQuery

	// 4. Create request to ES
	proxyReq, err := http.NewRequest(c.Request.Method, targetURL.String(), c.Request.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create proxy request"})
		return
	}

	// 5. Copy headers and set authentication
	for name, values := range c.Request.Header {
		// Skip headers that should not be forwarded
		if name == "X-Cluster-ID" || name == "Host" {
			continue
		}
		for _, value := range values {
			proxyReq.Header.Add(name, value)
		}
	}

	switch cluster.AuthType {
	case "basic":
		proxyReq.SetBasicAuth(cluster.Username, cluster.Password)
	case "api_key":
		proxyReq.Header.Set("Authorization", "ApiKey "+cluster.APIKey)
	}

	// 6. Execute request
	resp, err := httpClient.Do(proxyReq)
	if err != nil {
		log.Printf("Proxy error: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to connect to Elasticsearch"})
		return
	}
	defer resp.Body.Close()

	// 7. Copy response headers and body
	for name, values := range resp.Header {
		for _, value := range values {
			c.Header(name, value)
		}
	}
	c.Status(resp.StatusCode)
	io.Copy(c.Writer, resp.Body)
}
