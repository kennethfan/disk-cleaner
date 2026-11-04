//go:build darwin && !cgo

package volinfo

import "syscall"

// queryRaw 是 CGO_ENABLED=0 时的退路：只能用 statfs。
//
// 这条路径拿不到 important usage，因此「可用」会比 Finder 少几 GB。
// 正常构建（Makefile 与 CI 都是 CGO_ENABLED=1，WebView 壳本身也依赖 cgo）
// 不会走到这里，保留它只为让 `CGO_ENABLED=0 go test ./internal/volinfo`
// 之类的场景仍能编译。
func queryRaw(path string) (rawCapacity, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return rawCapacity{}, err
	}
	bsize := int64(st.Bsize)
	if bsize <= 0 {
		bsize = 4096
	}
	avail := int64(st.Bavail) * bsize
	return rawCapacity{
		total:     int64(st.Blocks) * bsize,
		available: avail,
		important: avail,
	}, nil
}
