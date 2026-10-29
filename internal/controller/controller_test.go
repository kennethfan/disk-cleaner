package controller

import (
	"strings"
	"testing"

	"github.com/kennethfan/disk-cleaner/internal/clean"
	"github.com/kennethfan/disk-cleaner/internal/guard"
	"github.com/kennethfan/disk-cleaner/internal/model"
)

// TestMarkUncleanableMarksGuardRejectedItems 验证扫描结果与 guard 对齐：
// guard 不允许清理的项在界面上被标为只读，允许的项保持可勾选。
func TestMarkUncleanableMarksGuardRejectedItems(t *testing.T) {
	home := "/Users/tester"
	g, err := guard.New(clean.DefaultRoots(home), home)
	if err != nil {
		t.Fatalf("guard.New: %v", err)
	}
	r := &model.Report{Items: []model.Item{
		{Path: home + "/Library/Caches/A", Category: model.CategoryUserCache},
		{Path: home + "/Downloads/x.zip", Category: model.CategoryJunk},
		{Path: "/Library/Caches", Category: model.CategorySystemCache},
		{Path: home + "/Documents/report.pdf", Category: model.CategoryLargeFile, Note: "原有说明"},
		{Path: home + "/Library/Application Support/Google/Chrome/History", Category: model.CategoryLargeFile},
		{Path: "com.apple.TimeMachine.2026-10-29-120000", Category: model.CategorySnapshot, Permanent: true},
		{Path: home + "/Downloads/ro.bin", Category: model.CategoryJunk, ReadOnly: true, Note: "指定目录扫描：仅可查看"},
	}}
	MarkUncleanable(g, r)

	// 白名单内的项不应被标只读。
	for _, i := range []int{0, 1} {
		if r.Items[i].ReadOnly {
			t.Errorf("可清理项被误标只读: %s", r.Items[i].Path)
		}
	}

	// 系统缓存（白名单外）→ 只读。
	if !r.Items[2].ReadOnly {
		t.Error("系统缓存 /Library/Caches 应标只读（guard 白名单外）")
	}

	// 受保护的用户文档 → 只读，且原有说明保留。
	if !r.Items[3].ReadOnly {
		t.Error("Documents 下的大文件应标只读（guard 保护）")
	}
	if !strings.Contains(r.Items[3].Note, "原有说明") || !strings.Contains(r.Items[3].Note, "仅可查看") {
		t.Errorf("只读说明应追加而非覆盖, got %q", r.Items[3].Note)
	}

	// 红线路径 → 只读。
	if !r.Items[4].ReadOnly {
		t.Error("Chrome 用户数据应标只读")
	}

	// 快照的 Path 是快照名，不能按路径判定，也不该被改动。
	if r.Items[5].ReadOnly {
		t.Error("快照项不应按路径标只读")
	}

	// 已经是只读的项保持不变。
	if r.Items[6].Note != "指定目录扫描：仅可查看" {
		t.Errorf("已只读项的说明被改动: %q", r.Items[6].Note)
	}
}
