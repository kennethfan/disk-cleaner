package scan

import (
	"bufio"
	"context"
	"os/exec"
	"strings"

	"github.com/kennethfan/disk-cleaner/internal/model"
)

// SnapshotScanner 通过 tmutil 列出 APFS 本地快照。
//
// 注意：删除快照需要 sudo，本工具不在扫描阶段提权，
// 也不自动执行删除，只在界面提示用户手动执行 tmutil 命令。
type SnapshotScanner struct{}

// NewSnapshotScanner 创建快照扫描器。
func NewSnapshotScanner(opts Options) *SnapshotScanner {
	return &SnapshotScanner{}
}

// Name 实现 Scanner。
func (s *SnapshotScanner) Name() string { return "APFS 快照" }

// Scan 实现 Scanner。
func (s *SnapshotScanner) Scan(ctx context.Context) ([]model.Item, error) {
	// tmutil 不存在时静默返回（例如非 APFS 卷）。
	if _, err := exec.LookPath("tmutil"); err != nil {
		return nil, nil
	}

	cmd := exec.CommandContext(ctx, "tmutil", "listlocalsnapshots", "/")
	out, err := cmd.Output()
	if err != nil {
		// 无快照或权限不足都属于正常情况，不作为错误上报。
		return nil, nil
	}

	var items []model.Item
	sc := bufio.NewScanner(strings.NewReader(string(out)))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "Snapshots for disk") {
			continue
		}
		// 输出形如 com.apple.TimeMachine.2026-10-26-120000.local
		items = append(items, model.Item{
			Path:        line,
			DisplayName: "快照: " + line,
			Category:    model.CategorySnapshot,
			Risk:        model.RiskHigh,
			Size:        0, // tmutil 不提供单快照大小
			IsDir:       false,
			Permanent:   true,
			Note:        "删除会丢失本地恢复点，需 sudo 手动执行：tmutil deletelocalsnapshots " + snapshotDate(line),
		})
	}
	return items, nil
}

// snapshotDate 从快照名中提取日期部分（形如 2026-10-26-120000）。
func snapshotDate(name string) string {
	const marker = "TimeMachine."
	i := strings.Index(name, marker)
	if i < 0 {
		return name
	}
	rest := name[i+len(marker):]
	rest = strings.TrimSuffix(rest, ".local")
	return rest
}
