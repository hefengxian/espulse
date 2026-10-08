package es

import (
	"log"
	"sync"
	"time"

	"github.com/hefengxian/espulse/internal/database"
	"github.com/hefengxian/espulse/internal/models"
)

// Status 是一次采集得到的集群快照。
// 它来自 ES，可能过期或采集失败，因此调用方必须同时展示 CollectedAt，
// 不能把它当作实时数据使用。
type Status struct {
	Reachable           bool      `json:"reachable"`
	Error               string    `json:"error,omitempty"`
	Name                string    `json:"name,omitempty"`
	Version             string    `json:"version,omitempty"`
	Health              string    `json:"health,omitempty"`
	NodeCount           int       `json:"node_count"`
	IndexCount          int       `json:"index_count"`
	ActivePrimaryShards int       `json:"active_primary_shards"`
	ActiveShards        int       `json:"active_shards"`
	RelocatingShards    int       `json:"relocating_shards"`
	InitializingShards  int       `json:"initializing_shards"`
	UnassignedShards    int       `json:"unassigned_shards"`
	ActiveShardsPercent float64   `json:"active_shards_percent"`
	CollectedAt         time.Time `json:"collected_at"`
}

// Kind 标识一类被采集的数据。不同数据类型的刷新间隔与开销不同，各自独立缓存与刷新。
type Kind string

const (
	// KindStatus 是集群基本状态：health / 版本 / 节点数 / 索引数 / 未分配分片。
	KindStatus Kind = "status"
	// KindNodes 是节点列表（_cat/nodes）。
	KindNodes Kind = "nodes"
	// KindIndices 是索引列表（_cat/indices）。
	KindIndices Kind = "indices"
	// KindShards 是分片列表（_cat/shards）。
	KindShards Kind = "shards"
)

// kindSpec 描述一类数据的刷新间隔与获取方式。
// 新增数据类型时，在此登记一个 kindSpec 即可复用整套共享采集机制（见 PRD §6.3）。
type kindSpec struct {
	interval time.Duration
	fetch    func(models.Cluster) (any, error)
	// failed 判断「采集本身成功、但结果代表失败」的数据（如 status 的集群不可达）。
	// 这类数据仍会作为最新快照返回，但会触发指数退避重试；nil 表示只依据 err 判定失败。
	failed func(data any) bool
}

var kindSpecs = map[Kind]kindSpec{
	KindStatus:  {interval: 5 * time.Second, fetch: fetchStatus, failed: statusFailed},
	KindNodes:   {interval: 5 * time.Second, fetch: fetchNodes},
	KindIndices: {interval: 5 * time.Second, fetch: fetchIndices},
	KindShards:  {interval: 5 * time.Second, fetch: fetchShards},
}

const (
	// sweepInterval 是调度器的扫描间隔，决定「到点刷新」的精度，与各数据类型的刷新间隔无关。
	sweepInterval = time.Second
	// idleTTL 是缓存被判为「无人查看」的空闲阈值：超过它即停止采集并释放缓存。
	// 必须大于各数据类型的刷新间隔，否则会在淘汰与激活之间抖动。
	idleTTL = 60 * time.Second
	// backoffMax 是失败退避的封顶间隔：连续失败时重试间隔按 2 倍递增直至此值。
	backoffMax = 300 * time.Second
)

// entryKey 是共享缓存的键：每个 (集群, 数据类型) 一份。
type entryKey struct {
	clusterID string
	kind      Kind
}

// entry 是一份共享采集缓存。
// 同一 (集群, 数据类型) 的所有读取者共享它，且同一时刻至多有一次 in-flight 采集。
type entry struct {
	mu          sync.Mutex
	data        any
	hasData     bool
	updatedAt   time.Time // 最近一次采集成功的时间
	lastAttempt time.Time // 最近一次采集尝试的时间（含失败），用于控制重试节奏
	lastAccess  time.Time // 最近一次被读取的时间，即「有人在看」的活跃信号
	refreshing  bool
	done        chan struct{}
	failures    int // 连续失败次数，用于指数退避；任一次成功即归零
}

// Collector 持有全部共享采集缓存。
// 它不做任何定时轮询，采集完全由读取（页面访问）激活（见 PRD §6.3）。
type Collector struct {
	mu      sync.Mutex
	entries map[entryKey]*entry
}

var collector = &Collector{entries: make(map[entryKey]*entry)}

// StartCollector 启动采集调度器。
//
// 与旧实现不同，进程启动时不再无条件采集任何集群：采集由页面访问激活——
// 有人查看时按刷新间隔共享采集，读取者全部离开（超过 idleTTL）后停止采集并释放缓存。
func StartCollector() {
	go func() {
		ticker := time.NewTicker(sweepInterval)
		for range ticker.C {
			collector.sweep(time.Now())
		}
	}()
}

// sweep 是调度器的一次扫描：淘汰无人查看的缓存，并按间隔刷新仍活跃的缓存。
func (c *Collector) sweep(now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for key, e := range c.entries {
		if e.idleFor(now) > idleTTL {
			// 无人查看：停止采集并释放内存
			delete(c.entries, key)
			continue
		}

		spec, ok := kindSpecs[key.kind]
		if !ok {
			continue
		}
		if e.shouldRefresh(spec, now) {
			startRefresh(e, key, spec)
		}
	}
}

// Get 读取某集群某类数据的共享缓存，同时把该缓存标记为活跃。
// 语义（见 PRD §6.3）：
//   - 有新鲜数据：直接返回，不触发 ES 请求；
//   - 有过期数据：立即返回旧数据，同时后台刷新（stale-while-revalidate）；
//   - 无数据：同步等待一次采集后返回（冷启动）。
//
// 返回值为数据、数据时间、以及是否取到了数据。
func (c *Collector) Get(clusterID string, kind Kind) (any, time.Time, bool) {
	spec, ok := kindSpecs[kind]
	if !ok {
		return nil, time.Time{}, false
	}

	key := entryKey{clusterID: clusterID, kind: kind}
	e := c.getOrCreate(key)
	now := time.Now()
	e.touch(now)

	if data, at, has := e.read(); has {
		if e.shouldRefresh(spec, now) {
			startRefresh(e, key, spec)
		}
		return data, at, true
	}

	// 冷启动：没有旧数据可返回，只能等这一次采集完成。
	// 已有 in-flight 采集时等它（单飞去重）；仍在失败退避窗口内则直接返回「暂无数据」，
	// 避免页面轮询把退避绕过去、对挂掉的集群持续打 ES。
	if e.refreshingNow() || e.shouldRefresh(spec, now) {
		<-startRefresh(e, key, spec)
	}
	return e.read()
}

// Activate 把某集群某类数据标记为活跃，并在需要时触发后台刷新，但不等待结果。
// 用于一次涉及多个集群、不适合逐个同步等待的场景。
func (c *Collector) Activate(clusterID string, kind Kind) {
	spec, ok := kindSpecs[kind]
	if !ok {
		return
	}

	key := entryKey{clusterID: clusterID, kind: kind}
	e := c.getOrCreate(key)
	now := time.Now()
	e.touch(now)

	if e.shouldRefresh(spec, now) {
		startRefresh(e, key, spec)
	}
}

// RefreshNow 强制立即刷新（忽略刷新间隔）并阻塞至采集完成，用于手动刷新。
// 若已有 in-flight 采集，则等待该次结果，不会重复请求 ES。
func (c *Collector) RefreshNow(clusterID string, kind Kind) {
	spec, ok := kindSpecs[kind]
	if !ok {
		return
	}

	key := entryKey{clusterID: clusterID, kind: kind}
	e := c.getOrCreate(key)
	e.touch(time.Now())
	<-startRefresh(e, key, spec)
}

// StatusSnapshot 读取一批集群的状态缓存，并把它们标记为活跃。
// 对尚无缓存的集群会同步等待一次采集，因此最坏耗时约等于一次采集的超时。
func StatusSnapshot(clusterIDs []string) map[string]Status {
	out := make(map[string]Status, len(clusterIDs))

	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, id := range clusterIDs {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			data, _, has := collector.Get(id, KindStatus)
			if !has {
				return
			}
			if status, ok := data.(Status); ok {
				mu.Lock()
				out[id] = status
				mu.Unlock()
			}
		}(id)
	}
	wg.Wait()

	return out
}

// Forget 丢弃某集群的全部缓存，避免已删除的集群残留在内存里。
func Forget(clusterID string) {
	collector.mu.Lock()
	defer collector.mu.Unlock()
	for key := range collector.entries {
		if key.clusterID == clusterID {
			delete(collector.entries, key)
		}
	}
}

// CollectAll 立即刷新全部集群的状态，用于页面上的手动刷新。
func CollectAll() {
	clusters, err := loadClusterTargets()
	if err != nil {
		log.Printf("刷新集群失败，读取集群列表出错: %v", err)
		return
	}

	var wg sync.WaitGroup
	for _, cluster := range clusters {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			collector.RefreshNow(id, KindStatus)
		}(cluster.ID)
	}
	wg.Wait()
}

// CollectOne 立即刷新单个集群的状态，用于新增 / 编辑后刷新卡片状态，避免等待下个周期。
func CollectOne(clusterID string) {
	collector.RefreshNow(clusterID, KindStatus)
}

// RefreshKind 强制刷新某集群的某一类数据（忽略刷新间隔），用于页面的手动刷新。
func RefreshKind(clusterID string, kind Kind) {
	collector.RefreshNow(clusterID, kind)
}

// RefreshCluster 强制刷新某集群的全部数据类型，用于页面的手动刷新。
// 各组数据并行刷新，且各自仍受单飞去重保护。
func RefreshCluster(clusterID string) {
	kinds := make([]Kind, 0, len(kindSpecs))
	for kind := range kindSpecs {
		kinds = append(kinds, kind)
	}

	var wg sync.WaitGroup
	for _, kind := range kinds {
		wg.Add(1)
		go func(kind Kind) {
			defer wg.Done()
			collector.RefreshNow(clusterID, kind)
		}(kind)
	}
	wg.Wait()
}

func (c *Collector) getOrCreate(key entryKey) *entry {
	c.mu.Lock()
	defer c.mu.Unlock()
	if e, ok := c.entries[key]; ok {
		return e
	}
	e := &entry{}
	c.entries[key] = e
	return e
}

func (e *entry) touch(now time.Time) {
	e.mu.Lock()
	e.lastAccess = now
	e.mu.Unlock()
}

func (e *entry) read() (any, time.Time, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.data, e.updatedAt, e.hasData
}

func (e *entry) idleFor(now time.Time) time.Duration {
	e.mu.Lock()
	defer e.mu.Unlock()
	return now.Sub(e.lastAccess)
}

// shouldRefresh 判断是否到了该刷新的时候。
// 以 lastAttempt 为基准，保证采集失败时也按间隔重试，而不会每个扫描周期都打一次 ES；
// 连续失败时间隔按 2 倍指数退避，封顶 backoffMax。
func (e *entry) shouldRefresh(spec kindSpec, now time.Time) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.refreshing {
		return false
	}
	return now.Sub(e.lastAttempt) >= backoffInterval(spec.interval, e.failures)
}

// refreshingNow 报告当前是否有 in-flight 采集。
func (e *entry) refreshingNow() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.refreshing
}

// backoffInterval 按连续失败次数计算生效的刷新间隔：
// 0 次失败即基准间隔，之后每次翻倍，直至封顶 backoffMax。
func backoffInterval(base time.Duration, failures int) time.Duration {
	d := base
	for range failures {
		d *= 2
		if d >= backoffMax {
			return backoffMax
		}
	}
	return d
}

// startRefresh 启动一次采集并返回其完成信号。
// 若已有 in-flight 采集，则直接返回该次采集的信号——并发请求只会触发一次真实的 ES 请求（single-flight）。
func startRefresh(e *entry, key entryKey, spec kindSpec) <-chan struct{} {
	e.mu.Lock()
	if e.refreshing {
		ch := e.done
		e.mu.Unlock()
		return ch
	}
	e.refreshing = true
	e.done = make(chan struct{})
	ch := e.done
	e.mu.Unlock()

	go func() {
		data, err := loadAndFetch(key.clusterID, spec)

		e.mu.Lock()
		e.lastAttempt = time.Now()
		switch {
		case err != nil:
			// 保留上一次的成功数据，仅记录失败；进入指数退避，下个间隔再重试
			e.failures++
			log.Printf("采集失败 (cluster=%s, kind=%s): %v", key.clusterID, key.kind, err)
		case spec.failed != nil && spec.failed(data):
			// 数据本身代表失败（如集群不可达）：仍作为最新快照返回，但同样进入退避
			e.data = data
			e.hasData = true
			e.updatedAt = e.lastAttempt
			e.failures++
			log.Printf("采集异常 (cluster=%s, kind=%s): 数据标记为失败，进入退避", key.clusterID, key.kind)
		default:
			e.data = data
			e.hasData = true
			e.updatedAt = e.lastAttempt
			e.failures = 0
		}
		e.refreshing = false
		e.done = nil
		e.mu.Unlock()

		close(ch)
	}()

	return ch
}

func loadAndFetch(clusterID string, spec kindSpec) (any, error) {
	cluster, err := loadCluster(clusterID)
	if err != nil {
		return nil, err
	}
	return spec.fetch(cluster)
}

// fetchStatus 采集集群状态。
// 注意：连不上的集群不是错误，而是 reachable=false 的一种数据，因此这里始终返回 nil error，
// 保证快照里保留「不可达」这一状态，而不是被当作采集失败丢掉。
func fetchStatus(cluster models.Cluster) (any, error) {
	return collect(cluster), nil
}

// statusFailed 把「集群不可达」视为失败：数据仍会返回并展示，但触发指数退避重试。
func statusFailed(data any) bool {
	status, ok := data.(Status)
	return ok && !status.Reachable
}

func collect(cluster models.Cluster) Status {
	collectedAt := time.Now()
	target := TargetFromCluster(cluster)

	info, err := Probe(target)
	if err != nil {
		return Status{Reachable: false, Error: err.Error(), CollectedAt: collectedAt}
	}

	// 索引数采集失败不影响"集群可用"这个判定
	indexCount, err := IndexCount(target)
	if err != nil {
		log.Printf("采集索引数失败 (%s): %v", cluster.Name, err)
	}

	return Status{
		Reachable:           true,
		Name:                info.Name,
		Version:             info.Version,
		Health:              info.Health,
		NodeCount:           info.NodeCount,
		IndexCount:          indexCount,
		ActivePrimaryShards: info.ActivePrimaryShards,
		ActiveShards:        info.ActiveShards,
		RelocatingShards:    info.RelocatingShards,
		InitializingShards:  info.InitializingShards,
		UnassignedShards:    info.UnassignedShards,
		ActiveShardsPercent: info.ActiveShardsPercent,
		CollectedAt:         collectedAt,
	}
}

// GetStatus 读取某集群的状态快照，并把该集群标记为活跃（见 PRD §6.3）。
func GetStatus(clusterID string) (Status, time.Time, bool) {
	data, at, has := collector.Get(clusterID, KindStatus)
	if !has {
		return Status{}, at, false
	}
	status, ok := data.(Status)
	if !ok {
		return Status{}, at, false
	}
	return status, at, true
}

// loadCluster 从数据库读取单个集群的连接信息，供采集使用。
func loadCluster(id string) (models.Cluster, error) {
	var cluster models.Cluster
	err := database.DB.QueryRow(
		"SELECT id, name, hosts, auth_type, username, password, api_key, color, notes FROM clusters WHERE id = ?",
		id,
	).Scan(
		&cluster.ID, &cluster.Name, &cluster.Hosts, &cluster.AuthType,
		&cluster.Username, &cluster.Password, &cluster.APIKey,
		&cluster.Color, &cluster.Notes,
	)
	return cluster, err
}

func loadClusterTargets() ([]models.Cluster, error) {
	rows, err := database.DB.Query(
		"SELECT id, name, hosts, auth_type, username, password, api_key, color, notes FROM clusters",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	clusters := []models.Cluster{}
	for rows.Next() {
		var cluster models.Cluster
		if err := rows.Scan(
			&cluster.ID, &cluster.Name, &cluster.Hosts, &cluster.AuthType,
			&cluster.Username, &cluster.Password, &cluster.APIKey,
			&cluster.Color, &cluster.Notes,
		); err != nil {
			return nil, err
		}
		clusters = append(clusters, cluster)
	}
	return clusters, rows.Err()
}
