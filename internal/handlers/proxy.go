package handlers

import (
	"database/sql"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/hefengxian/espulse/internal/database"
	"github.com/hefengxian/espulse/internal/es"
	"github.com/hefengxian/espulse/internal/models"
)

// maxProxyBodyBytes 限制信封内 payload 的大小。
// 改信封后 body 会整体进内存，需要一个上限防止滥用（原流式代理在超过 1MiB 时会退化为只打首个 host）。
const maxProxyBodyBytes = 10 << 20 // 10 MiB

// proxyRequest 是 Dev Console 构建请求时的固定信封：
//   - Path   命令首行去掉动词后的部分，可携带 query string，如 /_cat/shards?v&h=index,shard
//   - Method 命令行的动词，决定打到 ES 的 HTTP 方法
//   - Body   命令行下方的原始请求体文本；不一定是 JSON（_bulk 是 NDJSON），故按字符串透传
type proxyRequest struct {
	Path   string `json:"path"`
	Method string `json:"method"`
	Body   string `json:"body"`
}

// allowedProxyMethods 是允许透传到 ES 的 HTTP 方法白名单，避免用户构造 CONNECT、TRACE 等。
var allowedProxyMethods = map[string]bool{
	http.MethodGet:    true,
	http.MethodPost:   true,
	http.MethodPut:    true,
	http.MethodDelete: true,
	http.MethodHead:   true,
	http.MethodPatch:  true,
}

// ProxyES 将 Dev Console 构建的请求转发到目标 Elasticsearch 集群，不做任何解析或封装。
func ProxyES(c *gin.Context) {
	clusterID := c.GetHeader("X-Cluster-ID")
	if clusterID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "X-Cluster-ID header is required"})
		return
	}

	// 1. 解析固定信封
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxProxyBodyBytes)
	var in proxyRequest
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}

	method := strings.ToUpper(strings.TrimSpace(in.Method))
	if !allowedProxyMethods[method] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unsupported method: " + in.Method})
		return
	}

	// path 可能携带 query string，必须在此拆开交给 es.Request，
	// 否则会被 es 层的 url.JoinPath 当作 path 的一部分转义掉，query 就丢了。
	u, err := url.Parse(in.Path)
	if err != nil || u.Host != "" || !strings.HasPrefix(u.Path, "/") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid path: " + in.Path})
		return
	}

	// 2. 取出集群连接信息
	var cluster models.Cluster
	err = database.DB.QueryRow(
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

	// 3. 构造转发请求（认证由请求层统一处理）
	// 使用不设超时的共享客户端：reindex、长查询不能被打断。
	target := es.TargetFromCluster(cluster)

	// body 已是内存中的字符串，每次尝试 host 都返回全新的读取器即可重放，天然支持故障转移。
	var body func() (io.Reader, error)
	if in.Body != "" {
		payload := in.Body
		body = func() (io.Reader, error) { return strings.NewReader(payload), nil }
	}

	// 4. 透传客户端请求头，但不转发 X-Cluster-ID、Host、Authorization（认证由本服务生成）
	// 以及信封自身的 Content-Type / Content-Length 等逐跳或会误导 ES 的头。
	copyHeaders := func(req *http.Request) {
		for name, values := range c.Request.Header {
			switch http.CanonicalHeaderKey(name) {
			case "X-Cluster-Id", "Host", "Authorization",
				"Content-Type", "Content-Length", "Connection", "Transfer-Encoding":
				continue
			}
			for _, value := range values {
				req.Header.Add(name, value)
			}
		}
		if in.Body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
	}

	// 5. 执行请求（依次尝试集群的各个 host）
	resp, err := target.Do(es.SharedClient, es.Request{
		Method: method,
		Path:   u.Path,
		Query:  u.Query(),
		Body:   body,
	}, copyHeaders)
	if err != nil {
		log.Printf("Proxy error: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to connect to Elasticsearch"})
		return
	}
	defer resp.Body.Close()

	// 6. 原样回传响应
	for name, values := range resp.Header {
		for _, value := range values {
			c.Header(name, value)
		}
	}
	c.Status(resp.StatusCode)
	io.Copy(c.Writer, resp.Body)
}
