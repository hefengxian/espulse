package es

import (
	"bytes"
	"compress/gzip"
	"database/sql"
	"encoding/json"
	"io"
	"log"
	"time"

	"github.com/hefengxian/espulse/internal/database"
)

// 本文件是「快照落库」的全部 SQL。分成两类数据：
//
//   - snapshots：每个 (集群, kind) 最新的一份快照。用于冷启动秒开 —— 无论是因为空闲超过
//     idleTTL 被淘汰，还是因为进程重启，都能先把旧数据画出来再后台刷新（见 PRD §6.3）。
//   - index_samples：索引累计计数的短期样本环，只服务于写入 / 搜索速率的就地差分。
//     它是有界、不可任意查询的（见 PRD §11），保留期即 rateWindow。
//
// 落库失败不影响采集结果：快照已经在内存缓存里，页面照常可用，只记日志。

// saveSnapshot 写入 / 覆盖某集群某类数据的最新快照。
func saveSnapshot(clusterID string, kind Kind, payload any, at time.Time) error {
	raw, err := encodeSnapshot(payload)
	if err != nil {
		return err
	}
	_, err = database.DB.Exec(
		`INSERT INTO snapshots (cluster_id, kind, data, updated_at) VALUES (?, ?, ?, ?)
		 ON CONFLICT(cluster_id, kind) DO UPDATE SET data = excluded.data, updated_at = excluded.updated_at`,
		clusterID, string(kind), raw, at.UnixMilli(),
	)
	return err
}

// encodeSnapshot 把快照序列化并压缩。
//
// 采集器每 5s 就要重写一次快照，而 _cat/shards、_cat/indices 的 JSON 重复度极高：
// 实测某 7000 索引的集群里，shards 快照 6.6MB、indices 快照 3.2MB，gzip 一级分别压到
// 400KB / 343KB（16.5x / 9.4x）。不压缩意味着 2MB/s 的持续写盘，压缩后降到约 150KB/s，
// 代价是每轮几十毫秒的 CPU。用 BestSpeed 而非默认级别，因为这里要的是省 I/O 而不是极限压缩率。
func encodeSnapshot(payload any) ([]byte, error) {
	var buf bytes.Buffer
	zw, err := gzip.NewWriterLevel(&buf, gzip.BestSpeed)
	if err != nil {
		return nil, err
	}
	if err := json.NewEncoder(zw).Encode(payload); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// decodeSnapshot 解压快照。靠 gzip 魔数判断，未压缩的历史行也能原样读出，
// 因此这次改动不需要 schema 迁移。
func decodeSnapshot(raw []byte) ([]byte, error) {
	if len(raw) < 2 || raw[0] != 0x1f || raw[1] != 0x8b {
		return raw, nil
	}
	zr, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	defer func() { _ = zr.Close() }()
	return io.ReadAll(zr)
}

// readSnapshotRaw 读取落库快照的原始 JSON 与采集时间。
// 从未采集过该 kind 时 ok=false；查询或解压出错时同样返回 ok=false，但会记日志。
func readSnapshotRaw(clusterID string, kind Kind) ([]byte, time.Time, bool) {
	var (
		raw string
		ms  int64
	)
	err := database.DB.QueryRow(
		`SELECT data, updated_at FROM snapshots WHERE cluster_id = ? AND kind = ?`,
		clusterID, string(kind),
	).Scan(&raw, &ms)
	if err != nil {
		if err != sql.ErrNoRows {
			log.Printf("读取落库快照失败 (cluster=%s, kind=%s): %v", clusterID, kind, err)
		}
		return nil, time.Time{}, false
	}
	decoded, err := decodeSnapshot([]byte(raw))
	if err != nil {
		log.Printf("解压落库快照失败 (cluster=%s, kind=%s): %v", clusterID, kind, err)
		return nil, time.Time{}, false
	}
	return decoded, time.UnixMilli(ms), true
}

// indexSampleRow 是一条待落库的索引样本（本轮采集的累计计数）。
type indexSampleRow struct {
	Index       string
	IndexTotal  int64
	SearchTotal int64
	DocsCount   int64
}

// saveIndexSamples 批量写入本轮样本，并裁剪窗口之外的旧样本。
// 两者放在同一事务里，避免「写了新的、没删旧的」导致样本环无限增长。
func saveIndexSamples(clusterID string, at time.Time, rows []indexSampleRow, cutoff time.Time) error {
	// 本轮没有样本（如索引全被删除）时也要裁剪，否则旧样本会一直留着
	if len(rows) == 0 {
		_, err := database.DB.Exec(
			`DELETE FROM index_samples WHERE cluster_id = ? AND at < ?`,
			clusterID, cutoff.UnixMilli(),
		)
		return err
	}

	tx, err := database.DB.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.Prepare(
		`INSERT INTO index_samples (cluster_id, at, index_name, idx_total, search_total, docs_count)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(cluster_id, at, index_name) DO NOTHING`,
	)
	if err != nil {
		return err
	}
	defer func() { _ = stmt.Close() }()

	atMs := at.UnixMilli()
	for _, row := range rows {
		if _, err := stmt.Exec(clusterID, atMs, row.Index, row.IndexTotal, row.SearchTotal, row.DocsCount); err != nil {
			return err
		}
	}

	if _, err := tx.Exec(
		`DELETE FROM index_samples WHERE cluster_id = ? AND at < ?`,
		clusterID, cutoff.UnixMilli(),
	); err != nil {
		return err
	}

	return tx.Commit()
}

// loadIndexSamples 读取某集群窗口内的全部样本，按「索引名 -> 按时间升序的样本」组织。
func loadIndexSamples(clusterID string, since time.Time) (map[string][]rateSample, error) {
	rows, err := database.DB.Query(
		`SELECT index_name, at, idx_total, search_total, docs_count FROM index_samples
		 WHERE cluster_id = ? AND at >= ? ORDER BY at`,
		clusterID, since.UnixMilli(),
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	samples := make(map[string][]rateSample)
	for rows.Next() {
		var (
			name        string
			ms          int64
			indexTotal  int64
			searchTotal int64
			docsCount   int64
		)
		if err := rows.Scan(&name, &ms, &indexTotal, &searchTotal, &docsCount); err != nil {
			return nil, err
		}
		samples[name] = append(samples[name], rateSample{
			At:          time.UnixMilli(ms),
			IndexTotal:  indexTotal,
			SearchTotal: searchTotal,
			DocsCount:   docsCount,
		})
	}
	return samples, rows.Err()
}

// pruneAllIndexSamples 全局裁剪过期样本，用于长时间停机后回收空间。
func pruneAllIndexSamples(cutoff time.Time) error {
	_, err := database.DB.Exec(`DELETE FROM index_samples WHERE at < ?`, cutoff.UnixMilli())
	return err
}

// forgetSnapshots 删除某集群的全部落库数据（快照 + 样本），随集群一同删除。
func forgetSnapshots(clusterID string) error {
	if _, err := database.DB.Exec(`DELETE FROM snapshots WHERE cluster_id = ?`, clusterID); err != nil {
		return err
	}
	_, err := database.DB.Exec(`DELETE FROM index_samples WHERE cluster_id = ?`, clusterID)
	return err
}
