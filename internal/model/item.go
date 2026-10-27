// Package model 定义扫描结果的统一数据模型。
//
// 扫描与删除彻底分离：本包只描述"发现了什么"，不包含任何删除动作。
package model

import (
	"fmt"
	"sort"
	"time"
)

// Risk 表示清理某一项的风险等级。
type Risk int

const (
	// RiskLow 纯缓存，删除后会被程序自动重建。
	RiskLow Risk = iota
	// RiskMedium 需要重新下载或重新生成才能恢复。
	RiskMedium
	// RiskHigh 可能丢失用户数据，或影响系统恢复能力。
	RiskHigh
)

// String 返回风险等级的中文描述。
func (r Risk) String() string {
	switch r {
	case RiskLow:
		return "低"
	case RiskMedium:
		return "中"
	case RiskHigh:
		return "高"
	default:
		return "未知"
	}
}

// Category 表示清理项的归类，用于界面分组。
type Category string

const (
	CategoryUserCache   Category = "用户缓存"
	CategorySystemCache Category = "系统缓存"
	CategoryDevCache    Category = "开发缓存"
	CategoryLargeFile   Category = "大文件"
	CategoryJunk        Category = "垃圾文件"
	CategorySnapshot    Category = "APFS 快照"
)

// Item 是一个可清理项。
type Item struct {
	// Path 是该项的绝对路径。快照类项目此处存快照名。
	Path string
	// DisplayName 是界面上展示的名称。
	DisplayName string
	Category    Category
	Risk        Risk
	// Size 是字节数；无法得知时为 0。
	Size  int64
	IsDir bool
	// Permanent 为 true 表示该项无法"移入废纸篓"（例如它本身就在废纸篓里），
	// 只能永久删除。界面必须据此给出更强的二次确认。
	Permanent bool
	// ReadOnly 为 true 表示该项仅可查看、不可勾选清理。
	// 用于"指定目录扫描"：用户可以浏览任意目录，但删除仍受
	// guard 白名单约束，界面提前置灰避免误导。
	ReadOnly bool
	// Note 是给用户看的补充说明，可为空。
	Note string
}

// ScanError 记录某个扫描器的非致命错误。
// 单个扫描器失败不应中断整体扫描。
type ScanError struct {
	Scanner string
	Err     error
}

// Error 实现 error 接口。
func (e ScanError) Error() string {
	return fmt.Sprintf("%s: %v", e.Scanner, e.Err)
}

// Report 是一次完整扫描的结果。
type Report struct {
	Items      []Item
	Errors     []ScanError
	TotalBytes int64
	ScannedAt  time.Time
	// ExcludedCount / ExcludedBytes 记录被排除规则过滤掉的项数与体积。
	// 界面据此提示"已排除 N 项"，让用户知道排除列表确实生效了。
	ExcludedCount int
	ExcludedBytes int64
	// TargetPaths 非空表示本次是"指定目录扫描"，这些项仅可查看。
	TargetPaths []string
}

// Finalize 按大小降序排序并汇总总字节数。ScanAll 在返回前调用。
// 大小相同时以路径为决胜条件，保证多次运行顺序一致。
func (r *Report) Finalize() {
	sort.SliceStable(r.Items, func(i, j int) bool {
		if r.Items[i].Size != r.Items[j].Size {
			return r.Items[i].Size > r.Items[j].Size
		}
		return r.Items[i].Path < r.Items[j].Path
	})
	var total int64
	for _, it := range r.Items {
		total += it.Size
	}
	r.TotalBytes = total
}

// ByCategory 按类别分组，返回各组的项与字节数。
func (r *Report) ByCategory() map[Category]CategoryGroup {
	groups := make(map[Category]CategoryGroup)
	for _, it := range r.Items {
		g := groups[it.Category]
		g.Items = append(g.Items, it)
		g.Bytes += it.Size
		groups[it.Category] = g
	}
	return groups
}

// CategoryGroup 是同一类别下的清理项集合。
type CategoryGroup struct {
	Items []Item
	Bytes int64
}

// HumanBytes 把字节数格式化为人类可读形式（KiB/MiB/GiB）。
func HumanBytes(n int64) string {
	const unit = 1024
	if n < 0 {
		return "0 B"
	}
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	const units = "KMGTPE"
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit && exp < len(units)-1; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), units[exp])
}
