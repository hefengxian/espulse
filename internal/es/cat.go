package es

import (
	"net/url"
	"strings"
	"time"

	"github.com/hefengxian/espulse/internal/models"
)

// 以下类型是 _cat/*?format=json 原始行的映射。
// _cat API 的数值列在 JSON 里同样是字符串（如 "heap.percent":"43"），因此统一用 string，
// 由前端按需转换，避免空值或 "-" 之类占位符导致解析失败。

// Node 是 _cat/nodes 的一行。
type Node struct {
	Name            string `json:"name"`
	Role            string `json:"node.role"`
	Master          string `json:"master"`
	HeapPercent     string `json:"heap.percent"`
	CPU             string `json:"cpu"`
	Load1m          string `json:"load_1m"`
	Load5m          string `json:"load_5m"`
	Load15m         string `json:"load_15m"`
	DiskUsedPercent string `json:"disk.used_percent"`
}

// Index 是 _cat/indices 的一行。
type Index struct {
	Index      string `json:"index"`
	Health     string `json:"health"`
	Status     string `json:"status"`
	Pri        string `json:"pri"`
	Rep        string `json:"rep"`
	DocsCount  string `json:"docs.count"`
	StoreBytes string `json:"store.size"` // 字节数（查询带 bytes=b），便于汇总

	// IndexTotal / SearchTotal 是累计计数器（单位：次），仅用于差分出写入 / 搜索速率。
	IndexTotal  string `json:"indexing.index_total"`
	SearchTotal string `json:"search.query_total"`

	// 以下为采集器用样本环算出的派生速率；样本不足时为 nil，前端显示 "—"（见 PRD §11）。
	IndexRate  *float64 `json:"index_rate,omitempty"`
	SearchRate *float64 `json:"search_rate,omitempty"`
	DocsRate   *float64 `json:"docs_rate,omitempty"`
}

// IndexList 是 KindIndices 的快照载荷：索引行 + 这批速率的实际窗口。
//
// 窗口会随页面打开时长从一个采集间隔逐步长到 rateWindow，因此必须如实上报 ——
// 否则前端会把 5s 窗口的毛刺当成稳定速率（见 PRD §6.3）。
type IndexList struct {
	Rows []Index `json:"rows"`
	// RateWindowMs 是实际参与差分的窗口长度；0 表示本轮尚未算出任何速率。
	RateWindowMs int64 `json:"rate_window_ms"`
}

// Shard 是 _cat/shards 的一行。
type Shard struct {
	Index            string `json:"index"`
	Shard            string `json:"shard"`
	PriRep           string `json:"prirep"`
	State            string `json:"state"`
	Node             string `json:"node"`
	UnassignedReason string `json:"unassigned.reason"`
}

// Alias 是 _cat/aliases 的一行。一个别名指向多个索引时会展开成多行，这是 ES 的原始表达，
// 前端按 (别名, 索引) 逐行展示即可。
// 可选列在无值时是 "-"，因此不做类型转换，由前端判定。
type Alias struct {
	Alias         string `json:"alias"`
	Index         string `json:"index"`
	Filter        string `json:"filter"`
	RoutingIndex  string `json:"routing.index"`
	RoutingSearch string `json:"routing.search"`
	IsWriteIndex  string `json:"is_write_index"`
}

func fetchNodes(cluster models.Cluster) (any, error) {
	query := url.Values{}
	query.Set("format", "json")
	query.Set("h", "name,node.role,master,heap.percent,cpu,load_1m,load_5m,load_15m,disk.used_percent")
	query.Set("s", "name") // 按节点名升序，与索引/分片列表的排序方式保持一致
	return fetchCat[Node](cluster, "/_cat/nodes", query)
}

func fetchIndices(cluster models.Cluster) (any, error) {
	query := url.Values{}
	query.Set("format", "json")
	// indexing.index_total / search.query_total 是 6.x 起就存在的 _cat 列，
	// 用于就地差分出写入 / 搜索速率，无需另调 _stats（见 PRD §11）。
	query.Set("h", "index,health,status,pri,rep,docs.count,store.size,indexing.index_total,search.query_total")
	query.Set("s", "index")
	query.Set("bytes", "b") // store.size 以字节返回，便于直接汇总
	rows, err := fetchCatRows[Index](cluster, "/_cat/indices", query)
	if err != nil {
		return nil, err
	}
	return IndexList{Rows: rows}, nil
}

func fetchShards(cluster models.Cluster) (any, error) {
	query := url.Values{}
	query.Set("format", "json")
	query.Set("h", "index,shard,prirep,state,node,unassigned.reason")
	query.Set("s", "index,shard")
	return fetchCat[Shard](cluster, "/_cat/shards", query)
}

// aliasColumns 是全部别名列；末尾三项（filter / routing / is_write_index）在老版本 ES 上可能不被识别。
var aliasColumns = []string{"alias", "index", "filter", "routing.index", "routing.search", "is_write_index"}

// aliasQuery 按给定列构造 _cat/aliases 查询。
func aliasQuery(columns []string) url.Values {
	query := url.Values{}
	query.Set("format", "json")
	query.Set("h", strings.Join(columns, ","))
	query.Set("s", "alias,index")
	return query
}

// fetchAliases 拉取索引别名列表。
//
// is_write_index 一类列由较新的 ES 版本引入，老版本对未知列会直接返回 400（Unknown column）。
// 为守住 §9 的 6.x / 7.x / 8.x 兼容要求，这里做一次降级重试：先用全量列，被拒则退回基础列。
func fetchAliases(cluster models.Cluster) (any, error) {
	rows, err := fetchCat[Alias](cluster, "/_cat/aliases", aliasQuery(aliasColumns))
	if err == nil {
		return rows, nil
	}

	// 只有「ES 明确拒绝了这次查询」才降级；连不上之类的问题不该被重试掩盖
	if !strings.Contains(err.Error(), "ES 返回 400") {
		return nil, err
	}

	fallback, fallbackErr := fetchCat[Alias](cluster, "/_cat/aliases", aliasQuery(aliasColumns[:3]))
	if fallbackErr != nil {
		return nil, err // 返回原始错误，它对排查更有说明力
	}
	return fallback, nil
}

// fetchCatRows 向 _cat API 取一份 JSON 列表并解析为具体类型。
func fetchCatRows[T any](cluster models.Cluster, path string, query url.Values) ([]T, error) {
	var rows []T
	if err := TargetFromCluster(cluster).getJSON(CatalogClient, path, query, &rows); err != nil {
		return nil, err
	}
	return rows, nil
}

// fetchCat 与 fetchCatRows 相同，但以 any 返回。
// _cat 列表的消费方（缓存、落库、HTTP 层）都以 any 传递，泛型仅用于省掉重复的解析样板。
func fetchCat[T any](cluster models.Cluster, path string, query url.Values) (any, error) {
	return fetchCatRows[T](cluster, path, query)
}

// GetNodes 读取某集群的节点列表，并把该集群标记为活跃（见 PRD §6.3）。
// 返回数据、数据时间、以及是否取到了数据。
func GetNodes(clusterID string) ([]Node, time.Time, bool) {
	data, at, has := collector.Get(clusterID, KindNodes)
	if !has {
		return nil, at, false
	}
	rows, ok := data.([]Node)
	if !ok {
		return nil, at, false
	}
	return rows, at, true
}

// GetIndices 读取某集群的索引列表，并把该集群标记为活跃。
// 返回值里的 Rows 是索引行，RateWindowMs 是这批速率的实际窗口（见 IndexList）。
func GetIndices(clusterID string) (IndexList, time.Time, bool) {
	data, at, has := collector.Get(clusterID, KindIndices)
	if !has {
		return IndexList{}, at, false
	}
	list, ok := data.(IndexList)
	if !ok {
		return IndexList{}, at, false
	}
	return list, at, true
}

// GetShards 读取某集群的分片列表，并把该集群标记为活跃。
func GetShards(clusterID string) ([]Shard, time.Time, bool) {
	data, at, has := collector.Get(clusterID, KindShards)
	if !has {
		return nil, at, false
	}
	rows, ok := data.([]Shard)
	if !ok {
		return nil, at, false
	}
	return rows, at, true
}

// GetAliases 读取某集群的索引别名列表，并把该集群标记为活跃。
func GetAliases(clusterID string) ([]Alias, time.Time, bool) {
	data, at, has := collector.Get(clusterID, KindAliases)
	if !has {
		return nil, at, false
	}
	rows, ok := data.([]Alias)
	if !ok {
		return nil, at, false
	}
	return rows, at, true
}
