package main

import (
	"encoding/json"
	"os"

	"github.com/kennethfan/disk-cleaner/internal/model"
)

// jsonItem 是 Item 的 JSON 投影，补充人类可读的大小字段。
type jsonItem struct {
	Path        string `json:"path"`
	DisplayName string `json:"display_name"`
	Category    string `json:"category"`
	Risk        string `json:"risk"`
	Size        int64  `json:"size"`
	SizeHuman   string `json:"size_human"`
	IsDir       bool   `json:"is_dir"`
	Permanent   bool   `json:"permanent"`
	ReadOnly    bool   `json:"read_only"`
	Note        string `json:"note,omitempty"`
}

// jsonReport 是 Report 的 JSON 投影。
//
// total_bytes 是全部发现项（含只读/永久删除），cleanable_bytes 才是
// 可直接清理的部分，脚本判断可回收空间应使用后者。
type jsonReport struct {
	ScannedAt      string     `json:"scanned_at"`
	TotalBytes     int64      `json:"total_bytes"`
	TotalHuman     string     `json:"total_human"`
	CleanableCount int        `json:"cleanable_count"`
	CleanableBytes int64      `json:"cleanable_bytes"`
	CleanableHuman string     `json:"cleanable_human"`
	Items          []jsonItem `json:"items"`
	Errors         []string   `json:"errors,omitempty"`
}

// printJSON 把报告以 JSON 写到 stdout，便于脚本消费。
func printJSON(r *model.Report) error {
	cleanCount, cleanBytes, _, _ := cleanableStats(r.Items)
	out := jsonReport{
		ScannedAt:      r.ScannedAt.Format("2006-01-02T15:04:05Z07:00"),
		TotalBytes:     r.TotalBytes,
		TotalHuman:     model.HumanBytes(r.TotalBytes),
		CleanableCount: cleanCount,
		CleanableBytes: cleanBytes,
		CleanableHuman: model.HumanBytes(cleanBytes),
		Items:          make([]jsonItem, 0, len(r.Items)),
	}
	for _, it := range r.Items {
		out.Items = append(out.Items, jsonItem{
			Path:        it.Path,
			DisplayName: it.DisplayName,
			Category:    string(it.Category),
			Risk:        it.Risk.String(),
			Size:        it.Size,
			SizeHuman:   model.HumanBytes(it.Size),
			IsDir:       it.IsDir,
			Permanent:   it.Permanent,
			ReadOnly:    it.ReadOnly,
			Note:        it.Note,
		})
	}
	for _, e := range r.Errors {
		out.Errors = append(out.Errors, e.Error())
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
