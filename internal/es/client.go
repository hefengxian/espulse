package es

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/hefengxian/espulse/internal/models"
)

// Target 描述一次 ES 请求所需的连接信息。
// 刻意不依赖 models.Cluster，这样尚未落库的集群也能被探测。
type Target struct {
	Hosts    []string
	AuthType string
	Username string
	Password string
	APIKey   string
}

// TargetFromCluster 将数据库中的集群转换为请求目标。
func TargetFromCluster(c models.Cluster) Target {
	return Target{
		Hosts:    c.Hosts,
		AuthType: c.AuthType,
		Username: c.Username,
		Password: c.Password,
		APIKey:   c.APIKey,
	}
}

// SharedClient 用于代理转发：不设超时，避免打断 reindex、长查询之类的请求。
var SharedClient = &http.Client{
	Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // 允许 ES 自签名证书
	},
}

// ProbeClient 用于探测与采集：必须有超时，否则一个失联集群会拖垮整个采集流程。
var ProbeClient = &http.Client{
	Timeout: 5 * time.Second,
	Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	},
}

// CatalogClient 用于节点 / 索引 / 分片等 _cat 列表拉取。
// 超时比探测宽松：万级索引下 _cat/indices 的耗时可能远超 5s（见 PRD §10）。
var CatalogClient = &http.Client{
	Timeout: 30 * time.Second,
	Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	},
}

// NewRequest 构造一个指向目标集群的请求，认证信息会自动写入。
// path 需以 / 开头；query 可为 nil。
func (t Target) NewRequest(method, path string, query url.Values, body io.Reader) (*http.Request, error) {
	base, err := t.baseURL()
	if err != nil {
		return nil, err
	}
	u, err := url.Parse(base)
	if err != nil {
		return nil, fmt.Errorf("无法解析集群地址: %w", err)
	}
	u.Path, err = url.JoinPath(u.Path, path)
	if err != nil {
		return nil, fmt.Errorf("拼接请求路径失败: %w", err)
	}
	if query != nil {
		u.RawQuery = query.Encode()
	}

	req, err := http.NewRequest(method, u.String(), body)
	if err != nil {
		return nil, err
	}
	t.applyAuth(req)
	return req, nil
}

// baseURL 返回带 scheme 的基地址。
// 当前只取第一个 host —— 多 host 故障转移是已知缺口（见 PRD §10）。
func (t Target) baseURL() (string, error) {
	if len(t.Hosts) == 0 {
		return "", fmt.Errorf("集群未配置 hosts")
	}
	host := t.Hosts[0]
	if !strings.HasPrefix(host, "http://") && !strings.HasPrefix(host, "https://") {
		host = "http://" + host
	}
	u, err := url.Parse(host)
	if err != nil {
		return "", fmt.Errorf("集群地址非法: %w", err)
	}
	return u.String(), nil
}

func (t Target) applyAuth(req *http.Request) {
	switch t.AuthType {
	case "basic":
		req.SetBasicAuth(t.Username, t.Password)
	case "api_key":
		req.Header.Set("Authorization", "ApiKey "+t.APIKey)
	}
}

// Info 是探测得到的集群基本信息。
type Info struct {
	Name                string  `json:"name"`
	Version             string  `json:"version"`
	Health              string  `json:"health"`
	NodeCount           int     `json:"node_count"`
	ActivePrimaryShards int     `json:"active_primary_shards"`
	ActiveShards        int     `json:"active_shards"`
	RelocatingShards    int     `json:"relocating_shards"`
	InitializingShards  int     `json:"initializing_shards"`
	UnassignedShards    int     `json:"unassigned_shards"`
	ActiveShardsPercent float64 `json:"active_shards_percent"`
}

type rootResponse struct {
	ClusterName string `json:"cluster_name"`
	Version     struct {
		Number string `json:"number"`
	} `json:"version"`
}

type healthResponse struct {
	Status                      string  `json:"status"`
	NumberOfNodes               int     `json:"number_of_nodes"`
	ActivePrimaryShards         int     `json:"active_primary_shards"`
	ActiveShards                int     `json:"active_shards"`
	RelocatingShards            int     `json:"relocating_shards"`
	InitializingShards          int     `json:"initializing_shards"`
	UnassignedShards            int     `json:"unassigned_shards"`
	ActiveShardsPercentAsNumber float64 `json:"active_shards_percent_as_number"`
}

// Probe 探测集群连通性：GET / 取名称与版本，GET /_cluster/health 取健康状态。
// 任意一步失败都会返回错误，调用方据此判定集群不可达。
func Probe(t Target) (*Info, error) {
	var root rootResponse
	if err := t.getJSON(ProbeClient, "/", nil, &root); err != nil {
		return nil, fmt.Errorf("无法连接集群: %w", err)
	}

	var health healthResponse
	if err := t.getJSON(ProbeClient, "/_cluster/health", nil, &health); err != nil {
		return nil, fmt.Errorf("读取集群健康状态失败: %w", err)
	}

	return &Info{
		Name:                root.ClusterName,
		Version:             root.Version.Number,
		Health:              health.Status,
		NodeCount:           health.NumberOfNodes,
		ActivePrimaryShards: health.ActivePrimaryShards,
		ActiveShards:        health.ActiveShards,
		RelocatingShards:    health.RelocatingShards,
		InitializingShards:  health.InitializingShards,
		UnassignedShards:    health.UnassignedShards,
		ActiveShardsPercent: health.ActiveShardsPercentAsNumber,
	}, nil
}

// IndexCount 统计索引数量。
// 为了控制开销只取 index 一列，但万级索引下响应体仍然不小，是已知的性能待评估项（见 PRD §10）。
func IndexCount(t Target) (int, error) {
	query := url.Values{}
	query.Set("format", "json")
	query.Set("h", "index")

	var rows []struct {
		Index string `json:"index"`
	}
	if err := t.getJSON(ProbeClient, "/_cat/indices", query, &rows); err != nil {
		return 0, err
	}
	return len(rows), nil
}

func (t Target) getJSON(client *http.Client, path string, query url.Values, out any) error {
	req, err := t.NewRequest(http.MethodGet, path, query, nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		msg := strings.TrimSpace(string(body))
		if msg == "" {
			msg = http.StatusText(resp.StatusCode)
		}
		return fmt.Errorf("ES 返回 %d: %s", resp.StatusCode, msg)
	}

	return json.NewDecoder(resp.Body).Decode(out)
}
