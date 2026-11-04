package volinfo

import (
	"os"
	"testing"
)

// TestQueryMatchesSystemCapacity 验证查询结果可用且自洽：
// 总容量为正、可用不超过总计、可回收部分不超过可用。
//
// 这里不比对具体字节数——磁盘随时在变，任何绝对值断言都会随机失败。
// 与系统口径的一致性靠实现方式保证（同一个 Foundation 属性），
// 能测的是边界关系。
func TestQueryMatchesSystemCapacity(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("取主目录失败：%v", err)
	}

	cap, err := Query(home)
	if err != nil {
		t.Fatalf("Query(%s) 出错：%v", home, err)
	}
	if cap.Total <= 0 {
		t.Errorf("总容量 = %d, 期望正数", cap.Total)
	}
	if cap.Free < 0 || cap.Free > cap.Total {
		t.Errorf("可用 %d 超出 [0, %d]", cap.Free, cap.Total)
	}
	if cap.Reclaimable < 0 || cap.Reclaimable > cap.Free {
		t.Errorf("可回收 %d 超出 [0, 可用 %d]", cap.Reclaimable, cap.Free)
	}
}

// TestQueryUnknownPathFails 验证不存在的路径返回错误而不是零值容量，
// 让调用方能区分「读不到」和「真的是 0」。
func TestQueryUnknownPathFails(t *testing.T) {
	if _, err := Query("/definitely-not-a-real-volume-9f3a"); err == nil {
		t.Error("不存在的路径应返回错误")
	}
}
