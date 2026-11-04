// Package volinfo 查询卷容量，统计口径与 Finder / 系统设置保持一致。
//
// macOS 对同一块盘有两套「可用空间」：
//
//   - statfs 的 Bavail：此刻就能分配的块，是磁盘工具视角的真实空闲；
//   - NSURLVolumeAvailableCapacityForImportantUsage：Finder、系统设置与
//     「关于本机」显示的可用空间，额外计入系统随时可以回收的部分
//     （APFS 预留、可清理缓存、可清除快照等）。
//
// 两者在正常使用的机器上会差几 GB。概览页展示的是给用户看的容量，
// 必须与系统设置同源，否则用户对照系统设置会认为是工具算错了。
package volinfo

import (
	"fmt"
)

// Capacity 描述一个卷的容量（字节）。
type Capacity struct {
	// Total 是卷的总容量，两套口径一致。
	Total int64
	// Free 是可用空间，与 Finder / 系统设置一致（important usage）。
	Free int64
	// Reclaimable 是 Free 中由系统按需回收的部分。
	// 它解释了「本工具的数字」与「df 的数字」为何不同。
	Reclaimable int64
}

// Query 返回包含 path 的卷的容量。
//
// path 只用于定位卷（statfs 语义），可以是不存在的深层路径，
// 只要它所在卷已挂载即可。
func Query(path string) (Capacity, error) {
	raw, err := queryRaw(path)
	if err != nil {
		return Capacity{}, err
	}
	if raw.total <= 0 {
		return Capacity{}, fmt.Errorf("卷总容量无效：%s", path)
	}

	// important usage 在极少数情况下会缺失（返回 0），此时退回普通可用空间，
	// 至少不会显示成「可用 0 GB」。
	free := raw.important
	if free <= 0 {
		free = raw.available
	}
	if free > raw.total {
		free = raw.total
	}

	// 回收空间 = 系统口径的可用 − 此刻就能分配的可用。
	// 若系统口径反而更小（statfs 与实际分配瞬时不同步），钳到 0，
	// 避免界面上出现「含可回收 -1.2 GB」。
	reclaim := free - raw.available
	if reclaim < 0 {
		reclaim = 0
	}

	return Capacity{Total: raw.total, Free: free, Reclaimable: reclaim}, nil
}

// rawCapacity 是底层返回的原始三项，未做任何口径选择。
type rawCapacity struct {
	total     int64
	available int64
	important int64
}
