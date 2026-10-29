package ui

import (
	"path/filepath"
	"strings"
	"syscall"
)

// diskUsage 返回包含 path 的卷的总/已用/可用容量（字节）。
// 第二个返回值为 false 表示系统调用失败（理论上极少发生）。
//
// 只读用途：界面上磁盘数字如实展示，在清空废纸篓前不会
// 因清理操作而变化，这与「移入废纸篓」的真实语义一致。
func diskUsage(path string) (total, used, free int64, ok bool) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, 0, 0, false
	}
	bsize := int64(st.Bsize)
	if bsize <= 0 {
		bsize = 4096
	}
	total = int64(st.Blocks) * bsize
	free = int64(st.Bavail) * bsize
	used = total - int64(st.Bfree)*bsize
	if used < 0 {
		used = 0
	}
	if free < 0 {
		free = 0
	}
	return total, used, free, true
}

// int8SliceToString 把定长 int8 数组（Syscall 字段常见类型）里 NUL 结尾
// 的字符串转为 Go 字符串。
func int8SliceToString(b []int8) string {
	buf := make([]byte, len(b))
	for i, v := range b {
		buf[i] = byte(v)
	}
	return strings.TrimRight(string(buf), "\x00")
}

// volumeLabel 返回包含 path 的卷的展示名。
//
// Macintosh 系统卷的挂载点是 "/"，APFS 在 Catalina 之后把数据卷挂到
// /System/Volumes/...，直接取挂载点对用户不友好。这里映射为
// 常见展示名：根卷显示「Macintosh HD」，其余显示挂载点的末段。
func volumeLabel(path string) string {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return "磁盘"
	}
	mnt := int8SliceToString(st.Mntonname[:])
	if mnt == "" || mnt == "/" {
		return "Macintosh HD"
	}
	if name := filepath.Base(mnt); name != "." && name != "/" {
		return name
	}
	return mnt
}

// byteSliceToString 把定长字节数组里 NUL 结尾的字符串转为 Go 字符串。
func byteSliceToString(b []byte) string {
	return strings.TrimRight(string(b), "\x00")
}
