package es

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
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

// NewRequest 构造一个指向目标集群首个 host 的请求，认证信息会自动写入。
// path 需以 / 开头；query 可为 nil。
//
// 需要在多个 host 间做故障转移时请改用 Do。
func (t Target) NewRequest(method, path string, query url.Values, body io.Reader) (*http.Request, error) {
	hosts, err := t.baseURLs()
	if err != nil {
		return nil, err
	}
	return t.newRequest(hosts[0], method, path, query, body)
}

// Request 描述一次 ES 请求中与 host 无关的部分。
// 刻意不含 host：同一个 Request 会在多个 host 上依次尝试（见 Do）。
type Request struct {
	Method string
	Path   string
	Query  url.Values
	// Body 每尝试一个 host 就被调用一次，必须返回全新的读取器（同一个 body 不能被读两次）。
	// nil 表示无请求体。
	Body func() (io.Reader, error)
	// NoRetry 表示该请求无法在其他 host 上重放（如超大流式请求体），只尝试首个 host。
	NoRetry bool
}

// Do 依次尝试集群的所有 host，返回第一个成功的响应（见 PRD §10「多 host 故障转移」）。
//
// 只有传输层错误（连不上、超时）才会换下一个 host；ES 正常返回的 HTTP 错误码原样返回、不重试 ——
// 4xx/5xx 是集群对请求的正式答复，换个体再问一次既无意义，也可能把问题放大。
func (t Target) Do(client *http.Client, req Request, prepare func(*http.Request)) (*http.Response, error) {
	hosts, err := t.baseURLs()
	if err != nil {
		return nil, err
	}

	order := t.hostOrder(hosts)
	if req.NoRetry {
		order = order[:1]
	}

	var firstErr error
	for _, host := range order {
		var body io.Reader
		if req.Body != nil {
			body, err = req.Body()
			if err != nil {
				return nil, err
			}
		}

		httpReq, err := t.newRequest(host, req.Method, req.Path, req.Query, body)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if prepare != nil {
			prepare(httpReq)
		}

		resp, err := client.Do(httpReq)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}

		// 记住这个可用节点，后续请求优先命中它（见 hostOrder）
		t.rememberHost(host)
		return resp, nil
	}

	if firstErr == nil {
		firstErr = fmt.Errorf("集群未配置可用 hosts")
	}
	if len(order) > 1 {
		return nil, fmt.Errorf("集群的 %d 个 host 均不可达: %w", len(order), firstErr)
	}
	return nil, firstErr
}

// newRequest 针对指定 host 构造请求。
func (t Target) newRequest(host, method, path string, query url.Values, body io.Reader) (*http.Request, error) {
	u, err := url.Parse(host)
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

// baseURLs 返回所有 host 的规范化地址（补全 scheme）。
// 空白与无法解析的地址会被跳过，这样配置里混入坏值时仍能连上其余节点。
func (t Target) baseURLs() ([]string, error) {
	if len(t.Hosts) == 0 {
		return nil, fmt.Errorf("集群未配置 hosts")
	}

	hosts := make([]string, 0, len(t.Hosts))
	for _, host := range t.Hosts {
		host = strings.TrimSpace(host)
		if host == "" {
			continue
		}
		if !strings.HasPrefix(host, "http://") && !strings.HasPrefix(host, "https://") {
			host = "http://" + host
		}
		u, err := url.Parse(host)
		if err != nil {
			continue
		}
		hosts = append(hosts, u.String())
	}

	if len(hosts) == 0 {
		return nil, fmt.Errorf("集群配置的 hosts 均非法")
	}
	return hosts, nil
}

// hostOrder 决定尝试顺序：上次成功的 host 排在最前，其余按配置顺序补齐。
//
// 这样某个节点失联后，后续请求不必每次都先撞一次它（黑洞地址会白等一整个超时）；
// 同时也不主动切回曾经的失败节点，避免在抖动的节点之间来回横跳。
func (t Target) hostOrder(hosts []string) []string {
	preferred := preferredHost(t.Hosts)
	if preferred == "" {
		return hosts
	}

	order := make([]string, 0, len(hosts))
	for _, host := range hosts {
		if host == preferred {
			order = append(order, host)
			break
		}
	}
	for _, host := range hosts {
		if host != preferred {
			order = append(order, host)
		}
	}
	return order
}

// hostPreference 记录每个集群最近一次成功的 host，供后续请求优先命中。
// 键是 hosts 配置的签名：集群地址被编辑后签名随之变化，旧记录自然失效。
// 键的数量与「配置过的地址组合」同阶，实践中很小，因此不做淘汰。
var hostPreference = struct {
	sync.Mutex
	last map[string]string
}{last: make(map[string]string)}

func hostSignature(hosts []string) string {
	return strings.Join(hosts, ",")
}

func preferredHost(hosts []string) string {
	hostPreference.Lock()
	defer hostPreference.Unlock()
	return hostPreference.last[hostSignature(hosts)]
}

func (t Target) rememberHost(host string) {
	hostPreference.Lock()
	defer hostPreference.Unlock()
	hostPreference.last[hostSignature(t.Hosts)] = host
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
	resp, err := t.Do(client, Request{Method: http.MethodGet, Path: path, Query: query}, nil)
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
