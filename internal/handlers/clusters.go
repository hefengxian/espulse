package handlers

import (
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/hefengxian/espulse/internal/database"
	"github.com/hefengxian/espulse/internal/es"
	"github.com/hefengxian/espulse/internal/models"
)

// clusterWithStatus 是列表接口的返回结构：集群配置 + 最近一次采集快照。
// status 可能缺失（尚未采集），前端需要能处理这种情况。
type clusterWithStatus struct {
	models.Cluster
	Status *es.Status `json:"status,omitempty"`
}

// ListClusters 返回所有集群，并附带共享采集缓存中的状态。
// 读取会把集群标记为活跃，从而按需触发采集（见 PRD §6.3）。
func ListClusters(c *gin.Context) {
	rows, err := database.DB.Query("SELECT id, name, hosts, auth_type, username, color, notes, created_at, updated_at FROM clusters ORDER BY created_at DESC")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	clusters := []models.Cluster{}
	ids := []string{}

	for rows.Next() {
		var cluster models.Cluster
		err := rows.Scan(
			&cluster.ID, &cluster.Name, &cluster.Hosts, &cluster.AuthType,
			&cluster.Username, &cluster.Color, &cluster.Notes,
			&cluster.CreatedAt, &cluster.UpdatedAt,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		clusters = append(clusters, cluster)
		ids = append(ids, cluster.ID)
	}

	// 读取共享采集缓存：无缓存的集群会同步采集一次，多人同时访问也只触发一次（见 PRD §6.3）
	snapshots := es.StatusSnapshot(ids)

	items := make([]clusterWithStatus, 0, len(clusters))
	for _, cluster := range clusters {
		item := clusterWithStatus{Cluster: cluster}
		if snapshot, ok := snapshots[cluster.ID]; ok {
			status := snapshot
			item.Status = &status
		}
		items = append(items, item)
	}

	c.JSON(http.StatusOK, items)
}

// GetCluster 返回单个集群。
func GetCluster(c *gin.Context) {
	id := c.Param("id")
	var cluster models.Cluster
	err := database.DB.QueryRow(
		"SELECT id, name, hosts, auth_type, username, color, notes, created_at, updated_at FROM clusters WHERE id = ?",
		id,
	).Scan(
		&cluster.ID, &cluster.Name, &cluster.Hosts, &cluster.AuthType,
		&cluster.Username, &cluster.Color, &cluster.Notes,
		&cluster.CreatedAt, &cluster.UpdatedAt,
	)

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Cluster not found"})
		return
	}

	c.JSON(http.StatusOK, cluster)
}

// ProbeCluster 在保存前探测集群连通性，不写入数据库。
// 探测失败属于业务结果而非服务异常，因此同样返回 200，由前端展示原因。
func ProbeCluster(c *gin.Context) {
	var req struct {
		Hosts    models.StringArray `json:"hosts"`
		AuthType string             `json:"auth_type"`
		Username string             `json:"username"`
		Password string             `json:"password"`
		APIKey   string             `json:"api_key"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	info, err := es.Probe(es.Target{
		Hosts:    req.Hosts,
		AuthType: req.AuthType,
		Username: req.Username,
		Password: req.Password,
		APIKey:   req.APIKey,
	})
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"reachable": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"reachable": true, "info": info})
}

// CreateCluster 新增集群。落库前强制探测，避免写入连不上的死记录。
func CreateCluster(c *gin.Context) {
	var cluster models.Cluster
	if err := c.ShouldBindJSON(&cluster); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := verifyConnectable(cluster); err != nil {
		c.JSON(http.StatusOK, gin.H{"saved": false, "error": err.Error()})
		return
	}

	cluster.ID = uuid.New().String()
	cluster.CreatedAt = time.Now()
	cluster.UpdatedAt = time.Now()

	_, err := database.DB.Exec(
		"INSERT INTO clusters (id, name, hosts, auth_type, username, password, api_key, color, notes, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		cluster.ID, cluster.Name, cluster.Hosts, cluster.AuthType,
		cluster.Username, cluster.Password, cluster.APIKey,
		cluster.Color, cluster.Notes, cluster.CreatedAt, cluster.UpdatedAt,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 立即采集一次，避免新增后卡片长时间没有状态
	es.CollectOne(cluster.ID)

	c.JSON(http.StatusOK, gin.H{"saved": true, "cluster": cluster})
}

// UpdateCluster 更新集群。
// 只有连接信息（hosts / 认证）变更时才强制探测，这样集群临时不可达时仍可改名称或备注。
func UpdateCluster(c *gin.Context) {
	id := c.Param("id")

	var incoming models.Cluster
	if err := c.ShouldBindJSON(&incoming); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var existing models.Cluster
	err := database.DB.QueryRow(
		"SELECT id, hosts, auth_type, username, password, api_key, created_at FROM clusters WHERE id = ?",
		id,
	).Scan(
		&existing.ID, &existing.Hosts, &existing.AuthType,
		&existing.Username, &existing.Password, &existing.APIKey,
		&existing.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Cluster not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 密码 / API Key 留空表示不修改，避免把密钥回传前端后再提交
	if incoming.Password == "" {
		incoming.Password = existing.Password
	}
	if incoming.APIKey == "" {
		incoming.APIKey = existing.APIKey
	}

	if connectionChanged(existing, incoming) {
		if err := verifyConnectable(incoming); err != nil {
			c.JSON(http.StatusOK, gin.H{"saved": false, "error": err.Error()})
			return
		}
	}

	incoming.ID = id
	incoming.CreatedAt = existing.CreatedAt
	incoming.UpdatedAt = time.Now()

	_, err = database.DB.Exec(
		"UPDATE clusters SET name = ?, hosts = ?, auth_type = ?, username = ?, password = ?, api_key = ?, color = ?, notes = ?, updated_at = ? WHERE id = ?",
		incoming.Name, incoming.Hosts, incoming.AuthType,
		incoming.Username, incoming.Password, incoming.APIKey,
		incoming.Color, incoming.Notes, incoming.UpdatedAt, id,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	es.CollectOne(id)

	c.JSON(http.StatusOK, gin.H{"saved": true, "cluster": incoming})
}

// DeleteCluster 删除集群。刻意不校验集群可达性 —— 连不上的集群恰恰是最该删的。
func DeleteCluster(c *gin.Context) {
	id := c.Param("id")

	_, err := database.DB.Exec("DELETE FROM clusters WHERE id = ?", id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	es.Forget(id)

	c.JSON(http.StatusOK, gin.H{"message": "Cluster deleted"})
}

// RefreshClusters 立即重新采集全部集群，用于页面上的手动刷新。
func RefreshClusters(c *gin.Context) {
	es.CollectAll()
	c.JSON(http.StatusOK, gin.H{"message": "Refreshed"})
}

// verifyConnectable 强制执行「先探测、后保存」。
func verifyConnectable(cluster models.Cluster) error {
	if len(cluster.Hosts) == 0 {
		return fmt.Errorf("至少需要配置一个 host")
	}
	if _, err := es.Probe(es.TargetFromCluster(cluster)); err != nil {
		return err
	}
	return nil
}

func connectionChanged(old, new models.Cluster) bool {
	return !sameHosts(old.Hosts, new.Hosts) ||
		old.AuthType != new.AuthType ||
		old.Username != new.Username ||
		old.Password != new.Password ||
		old.APIKey != new.APIKey
}

func sameHosts(a, b models.StringArray) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
