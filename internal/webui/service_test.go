package webui

import (
	"strings"
	"testing"
	"time"

	"github.com/kennethfan/disk-cleaner/internal/model"
)

// testReport 构造一份固定的扫描结果，覆盖界面关心的几类项：
// 推荐清理的缓存、需人工确认的垃圾文件、单独成表的大文件，
// 以及永久删除与只读两种不可勾选项。
func testReport() *model.Report {
	r := &model.Report{
		ScannedAt:     time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC),
		TargetPaths:   []string{"/Users/me/Downloads"},
		ExcludedCount: 2,
		ExcludedBytes: 1 << 20,
		Errors: []model.ScanError{
			{Scanner: "系统缓存", Err: errString("permission denied")},
		},
	}
	r.Items = []model.Item{
		{Path: "/Users/me/Library/Caches/A", DisplayName: "A 缓存", Category: model.CategoryUserCache, Size: 3 << 30, IsDir: true},
		{Path: "/Users/me/Library/Caches/B", DisplayName: "B 缓存", Category: model.CategoryUserCache, Size: 1 << 30},
		{Path: "/Users/me/Downloads/big.mov", DisplayName: "big.mov", Category: model.CategoryLargeFile, Size: 500 << 20},
		{Path: "/Users/me/.Trash", DisplayName: "废纸篓", Category: model.CategoryJunk, Size: 2 << 20, Permanent: true},
		{Path: "/Users/me/Downloads/x.zip", DisplayName: "x.zip", Category: model.CategoryJunk, Size: 4 << 20, ReadOnly: true, Note: "指定目录扫描：仅可查看"},
	}
	return r
}

type errString string

func (e errString) Error() string { return string(e) }

// newTestService 构造只带报表状态的服务，不触碰真实文件系统。
func newTestService(r *model.Report) *Service {
	s := &Service{}
	s.setLatest(r)
	return s
}

// TestViewSplitsLargeAndGroupsCategories 验证报表视图：
// 大文件单独成表、其余按固定类别顺序分组、分类元数据正确。
func TestViewSplitsLargeAndGroupsCategories(t *testing.T) {
	s := newTestService(testReport())
	v := s.GetReport()
	if v == nil {
		t.Fatal("GetReport 返回 nil")
	}

	if len(v.Cats) != 2 {
		t.Fatalf("分类数 = %d, 期望 2（用户缓存、垃圾文件）", len(v.Cats))
	}
	if got := v.Cats[0].Name; got != "应用缓存" {
		t.Errorf("首个分类 = %q, 期望 应用缓存（大文件不应混进分类卡片）", got)
	}
	if got := v.Cats[1].Name; got != "垃圾文件" {
		t.Errorf("第二个分类 = %q, 期望 垃圾文件", got)
	}

	if len(v.Large) != 1 || v.Large[0].N != "big.mov" {
		t.Fatalf("大文件表 = %+v, 期望只有 big.mov", v.Large)
	}
	if len(v.Cats[0].Files) != 2 {
		t.Errorf("用户缓存项数 = %d, 期望 2", len(v.Cats[0].Files))
	}

	// 永久删除 / 只读标记必须原样透传给前端，界面据此置灰。
	junk := v.Cats[1].Files
	if !junk[0].Permanent {
		t.Errorf("废纸篓项应标记 Permanent: %+v", junk[0])
	}
	if !junk[1].ReadOnly {
		t.Errorf("指定目录项应标记 ReadOnly: %+v", junk[1])
	}

	if v.ExcludedCount != 2 || v.ExcludedBytes != 1<<20 {
		t.Errorf("排除统计 = %d 项/%d 字节, 期望 2 项/%d 字节", v.ExcludedCount, v.ExcludedBytes, 1<<20)
	}
	if len(v.Errors) != 1 || !strings.Contains(v.Errors[0], "permission denied") {
		t.Errorf("扫描器错误未透传: %v", v.Errors)
	}
}

// TestViewSkipHidesKey 验证行内排除后可以只从视图里去掉指定项，
// 不必重新扫描。
func TestViewSkipHidesKey(t *testing.T) {
	s := newTestService(testReport())
	all := s.GetReport()
	if len(all.Cats[0].Files) != 2 {
		t.Fatalf("前置条件不成立: %d 项", len(all.Cats[0].Files))
	}

	hidden := s.view(map[string]bool{all.Cats[0].Files[0].Key: true})
	if len(hidden.Cats[0].Files) != 1 {
		t.Fatalf("跳过 1 项后剩余 %d 项, 期望 1", len(hidden.Cats[0].Files))
	}
	// 原始报表不受影响（视图是快照，不修改引擎状态）。
	if again := s.GetReport(); len(again.Cats[0].Files) != 2 {
		t.Errorf("视图过滤污染了原始报表: %d 项", len(again.Cats[0].Files))
	}
}

// TestKeysRoundTripToItems 验证前端拿到的 key 能取回真实 Item，
// 且视图里的路径与大小就是引擎给出的值。
func TestKeysRoundTripToItems(t *testing.T) {
	s := newTestService(testReport())
	v := s.GetReport()

	f := v.Cats[0].Files[0]
	it, ok := s.keyOf(f.Key)
	if !ok {
		t.Fatalf("key %q 取不到 Item", f.Key)
	}
	if it.Path != f.P {
		t.Errorf("Item.Path = %q, 视图路径 = %q", it.Path, f.P)
	}
	if it.Size != f.S {
		t.Errorf("Item.Size = %d, 视图大小 = %d", it.Size, f.S)
	}
	if it.Category != model.CategoryUserCache {
		t.Errorf("Item.Category = %q, 期望 用户缓存", it.Category)
	}
}

// TestFileViewCarriesRisk 验证单项风险等级透传给前端：
// 界面据此显示中/高风险标签，「全选推荐项」据此跳过中高风险。
func TestFileViewCarriesRisk(t *testing.T) {
	r := &model.Report{Items: []model.Item{
		{Path: "/x/low", DisplayName: "低风险项", Category: model.CategoryUserCache, Risk: model.RiskLow},
		{Path: "/x/mid", DisplayName: "中风险项", Category: model.CategoryUserCache, Risk: model.RiskMedium},
		{Path: "/x/high", DisplayName: "高风险项", Category: model.CategoryUserCache, Risk: model.RiskHigh},
	}}
	v := newTestService(r).GetReport()
	if v == nil || len(v.Cats) != 1 {
		t.Fatalf("视图不成立: %+v", v)
	}

	got := make(map[string]FileView, len(v.Cats[0].Files))
	for _, f := range v.Cats[0].Files {
		got[f.N] = f
	}
	cases := []struct {
		name  string
		risk  int
		label string
	}{
		{"低风险项", 0, "低"},
		{"中风险项", 1, "中"},
		{"高风险项", 2, "高"},
	}
	for _, c := range cases {
		f, ok := got[c.name]
		if !ok {
			t.Errorf("缺少条目 %s", c.name)
			continue
		}
		if f.Risk != c.risk {
			t.Errorf("%s Risk = %d, 期望 %d", c.name, f.Risk, c.risk)
		}
		if f.RiskLabel != c.label {
			t.Errorf("%s RiskLabel = %q, 期望 %q", c.name, f.RiskLabel, c.label)
		}
	}
}

// TestSystemCacheCategoryNotRecommended 验证系统缓存分类不再标「推荐」：
// 这些路径不在 guard 白名单内，需要 sudo 处理，界面应要求人工确认。
func TestSystemCacheCategoryNotRecommended(t *testing.T) {
	r := &model.Report{Items: []model.Item{
		{Path: "/Library/Caches", DisplayName: "系统缓存", Category: model.CategorySystemCache, Size: 1 << 20, ReadOnly: true},
	}}
	v := newTestService(r).GetReport()
	if len(v.Cats) != 1 {
		t.Fatalf("分类数 = %d, 期望 1", len(v.Cats))
	}
	if v.Cats[0].Safe {
		t.Error("系统缓存分类不应标为推荐")
	}
}

// TestDisplayVolumeName 验证卷名：APFS 数据卷（主目录所在卷）不能显示成
// 挂载点末段的「Data」，外部磁盘仍显示自己的名字。
func TestDisplayVolumeName(t *testing.T) {
	startup := startupVolumeName()

	data := displayVolumeName("/System/Volumes/Data")
	if data == "Data" {
		t.Errorf("/System/Volumes/Data 显示成了挂载点末段 %q，应回退到启动磁盘名 %q", data, startup)
	}
	if data != startup {
		t.Errorf("/System/Volumes/Data → %q, 期望 %q", data, startup)
	}
	if got := displayVolumeName("/"); got != startup {
		t.Errorf("根卷 → %q, 期望 %q", got, startup)
	}
	if got := displayVolumeName("/Volumes/Backup"); got != "Backup" {
		t.Errorf("/Volumes/Backup → %q, 期望 Backup", got)
	}
}

// TestSelectItemsRejectsLockedItems 验证永久删除项与只读项在入口就被挡住，
// 界面即使误传 key 也不会让引擎尝试清理。
func TestSelectItemsRejectsLockedItems(t *testing.T) {
	s := newTestService(testReport())
	v := s.GetReport()
	junk := v.Cats[1].Files // [0] Permanent, [1] ReadOnly

	if _, err := s.selectItems([]string{junk[0].Key}); err == nil {
		t.Error("永久删除项应被拒绝")
	}
	if _, err := s.selectItems([]string{junk[1].Key}); err == nil {
		t.Error("只读项应被拒绝")
	}
	if _, err := s.selectItems([]string{"f999"}); err == nil {
		t.Error("未知 key 应被拒绝")
	}
}

// TestSelectItemsSortsBySizeDesc 验证清理顺序先大后小，
// 与前端展示顺序无关（进度通过 key 对齐）。
func TestSelectItemsSortsBySizeDesc(t *testing.T) {
	s := newTestService(testReport())
	v := s.GetReport()
	a, b := v.Cats[0].Files[0], v.Cats[0].Files[1] // 3GiB / 1GiB

	pairs, err := s.selectItems([]string{b.Key, a.Key})
	if err != nil {
		t.Fatalf("selectItems: %v", err)
	}
	if len(pairs) != 2 {
		t.Fatalf("选中 %d 项, 期望 2", len(pairs))
	}
	if pairs[0].key != a.Key {
		t.Errorf("首项 key = %q, 期望体积更大的 %q", pairs[0].key, a.Key)
	}
}
