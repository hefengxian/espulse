package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hefengxian/espulse/internal/es"
)

// overviewListLimit 是问题清单每类最多返回的条目数，避免大集群把响应撑爆；
// 同时返回总数（*_total），前端据此提示「仅显示前 N 条」。
const overviewListLimit = 20

type overviewHealth struct {
	Status              string  `json:"status"`
	ActiveShardsPercent float64 `json:"active_shards_percent"`
	ActiveShards        int     `json:"active_shards"`
	ActivePrimaryShards int     `json:"active_primary_shards"`
	RelocatingShards    int     `json:"relocating_shards"`
	InitializingShards  int     `json:"initializing_shards"`
	UnassignedShards    int     `json:"unassigned_shards"`
}

type overviewCounts struct {
	Nodes         int   `json:"nodes"`
	Indices       int   `json:"indices"`
	PrimaryShards int   `json:"primary_shards"`
	TotalShards   int   `json:"total_shards"`
	Docs          int64 `json:"docs"`
	StoreBytes    int64 `json:"store_bytes"`
}

type overviewNode struct {
	Name            string `json:"name"`
	Role            string `json:"role"`
	Master          string `json:"master"`
	HeapPercent     string `json:"heap_percent"`
	CPU             string `json:"cpu"`
	Load1m          string `json:"load_1m"`
	Load5m          string `json:"load_5m"`
	Load15m         string `json:"load_15m"`
	DiskUsedPercent string `json:"disk_used_percent"`
	// Shards 是该节点承载的分片数；分片列表不可用时为空（前端据此隐藏该列）。
	Shards *int `json:"shards,omitempty"`
}

type overviewShard struct {
	Index  string `json:"index"`
	Shard  string `json:"shard"`
	PriRep string `json:"prirep"`
	State  string `json:"state"`
	Node   string `json:"node"`
	Reason string `json:"reason,omitempty"`
}

type overviewIndex struct {
	Index  string `json:"index"`
	Health string `json:"health"`
	Status string `json:"status"`
}

type overviewProblems struct {
	UnassignedShards      []overviewShard `json:"unassigned_shards"`
	UnassignedShardsTotal int             `json:"unassigned_shards_total"`
	RelocatingShards      []overviewShard `json:"relocating_shards"`
	RelocatingShardsTotal int             `json:"relocating_shards_total"`
	UnhealthyIndices      []overviewIndex `json:"unhealthy_indices"`
	UnhealthyIndicesTotal int             `json:"unhealthy_indices_total"`
}

type overviewResponse struct {
	Reachable     bool             `json:"reachable"`
	Error         string           `json:"error,omitempty"`
	Health        overviewHealth   `json:"health"`
	Counts        overviewCounts   `json:"counts"`
	Nodes         []overviewNode   `json:"nodes"`
	Problems      overviewProblems `json:"problems"`
	NodesLoaded   bool             `json:"nodes_loaded"`
	IndicesLoaded bool             `json:"indices_loaded"`
	ShardsLoaded  bool             `json:"shards_loaded"`
	UpdatedAt     time.Time        `json:"updated_at"`
}

// GetOverview 聚合集群总览页所需的全部数据（见 PRD §6.1）。
// 数据来自共享采集缓存；首次访问（冷启动）会同步等待一次采集。
func GetOverview(c *gin.Context) {
	clusterID := c.Param("id")

	// refresh=1 表示用户手动刷新：绕过刷新间隔强制重采（仍受单飞去重保护）
	if c.Query("refresh") == "1" {
		es.RefreshCluster(clusterID)
	}

	status, statusAt, ok := es.GetStatus(clusterID)
	if !ok {
		c.JSON(http.StatusOK, overviewResponse{UpdatedAt: statusAt})
		return
	}
	if !status.Reachable {
		// 集群不可达：下面的列表拉取注定失败，直接返回，避免无谓等待
		c.JSON(http.StatusOK, overviewResponse{
			Reachable: false,
			Error:     status.Error,
			UpdatedAt: statusAt,
		})
		return
	}

	var (
		nodes         []es.Node
		indices       []es.Index
		shards        []es.Shard
		nodesLoaded   bool
		indicesLoaded bool
		shardsLoaded  bool
	)

	// 三类列表并行拉取：都走共享缓存，冷启动时最坏耗时约等于最慢的一次
	var wg sync.WaitGroup
	wg.Add(3)
	go func() { defer wg.Done(); nodes, _, nodesLoaded = es.GetNodes(clusterID) }()
	go func() { defer wg.Done(); indices, _, indicesLoaded = es.GetIndices(clusterID) }()
	go func() { defer wg.Done(); shards, _, shardsLoaded = es.GetShards(clusterID) }()
	wg.Wait()

	resp := overviewResponse{
		Reachable:     true,
		NodesLoaded:   nodesLoaded,
		IndicesLoaded: indicesLoaded,
		ShardsLoaded:  shardsLoaded,
		UpdatedAt:     statusAt,
		Health: overviewHealth{
			Status:              status.Health,
			ActiveShardsPercent: status.ActiveShardsPercent,
			ActiveShards:        status.ActiveShards,
			ActivePrimaryShards: status.ActivePrimaryShards,
			RelocatingShards:    status.RelocatingShards,
			InitializingShards:  status.InitializingShards,
			UnassignedShards:    status.UnassignedShards,
		},
		Counts: overviewCounts{
			Nodes:         status.NodeCount,
			Indices:       status.IndexCount,
			PrimaryShards: status.ActivePrimaryShards,
			TotalShards:   status.ActiveShards,
		},
		Problems: overviewProblems{
			UnassignedShards: []overviewShard{},
			RelocatingShards: []overviewShard{},
			UnhealthyIndices: []overviewIndex{},
		},
	}

	// 节点表：顺带把每个节点承载的分片数拼上，失衡与否一眼可见
	shardsByNode := map[string]int{}
	if shardsLoaded {
		for _, s := range shards {
			if s.Node != "" {
				shardsByNode[s.Node]++
			}
		}
	}
	resp.Nodes = make([]overviewNode, 0, len(nodes))
	for _, n := range nodes {
		node := overviewNode{
			Name:            n.Name,
			Role:            n.Role,
			Master:          n.Master,
			HeapPercent:     n.HeapPercent,
			CPU:             n.CPU,
			Load1m:          n.Load1m,
			Load5m:          n.Load5m,
			Load15m:         n.Load15m,
			DiskUsedPercent: n.DiskUsedPercent,
		}
		if shardsLoaded {
			count := shardsByNode[n.Name]
			node.Shards = &count
		}
		resp.Nodes = append(resp.Nodes, node)
	}

	if indicesLoaded {
		for _, idx := range indices {
			if v, err := strconv.ParseInt(idx.DocsCount, 10, 64); err == nil {
				resp.Counts.Docs += v
			}
			if v, err := strconv.ParseInt(idx.StoreBytes, 10, 64); err == nil {
				resp.Counts.StoreBytes += v
			}

			if idx.Health != "" && !strings.EqualFold(idx.Health, "green") {
				resp.Problems.UnhealthyIndicesTotal++
				if len(resp.Problems.UnhealthyIndices) < overviewListLimit {
					resp.Problems.UnhealthyIndices = append(resp.Problems.UnhealthyIndices, overviewIndex{
						Index:  idx.Index,
						Health: idx.Health,
						Status: idx.Status,
					})
				}
			}
		}
	}

	if shardsLoaded {
		for _, s := range shards {
			switch strings.ToUpper(s.State) {
			case "UNASSIGNED":
				resp.Problems.UnassignedShardsTotal++
				if len(resp.Problems.UnassignedShards) < overviewListLimit {
					resp.Problems.UnassignedShards = append(resp.Problems.UnassignedShards, toOverviewShard(s))
				}
			case "RELOCATING":
				resp.Problems.RelocatingShardsTotal++
				if len(resp.Problems.RelocatingShards) < overviewListLimit {
					resp.Problems.RelocatingShards = append(resp.Problems.RelocatingShards, toOverviewShard(s))
				}
			}
		}
	}

	c.JSON(http.StatusOK, resp)
}

func toOverviewShard(s es.Shard) overviewShard {
	return overviewShard{
		Index:  s.Index,
		Shard:  s.Shard,
		PriRep: s.PriRep,
		State:  s.State,
		Node:   s.Node,
		Reason: s.UnassignedReason,
	}
}
