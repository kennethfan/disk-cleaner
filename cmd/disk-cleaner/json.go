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
	Note        string `json:"note,omitempty"`
}

// jsonReport 是 Report 的 JSON 投影。
type jsonReport struct {
	ScannedAt  string     `json:"scanned_at"`
	TotalBytes int64      `json:"total_bytes"`
	TotalHuman string     `json:"total_human"`
	Items      []jsonItem `json:"items"`
	Errors     []string   `json:"errors,omitempty"`
}

// printJSON 把报告以 JSON 写到 stdout，便于脚本消费。
func printJSON(r *model.Report) error {
	out := jsonReport{
		ScannedAt:  r.ScannedAt.Format("2006-01-02T15:04:05Z07:00"),
		TotalBytes: r.TotalBytes,
		TotalHuman: model.HumanBytes(r.TotalBytes),
		Items:      make([]jsonItem, 0, len(r.Items)),
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
