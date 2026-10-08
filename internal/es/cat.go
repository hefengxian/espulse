package es

import (
	"net/url"
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
	query.Set("h", "index,health,status,pri,rep,docs.count,store.size")
	query.Set("s", "index")
	query.Set("bytes", "b") // store.size 以字节返回，便于直接汇总
	return fetchCat[Index](cluster, "/_cat/indices", query)
}

func fetchShards(cluster models.Cluster) (any, error) {
	query := url.Values{}
	query.Set("format", "json")
	query.Set("h", "index,shard,prirep,state,node,unassigned.reason")
	query.Set("s", "index,shard")
	return fetchCat[Shard](cluster, "/_cat/shards", query)
}

// fetchCat 向 _cat API 取一份 JSON 列表。泛型仅用于省掉三次重复的解析样板。
func fetchCat[T any](cluster models.Cluster, path string, query url.Values) (any, error) {
	var rows []T
	if err := TargetFromCluster(cluster).getJSON(CatalogClient, path, query, &rows); err != nil {
		return nil, err
	}
	return rows, nil
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
func GetIndices(clusterID string) ([]Index, time.Time, bool) {
	data, at, has := collector.Get(clusterID, KindIndices)
	if !has {
		return nil, at, false
	}
	rows, ok := data.([]Index)
	if !ok {
		return nil, at, false
	}
	return rows, at, true
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
