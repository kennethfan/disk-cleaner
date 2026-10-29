package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/kennethfan/disk-cleaner/internal/model"
)

// TestPrintReportMarksLockedAndExcludesFromTotal 验证 CLI 预览：
// 只读与永久删除项被标注，且不计入「可清理」合计。
func TestPrintReportMarksLockedAndExcludesFromTotal(t *testing.T) {
	r := &model.Report{
		ScannedAt: time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC),
		Items: []model.Item{
			{Path: "/Users/x/Library/Caches/A", DisplayName: "A 缓存", Category: model.CategoryUserCache, Risk: model.RiskLow, Size: 1000},
			{Path: "/Library/Caches", DisplayName: "系统缓存", Category: model.CategorySystemCache, Risk: model.RiskLow, Size: 9000, ReadOnly: true, Note: "需 sudo"},
			{Path: "/Users/x/.Trash", DisplayName: "废纸篓", Category: model.CategoryJunk, Risk: model.RiskHigh, Size: 500, Permanent: true},
		},
	}

	var buf bytes.Buffer
	printReport(&buf, r)
	out := buf.String()

	if !strings.Contains(out, "共发现 3 项，可清理 1000 B") {
		t.Errorf("合计应只含可直接清理项:\n%s", out)
	}
	if !strings.Contains(out, "另有 2 项只读或需专用流程") {
		t.Errorf("应说明被排除的项数与体积:\n%s", out)
	}
	if !strings.Contains(out, "[只读]") {
		t.Errorf("只读项应带标记:\n%s", out)
	}
	if !strings.Contains(out, "[永久删除]") {
		t.Errorf("永久删除项应带标记:\n%s", out)
	}
	// 系统缓存整类都不可直接清理，分类头应显示 0 B 并单列锁定体积。
	if !strings.Contains(out, "== 系统缓存 ==  可清理 0 B（另 1 项不可直接清理，8.8 KiB）") {
		t.Errorf("分类头未排除只读项:\n%s", out)
	}
	if !strings.Contains(out, "需 sudo") {
		t.Errorf("原有说明应保留:\n%s", out)
	}
}
