package stats

import (
	"sort"
	"time"
)

// CallLogFilter 定义调用日志查询的过滤和分页参数。
type CallLogFilter struct {
	Model    string
	Status   string // "success" | "failed" | ""
	Page     int
	PageSize int
}

// CallLogResult 是过滤分页后的调用日志查询结果。
type CallLogResult struct {
	Records  []CallRecord `json:"records"`
	Total    int          `json:"total"`
	Page     int          `json:"page"`
	PageSize int          `json:"page_size"`
}

// QueryLogs 从 since 之后的记录中按条件过滤、分页，返回最新在前。
func (s *Store) QueryLogs(since time.Time, filter CallLogFilter) CallLogResult {
	if s == nil {
		return CallLogResult{}
	}

	records, err := s.recordsSince(since)
	if err != nil {
		return CallLogResult{}
	}

	// 按时间倒序（最新在前）
	sort.SliceStable(records, func(i, j int) bool {
		return records[i].At.After(records[j].At)
	})

	// 过滤
	filtered := make([]CallRecord, 0, len(records))
	for _, r := range records {
		if filter.Model != "" && r.Model != filter.Model {
			continue
		}
		switch filter.Status {
		case "success":
			if !r.Success {
				continue
			}
		case "failed":
			if r.Success {
				continue
			}
		}
		filtered = append(filtered, r)
	}

	total := len(filtered)

	// 分页
	page := filter.Page
	if page <= 0 {
		page = 1
	}
	pageSize := filter.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 200 {
		pageSize = 200
	}

	start := (page - 1) * pageSize
	if start > len(filtered) {
		start = len(filtered)
	}
	end := start + pageSize
	if end > len(filtered) {
		end = len(filtered)
	}

	return CallLogResult{
		Records:  filtered[start:end],
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}
}
