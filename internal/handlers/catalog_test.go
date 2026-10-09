package handlers

import (
	"strings"
	"testing"

	"github.com/hefengxian/espulse/internal/es"
)

func ratePtr(v float64) *float64 { return &v }

func indexNames(rows []es.Index) string {
	names := make([]string, 0, len(rows))
	for _, row := range rows {
		names = append(names, row.Index)
	}
	return strings.Join(names, ",")
}

// 速率缺失的行必须恒排最后：降序看热点时，最前面不该是一片 "—"。
func TestSortIndicesByRateKeepsMissingValuesLast(t *testing.T) {
	rows := []es.Index{
		{Index: "a", IndexRate: nil, SearchRate: ratePtr(5)},
		{Index: "b", IndexRate: ratePtr(1), SearchRate: nil},
		{Index: "c", IndexRate: ratePtr(9), SearchRate: ratePtr(2)},
	}

	tests := []struct {
		name  string
		key   string
		order string
		want  string
	}{
		{"写入速率升序", "index_rate", "asc", "b,c,a"},
		{"写入速率降序", "index_rate", "desc", "c,b,a"},
		{"搜索速率升序", "search_rate", "asc", "c,a,b"},
		{"搜索速率降序", "search_rate", "desc", "a,c,b"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := indexNames(sortIndices(rows, tc.key, tc.order))
			if got != tc.want {
				t.Fatalf("排序结果应为 %s，实际 %s", tc.want, got)
			}
		})
	}

	// 排序不得就地修改被并发读取的共享采集缓存
	if indexNames(rows) != "a,b,c" {
		t.Fatalf("入参切片被就地修改：%s", indexNames(rows))
	}
}
