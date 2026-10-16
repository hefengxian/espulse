package es

import (
	"net/url"
	"sort"
	"time"

	"github.com/hefengxian/espulse/internal/models"
)

// Recovery 是一个正在恢复的分片副本，来自 _recovery?active_only=true。
//
// 这里刻意把 ES 的嵌套响应拍平成「一行一个分片副本」：_recovery 原始结构是按索引分组的两层对象，
// 而活动区的表本身就是一维列表，保留嵌套只会让前端多一层无意义的解包（见 PRD §6.1）。
// 百分比一律不取 ES 给的字段（6.x 是 "87.1%" 这样的字符串），改由前端按字节自算。
type Recovery struct {
	Index   string `json:"index"`
	Shard   int    `json:"shard"`
	Primary bool   `json:"primary"`
	Type    string `json:"type"`
	Stage   string `json:"stage"`
	// Source 是恢复的来源节点；本地恢复（EXISTING_STORE）没有来源节点，为空。
	Source string `json:"source,omitempty"`
	Target string `json:"target,omitempty"`

	FilesRecovered int64 `json:"files_recovered"`
	FilesTotal     int64 `json:"files_total"`
	BytesRecovered int64 `json:"bytes_recovered"`
	BytesTotal     int64 `json:"bytes_total"`
	// TranslogTotal 为 0 表示这一步没有工作量（如本地恢复），前端显示 "—" 而非 100%。
	TranslogRecovered int64 `json:"translog_recovered"`
	TranslogTotal     int64 `json:"translog_total"`

	StartTimeMs int64 `json:"start_time_ms"`
	ElapsedMs   int64 `json:"elapsed_ms"`
}

// recoveryShard 是 _recovery 响应中索引下的一个分片副本，只取活动区需要的字段。
type recoveryShard struct {
	ID      int    `json:"id"`
	Type    string `json:"type"`
	Stage   string `json:"stage"`
	Primary bool   `json:"primary"`
	Source  struct {
		Name string `json:"name"`
	} `json:"source"`
	Target struct {
		Name string `json:"name"`
	} `json:"target"`
	Index struct {
		Size struct {
			TotalInBytes     int64 `json:"total_in_bytes"`
			RecoveredInBytes int64 `json:"recovered_in_bytes"`
		} `json:"size"`
		Files struct {
			Total     int64 `json:"total"`
			Recovered int64 `json:"recovered"`
		} `json:"files"`
	} `json:"index"`
	Translog struct {
		Total     int64 `json:"total"`
		Recovered int64 `json:"recovered"`
	} `json:"translog"`
	StartTimeInMillis int64 `json:"start_time_in_millis"`
	TotalTimeInMillis int64 `json:"total_time_in_millis"`
}

type recoveryIndex struct {
	Shards []recoveryShard `json:"shards"`
}

// fetchRecovery 采集当前仍在进行的恢复。
//
// active_only=true 不能省：_recovery 默认会把集群里**每一个分片副本**的最后一次恢复都报出来
// （万级索引下是 MB 级响应），而活动区只关心「此刻还在跑」的那些；空闲时该接口返回 {}，代价接近零。
func fetchRecovery(cluster models.Cluster) (any, error) {
	query := url.Values{}
	query.Set("active_only", "true")

	var byIndex map[string]recoveryIndex
	if err := TargetFromCluster(cluster).getJSON(CatalogClient, "/_recovery", query, &byIndex); err != nil {
		return nil, err
	}

	// 按索引名 + 分片号排序：ES 不保证索引之间的顺序，排序后每次采集的行序稳定，
	// 前端刷新时行不会无缘无故跳动。
	names := make([]string, 0, len(byIndex))
	for name := range byIndex {
		names = append(names, name)
	}
	sort.Strings(names)

	rows := []Recovery{}
	for _, name := range names {
		shards := byIndex[name].Shards
		sort.Slice(shards, func(i, j int) bool { return shards[i].ID < shards[j].ID })
		for _, shard := range shards {
			rows = append(rows, flattenRecovery(name, shard))
		}
	}
	return rows, nil
}

func flattenRecovery(index string, s recoveryShard) Recovery {
	elapsed := s.TotalTimeInMillis
	// 进行中的恢复由 ES 动态给出 total_time；个别版本在未完成时会留 0，此时按起始时间自算。
	if elapsed <= 0 && s.StartTimeInMillis > 0 {
		elapsed = time.Now().UnixMilli() - s.StartTimeInMillis
	}

	return Recovery{
		Index:             index,
		Shard:             s.ID,
		Primary:           s.Primary,
		Type:              s.Type,
		Stage:             s.Stage,
		Source:            s.Source.Name,
		Target:            s.Target.Name,
		FilesRecovered:    s.Index.Files.Recovered,
		FilesTotal:        s.Index.Files.Total,
		BytesRecovered:    s.Index.Size.RecoveredInBytes,
		BytesTotal:        s.Index.Size.TotalInBytes,
		TranslogRecovered: s.Translog.Recovered,
		TranslogTotal:     s.Translog.Total,
		StartTimeMs:       s.StartTimeInMillis,
		ElapsedMs:         elapsed,
	}
}

// GetRecovery 读取某集群正在进行的恢复，并把该集群标记为活跃（见 PRD §6.3）。
func GetRecovery(clusterID string) ([]Recovery, time.Time, bool) {
	data, at, has := collector.Get(clusterID, KindRecovery)
	if !has {
		return nil, at, false
	}
	rows, ok := data.([]Recovery)
	if !ok {
		return nil, at, false
	}
	return rows, at, true
}
