package es

import (
	"encoding/json"
	"strconv"
	"testing"
	"time"

	"github.com/hefengxian/espulse/internal/database"
)

// newTestDB 为每个用例准备一个独立的临时库，并重置进程级的共享缓存。
func newTestDB(t *testing.T) {
	t.Helper()

	if err := database.InitDB(t.TempDir()); err != nil {
		t.Fatalf("初始化临时库失败: %v", err)
	}
	t.Cleanup(func() { _ = database.DB.Close() })

	collector = &Collector{
		entries: make(map[entryKey]*entry),
		rings:   make(map[string]*indexRing),
	}
}

func indexRows(indexTotal, searchTotal, docsCount string) []Index {
	return []Index{{
		Index:       "logs-1",
		Health:      "green",
		Status:      "open",
		DocsCount:   docsCount,
		IndexTotal:  indexTotal,
		SearchTotal: searchTotal,
	}}
}

func TestObserveComputesRatesFromWindow(t *testing.T) {
	newTestDB(t)
	base := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	ring := collector.ringFor("c1")

	ring.observe("c1", indexRows("1000", "200", "50"), base)
	rows, window := ring.observe("c1", indexRows("1600", "260", "80"), base.Add(10*time.Second))

	if window != 10*time.Second {
		t.Fatalf("窗口应为 10s，实际 %v", window)
	}
	// (1600-1000)/10 = 60 ops/s，(260-200)/10 = 6 qps，(80-50)/10 = 3 docs/s
	assertRate(t, "index_rate", rows[0].IndexRate, 60)
	assertRate(t, "search_rate", rows[0].SearchRate, 6)
	assertRate(t, "docs_rate", rows[0].DocsRate, 3)
}

func TestObserveWithoutEnoughSamplesLeavesRatesEmpty(t *testing.T) {
	newTestDB(t)
	base := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	ring := collector.ringFor("c1")

	rows, window := ring.observe("c1", indexRows("1000", "200", "50"), base)

	if window != 0 {
		t.Fatalf("只有一个样本时不应报出窗口，实际 %v", window)
	}
	if rows[0].IndexRate != nil || rows[0].SearchRate != nil || rows[0].DocsRate != nil {
		t.Fatalf("样本不足时速率必须留空，实际 %v / %v / %v", rows[0].IndexRate, rows[0].SearchRate, rows[0].DocsRate)
	}
}

func TestObserveDropsStaleSamplesAfterCounterRollback(t *testing.T) {
	newTestDB(t)
	base := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	ring := collector.ringFor("c1")

	ring.observe("c1", indexRows("1000", "200", "50"), base)

	// 索引被删除重建：累计计数归零，此前样本不可比
	rows, window := ring.observe("c1", indexRows("20", "4", "1"), base.Add(5*time.Second))
	if window != 0 || rows[0].IndexRate != nil {
		t.Fatalf("计数倒退后应重新积累样本，实际 window=%v rate=%v", window, rows[0].IndexRate)
	}

	// 新基线建立后应能正常出数，且不会把归零前后的差值算进去
	rows, _ = ring.observe("c1", indexRows("70", "14", "4"), base.Add(10*time.Second))
	assertRate(t, "index_rate", rows[0].IndexRate, 10)
	if samples := ring.samples["logs-1"]; len(samples) != 2 {
		t.Fatalf("计数倒退应丢弃旧样本，实际保留 %d 个", len(samples))
	}
}

func TestObserveTrimsSamplesOutsideWindow(t *testing.T) {
	newTestDB(t)
	base := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	ring := collector.ringFor("c1")

	ring.observe("c1", indexRows("1000", "200", "50"), base)
	ring.observe("c1", indexRows("1500", "300", "60"), base.Add(30*time.Second))

	// 距首个样本已 120s，窗口（60s）内的样本只剩本轮这一个
	rows, window := ring.observe("c1", indexRows("2000", "400", "70"), base.Add(120*time.Second))
	if window != 0 || rows[0].IndexRate != nil {
		t.Fatalf("窗口内只剩一个样本时不应出数，实际 window=%v rate=%v", window, rows[0].IndexRate)
	}
	if samples := ring.samples["logs-1"]; len(samples) != 1 {
		t.Fatalf("窗口外的样本应被裁剪，实际保留 %d 个", len(samples))
	}
}

// 落库的样本环让进程重启后仍然是「温」的，这是方案 B 相对纯内存环的核心收益。
func TestIndexRingSurvivesProcessRestart(t *testing.T) {
	newTestDB(t)
	base := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)

	collector.ringFor("c1").observe("c1", indexRows("1000", "200", "50"), base)

	// 模拟进程重启：内存环全部丢弃（DB 里的样本保留）
	collector.rings = make(map[string]*indexRing)

	rows, window := collector.ringFor("c1").observe("c1", indexRows("1600", "260", "80"), base.Add(10*time.Second))
	if window != 10*time.Second {
		t.Fatalf("重启后应能用落库样本继续差分，实际 window=%v", window)
	}
	assertRate(t, "index_rate", rows[0].IndexRate, 60)
}

func TestObservePrunesSamplesOfDeletedIndices(t *testing.T) {
	newTestDB(t)
	base := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	ring := collector.ringFor("c1")

	ring.observe("c1", []Index{
		{Index: "logs-1", IndexTotal: "10", SearchTotal: "1", DocsCount: "1"},
		{Index: "logs-2", IndexTotal: "20", SearchTotal: "2", DocsCount: "2"},
	}, base)

	ring.observe("c1", []Index{
		{Index: "logs-1", IndexTotal: "20", SearchTotal: "3", DocsCount: "2"},
	}, base.Add(5*time.Second))

	if _, ok := ring.samples["logs-2"]; ok {
		t.Fatal("已消失的索引应连同样本一起清理")
	}
}

// 关闭的索引在 _cat/indices 里计数列是 "-"，不应污染样本环，也不该算出速率。
func TestObserveSkipsUnparsableCounters(t *testing.T) {
	newTestDB(t)
	base := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	ring := collector.ringFor("c1")

	closed := []Index{{Index: "closed-1", IndexTotal: "-", SearchTotal: "-", DocsCount: "-"}}
	ring.observe("c1", closed, base)
	rows, window := ring.observe("c1", closed, base.Add(5*time.Second))

	if window != 0 || rows[0].IndexRate != nil {
		t.Fatalf("计数器不可解析时不应出数，实际 window=%v", window)
	}
	if len(ring.samples) != 0 {
		t.Fatalf("不可解析的索引不应进入样本环，实际 %d 项", len(ring.samples))
	}
}

func TestSnapshotRoundTrip(t *testing.T) {
	newTestDB(t)
	base := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	want := Status{Reachable: true, Name: "prod", Health: "green", NodeCount: 3, CollectedAt: base}

	if err := saveSnapshot("c1", KindStatus, want, base); err != nil {
		t.Fatalf("落库失败: %v", err)
	}
	raw, at, ok := readSnapshotRaw("c1", KindStatus)
	if !ok {
		t.Fatal("应能读回落库快照")
	}
	if !at.Equal(base) {
		t.Fatalf("采集时间应为 %v，实际 %v", base, at)
	}
	decoded, err := kindSpecs[KindStatus].decode(raw)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if got := decoded.(Status); got.Name != "prod" || got.NodeCount != 3 {
		t.Fatalf("解析结果不符: %+v", got)
	}
}

// 这是本次改动的核心场景：进程重启后进入总览页，应立刻拿到上次的快照，而不是等一次采集。
func TestGetServesPersistedSnapshotWithoutWaiting(t *testing.T) {
	newTestDB(t)
	base := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)

	if err := saveSnapshot("c1", KindStatus, Status{
		Reachable: true, Name: "prod", Health: "green", NodeCount: 3, CollectedAt: base,
	}, base); err != nil {
		t.Fatalf("落库失败: %v", err)
	}

	start := time.Now()
	data, at, ok := collector.Get("c1", KindStatus)
	elapsed := time.Since(start)

	if !ok {
		t.Fatal("有历史快照时应立即返回")
	}
	if elapsed > 500*time.Millisecond {
		t.Fatalf("冷启动不应同步等待采集，实际耗时 %v", elapsed)
	}
	if got := data.(Status); got.Name != "prod" {
		t.Fatalf("返回的应是落库快照，实际 %+v", got)
	}
	if !at.Equal(base) {
		t.Fatalf("应上报落库快照的采集时间 %v，实际 %v", base, at)
	}

	// 落库快照只读取一次，轮询不会反复打库
	if _, _, ok := collector.Get("c1", KindStatus); !ok {
		t.Fatal("第二次读取仍应命中内存缓存")
	}
}

// 从未采集过的集群（首次添加）没有历史可返回，只能等一次采集 —— 这是唯一需要等待的场景。
func TestGetWithoutHistoryFallsBackToCollecting(t *testing.T) {
	newTestDB(t)

	if _, _, ok := collector.Get("fresh-cluster", KindStatus); ok {
		t.Fatal("无历史数据的集群不应凭空返回快照")
	}
}

func TestForgetRemovesPersistedSnapshots(t *testing.T) {
	newTestDB(t)
	base := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)

	if err := saveSnapshot("c1", KindStatus, Status{Reachable: true}, base); err != nil {
		t.Fatalf("落库失败: %v", err)
	}
	collector.ringFor("c1").observe("c1", indexRows("10", "1", "1"), base)

	Forget("c1")

	if _, _, ok := readSnapshotRaw("c1", KindStatus); ok {
		t.Fatal("删除集群应清掉落库快照")
	}
	var count int
	if err := database.DB.QueryRow(`SELECT COUNT(*) FROM index_samples WHERE cluster_id = 'c1'`).Scan(&count); err != nil {
		t.Fatalf("统计样本失败: %v", err)
	}
	if count != 0 {
		t.Fatalf("删除集群应清掉索引样本，实际残留 %d 条", count)
	}
}

// 快照落库必须压缩：这是把每 5s 数兆字节的 JSON 写盘量压下去的关键。
func TestSnapshotStoreCompressesLargePayloads(t *testing.T) {
	newTestDB(t)
	base := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)

	rows := make([]Index, 0, 5000)
	for i := 0; i < 5000; i++ {
		rows = append(rows, Index{
			Index:       "logs-2026.10.09-" + strconv.Itoa(i),
			Health:      "green",
			Status:      "open",
			DocsCount:   "1000",
			IndexTotal:  "100000",
			SearchTotal: "250000",
		})
	}
	payload := IndexList{Rows: rows, RateWindowMs: 60000}

	if err := saveSnapshot("c1", KindIndices, payload, base); err != nil {
		t.Fatalf("落库失败: %v", err)
	}

	raw, _, ok := readSnapshotRaw("c1", KindIndices)
	if !ok {
		t.Fatal("应能读回落库快照")
	}
	decoded, err := kindSpecs[KindIndices].decode(raw)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	got := decoded.(IndexList)
	if len(got.Rows) != len(rows) || got.Rows[4999].Index != rows[4999].Index || got.RateWindowMs != 60000 {
		t.Fatalf("压缩往返后数据不一致: %d 行", len(got.Rows))
	}

	var stored int
	if err := database.DB.QueryRow(
		`SELECT length(data) FROM snapshots WHERE cluster_id = 'c1' AND kind = 'indices'`,
	).Scan(&stored); err != nil {
		t.Fatalf("读取存储长度失败: %v", err)
	}
	plain, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	if stored >= len(plain) {
		t.Fatalf("快照应压缩后落库：存储 %d 字节 >= 原始 %d 字节", stored, len(plain))
	}
}

// 未压缩的历史行仍要能读出来，否则这次改动就得配一次 schema 迁移。
func TestSnapshotStoreReadsLegacyUncompressedRows(t *testing.T) {
	newTestDB(t)
	base := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	plain := []byte(`{"reachable":true,"name":"legacy","health":"green"}`)

	if _, err := database.DB.Exec(
		`INSERT INTO snapshots (cluster_id, kind, data, updated_at) VALUES ('c1', 'status', ?, ?)`,
		string(plain), base.UnixMilli(),
	); err != nil {
		t.Fatalf("插入历史行失败: %v", err)
	}

	raw, at, ok := readSnapshotRaw("c1", KindStatus)
	if !ok {
		t.Fatal("应能读出未压缩的历史行")
	}
	decoded, err := kindSpecs[KindStatus].decode(raw)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if got := decoded.(Status); got.Name != "legacy" || !at.Equal(base) {
		t.Fatalf("历史行解析结果不符: %+v", got)
	}
}

func assertRate(t *testing.T, name string, got *float64, want float64) {
	t.Helper()
	if got == nil {
		t.Fatalf("%s 应有值，实际为 nil", name)
	}
	if *got != want {
		t.Fatalf("%s 应为 %v，实际 %v", name, want, *got)
	}
}
