package es

import (
	"log"
	"strconv"
	"strings"
	"sync"
	"time"
)

// 本文件实现「轻量运维信号」里的速率类派生指标：索引的写入 / 搜索速率。
//
// 做法是就地差分：把 _cat/indices 的累计计数（indexing.index_total / search.query_total）
// 按采集间隔存进一个短期样本环，用「窗口内最早样本」与「本轮样本」相减得到速率。
// 不引入任何时间范围查询能力，也不做趋势图与告警（见 PRD §11）。

// rateWindow 既是速率计算的滑动窗口，也是索引样本的保留期。
// 取 60s 是为了平滑掉单次采样的毛刺，同时让样本环天然有界。
const rateWindow = 60 * time.Second

// rateSample 是一次采集得到的索引累计计数。
type rateSample struct {
	At          time.Time
	IndexTotal  int64
	SearchTotal int64
	DocsCount   int64
}

// indexRing 保存各索引在 rateWindow 内的原始计数样本。
//
// 样本同时落库（index_samples 表），因此环在进程重启后是「温」的：
// air 热重载这类短时间重启不会让速率列断档（见 PRD §6.3）。
type indexRing struct {
	mu      sync.Mutex
	samples map[string][]rateSample
	loaded  bool
}

// ringFor 返回某集群的样本环，首次使用时从 SQLite 载入窗口内的历史样本。
func (c *Collector) ringFor(clusterID string) *indexRing {
	c.mu.Lock()
	ring, ok := c.rings[clusterID]
	if !ok {
		ring = &indexRing{samples: make(map[string][]rateSample)}
		c.rings[clusterID] = ring
	}
	c.mu.Unlock()

	ring.mu.Lock()
	defer ring.mu.Unlock()
	if !ring.loaded {
		samples, err := loadIndexSamples(clusterID, time.Now().Add(-rateWindow))
		if err != nil {
			// 不置 loaded：下次再试，避免一次读库抖动让环永久为空
			log.Printf("读取索引样本失败 (cluster=%s): %v", clusterID, err)
		} else {
			ring.samples = samples
			ring.loaded = true
		}
	}

	return ring
}

// decorateIndexRates 为索引列表补上写入 / 搜索速率。
//
// 速率来自「窗口内最早样本」与「本轮样本」的累计计数差分；样本不足时留 nil，
// 由前端显示 "—" —— 宁可不出数，也不展示没有依据的数字（见 PRD §11）。
func decorateIndexRates(clusterID string, data any) any {
	list, ok := data.(IndexList)
	if !ok {
		return data
	}

	rows, window := collector.ringFor(clusterID).observe(clusterID, list.Rows, time.Now())
	list.Rows = rows
	if window > 0 {
		list.RateWindowMs = window.Milliseconds()
	}
	return list
}

// observe 记录本轮样本，并用窗口内最早的样本为本轮各行算出速率。
// 返回补充了速率的行，以及实际参与差分的窗口长度（0 表示样本不足、未算出速率）。
func (r *indexRing) observe(clusterID string, rows []Index, at time.Time) ([]Index, time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()

	cutoff := at.Add(-rateWindow)
	seen := make(map[string]struct{}, len(rows))
	fresh := make([]indexSampleRow, 0, len(rows))
	var window time.Duration

	for i := range rows {
		indexTotal, okIndex := parseCounter(rows[i].IndexTotal)
		searchTotal, okSearch := parseCounter(rows[i].SearchTotal)
		docsCount, okDocs := parseCounter(rows[i].DocsCount)
		// 关闭的索引等场景计数器不可解析，跳过即可：它们的速率列留空，不参与样本环
		if !okIndex || !okSearch || !okDocs {
			continue
		}

		name := rows[i].Index
		seen[name] = struct{}{}

		history := r.samples[name]
		// 索引被删除重建时累计计数会归零，此前的样本已失去可比性，直接弃用
		if len(history) > 0 && indexTotal < history[len(history)-1].IndexTotal {
			history = nil
		}
		history = append(trimSamples(history, cutoff), rateSample{
			At:          at,
			IndexTotal:  indexTotal,
			SearchTotal: searchTotal,
			DocsCount:   docsCount,
		})
		r.samples[name] = history
		fresh = append(fresh, indexSampleRow{
			Index:       name,
			IndexTotal:  indexTotal,
			SearchTotal: searchTotal,
			DocsCount:   docsCount,
		})

		// 严格窗口：只认窗口内最早的样本；只有一个样本时无法差分，本轮不出数
		if len(history) < 2 {
			continue
		}
		base := history[0]
		elapsed := at.Sub(base.At)
		if elapsed < time.Second {
			continue
		}
		rows[i].IndexRate = rateOf(indexTotal, base.IndexTotal, elapsed)
		rows[i].SearchRate = rateOf(searchTotal, base.SearchTotal, elapsed)
		// docs.count 是 gauge（删除文档会让它下降），因此允许负增量
		rows[i].DocsRate = gaugeRate(docsCount, base.DocsCount, elapsed)
		if elapsed > window {
			window = elapsed
		}
	}

	// 已消失的索引：样本一并清掉，避免环随索引增删无限增长
	for name := range r.samples {
		if _, ok := seen[name]; !ok {
			delete(r.samples, name)
		}
	}

	if err := saveIndexSamples(clusterID, at, fresh, cutoff); err != nil {
		log.Printf("索引样本落库失败 (cluster=%s): %v", clusterID, err)
	}

	return rows, window
}

// trimSamples 丢弃窗口之外的样本，保持环有界（保留期即 rateWindow）。
// samples 必须按时间升序；返回新切片，不复用入参的底层数组。
func trimSamples(samples []rateSample, cutoff time.Time) []rateSample {
	keep := 0
	for keep < len(samples) && samples[keep].At.Before(cutoff) {
		keep++
	}
	if keep == 0 {
		return samples
	}
	return append([]rateSample(nil), samples[keep:]...)
}

// rateOf 计算累计计数的单位时间增量。
// 累计计数不可能倒退，一旦倒退说明索引被重建，返回 nil 而不是负数。
func rateOf(current, base int64, elapsed time.Duration) *float64 {
	if current < base {
		return nil
	}
	return gaugeRate(current, base, elapsed)
}

// gaugeRate 计算单位时间增量，允许为负（用于 gauge 类型的指标）。
func gaugeRate(current, base int64, elapsed time.Duration) *float64 {
	rate := float64(current-base) / elapsed.Seconds()
	return &rate
}

// parseCounter 解析 _cat 返回的数值列。空值或 "-"（如关闭的索引）返回 ok=false。
func parseCounter(value string) (int64, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}
	n, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}
