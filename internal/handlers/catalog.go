package handlers

import (
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hefengxian/espulse/internal/es"
)

// catalogResponse 是节点 / 分片两类列表的统一返回结构。
// 一律带上 updated_at，前端需据此展示数据新鲜度，不得把缓存当实时（见 PRD §6.3）。
type catalogResponse struct {
	Data      any       `json:"data"`
	UpdatedAt time.Time `json:"updated_at"`
}

// indicesResponse 是索引列表的返回结构：索引数量无上界，因此必须分页（见 PRD §10）。
type indicesResponse struct {
	Data      []es.Index `json:"data"`
	Total     int        `json:"total"`
	Page      int        `json:"page"`
	PageSize  int        `json:"page_size"`
	UpdatedAt time.Time  `json:"updated_at"`
}

const (
	defaultPageSize = 20
	maxPageSize     = 200
)

// ListNodes 返回某集群的节点列表。
func ListNodes(c *gin.Context) {
	id := c.Param("id")
	if refreshRequested(c) {
		es.RefreshKind(id, es.KindNodes)
	}
	rows, updatedAt, ok := es.GetNodes(id)
	respondCatalog(c, rows, updatedAt, ok)
}

// ListIndices 返回某集群的索引列表（支持 search / health / status 过滤与分页）。
// 过滤与分页都基于采集缓存，不额外请求 ES —— 万级索引下只是内存切片操作。
func ListIndices(c *gin.Context) {
	id := c.Param("id")
	if refreshRequested(c) {
		es.RefreshKind(id, es.KindIndices)
	}

	rows, updatedAt, ok := es.GetIndices(id)
	if !ok {
		c.JSON(http.StatusOK, indicesResponse{Data: []es.Index{}, UpdatedAt: updatedAt})
		return
	}
	if rows == nil {
		rows = []es.Index{}
	}

	rows = filterIndices(rows, c.Query("search"), c.Query("health"), c.Query("status"))
	rows = sortIndices(rows, c.Query("sort"), c.Query("order"))

	page, pageSize := pagination(c)
	total := len(rows)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}

	c.JSON(http.StatusOK, indicesResponse{
		Data:      rows[start:end],
		Total:     total,
		Page:      page,
		PageSize:  pageSize,
		UpdatedAt: updatedAt,
	})
}

// ListShards 返回某集群的分片列表。
func ListShards(c *gin.Context) {
	id := c.Param("id")
	if refreshRequested(c) {
		es.RefreshKind(id, es.KindShards)
	}
	rows, updatedAt, ok := es.GetShards(id)
	respondCatalog(c, rows, updatedAt, ok)
}

// ListAliases 返回某集群的索引别名列表。
// 别名数量有上界（远小于索引数），因此不分页，一次返回全量。
func ListAliases(c *gin.Context) {
	id := c.Param("id")
	if refreshRequested(c) {
		es.RefreshKind(id, es.KindAliases)
	}
	rows, updatedAt, ok := es.GetAliases(id)
	respondCatalog(c, rows, updatedAt, ok)
}

// filterIndices 按名称子串 / health / status 过滤，空参数表示不过滤。
func filterIndices(rows []es.Index, search, health, status string) []es.Index {
	search = strings.ToLower(strings.TrimSpace(search))
	health = strings.TrimSpace(health)
	status = strings.TrimSpace(status)
	if search == "" && health == "" && status == "" {
		return rows
	}

	out := make([]es.Index, 0, len(rows))
	for _, idx := range rows {
		if search != "" && !strings.Contains(strings.ToLower(idx.Index), search) {
			continue
		}
		if health != "" && !strings.EqualFold(idx.Health, health) {
			continue
		}
		if status != "" && !strings.EqualFold(idx.Status, status) {
			continue
		}
		out = append(out, idx)
	}
	return out
}

// sortIndices 按 sort / order 排序。
// 默认（空 sort，或 index 升序）直接返回：缓存来自 `_cat/indices?s=index`，本身已是索引名升序。
// 需要重排时先复制一份，避免就地修改被并发读取的共享采集缓存。
func sortIndices(rows []es.Index, sortKey, order string) []es.Index {
	key := strings.ToLower(strings.TrimSpace(sortKey))
	desc := strings.EqualFold(strings.TrimSpace(order), "desc")

	if (key == "" || key == "index") && !desc {
		return rows
	}

	out := make([]es.Index, len(rows))
	copy(out, rows)

	less := func(i, j int) bool { return out[i].Index < out[j].Index }
	switch key {
	case "store":
		less = func(i, j int) bool { return toInt64(out[i].StoreBytes) < toInt64(out[j].StoreBytes) }
	case "docs":
		less = func(i, j int) bool { return toInt64(out[i].DocsCount) < toInt64(out[j].DocsCount) }
	case "health":
		// 升序 = 严重在前（red → yellow → green）
		less = func(i, j int) bool { return healthRank(out[i].Health) < healthRank(out[j].Health) }
	}

	if desc {
		asc := less
		less = func(i, j int) bool { return asc(j, i) }
	}

	sort.SliceStable(out, less)
	return out
}

func toInt64(value string) int64 {
	n, _ := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	return n
}

// healthRank 让 red 最小，升序时把有问题的索引排在最前。
func healthRank(health string) int {
	switch strings.ToLower(strings.TrimSpace(health)) {
	case "red":
		return 0
	case "yellow":
		return 1
	case "green":
		return 2
	default:
		return 3
	}
}

// pagination 解析 page / page_size，并对越界值做收敛。
func pagination(c *gin.Context) (page, pageSize int) {
	page, _ = strconv.Atoi(c.Query("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ = strconv.Atoi(c.Query("page_size"))
	if pageSize < 1 {
		pageSize = defaultPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}
	return page, pageSize
}

// refreshRequested 表示用户手动刷新：绕过刷新间隔强制重采（仍受单飞去重保护）。
func refreshRequested(c *gin.Context) bool {
	return c.Query("refresh") == "1"
}

// respondCatalog 尚未取到数据时只返回 200 + updated_at（零值），由前端展示「尚未采集」。
// 采集失败不是请求错误 —— 失败原因通过集群状态（status）呈现。
func respondCatalog(c *gin.Context, rows any, updatedAt time.Time, ok bool) {
	if !ok {
		c.JSON(http.StatusOK, catalogResponse{UpdatedAt: updatedAt})
		return
	}
	c.JSON(http.StatusOK, catalogResponse{Data: rows, UpdatedAt: updatedAt})
}
