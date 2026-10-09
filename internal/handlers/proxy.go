package handlers

import (
	"bytes"
	"database/sql"
	"io"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/hefengxian/espulse/internal/database"
	"github.com/hefengxian/espulse/internal/es"
	"github.com/hefengxian/espulse/internal/models"
)

// maxReplayBodyBytes 是代理请求体可缓冲的上限。
// 超过它的请求不再缓冲、只尝试首个 host —— 在「多 host 重试」与「_bulk 大写入不被吃进内存」之间取平衡。
const maxReplayBodyBytes = 1 << 20 // 1 MiB

// replayableBody 把流式请求体包装成可重复调用的工厂函数，供故障转移时重放。
// body 为空时返回 (nil, true, nil)；超过上限时返回 (只读一次的工厂, false, nil)。
func replayableBody(body io.ReadCloser, limit int64) (func() (io.Reader, error), bool, error) {
	if body == nil {
		return nil, true, nil
	}

	buf, err := io.ReadAll(io.LimitReader(body, limit+1))
	if err != nil {
		return nil, false, err
	}

	if int64(len(buf)) > limit {
		// 太大：不缓冲，把已读部分与剩余流拼回去，只允许这次请求使用
		used := false
		return func() (io.Reader, error) {
			if used {
				return nil, io.EOF
			}
			used = true
			return io.MultiReader(bytes.NewReader(buf), body), nil
		}, false, nil
	}

	if len(buf) == 0 {
		return nil, true, nil
	}

	return func() (io.Reader, error) {
		return bytes.NewReader(buf), nil
	}, true, nil
}

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

	// 请求体是流式的，读一次就没了，而故障转移会把同一个请求发往多个 host。
	// 因此在体量可控时先缓冲一份；超出上限（如 _bulk 大批量写入）则不缓冲，
	// 该请求退化为「只打首个 host」，以免把大请求整个吃进内存。
	body, replayable, err := replayableBody(c.Request.Body, maxReplayBodyBytes)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 3. 透传客户端请求头，但不转发 X-Cluster-ID、Host 与 Authorization
	// （Authorization 必须由本服务按集群配置生成，避免被浏览器请求头覆盖）
	copyHeaders := func(req *http.Request) {
		for name, values := range c.Request.Header {
			if name == "X-Cluster-ID" || name == "Host" || name == "Authorization" {
				continue
			}
			for _, value := range values {
				req.Header.Add(name, value)
			}
		}
	}

	// 4. 执行请求（依次尝试集群的各个 host）
	resp, err := target.Do(es.SharedClient, es.Request{
		Method:  c.Request.Method,
		Path:    c.Param("path"),
		Query:   c.Request.URL.Query(),
		Body:    body,
		NoRetry: !replayable,
	}, copyHeaders)
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
