//go:build darwin && cgo

package volinfo

/*
#cgo LDFLAGS: -framework Foundation
#include <stdlib.h>
#include "volume_darwin.h"
*/
import "C"

import (
	"fmt"
	"unsafe"
)

// queryRaw 通过 Foundation 的 NSURL 资源值读取容量。
//
// 这里必须走 Foundation 而不是 statfs：只有
// NSURLVolumeAvailableCapacityForImportantUsageKey 才包含系统可回收空间，
// 与 Finder / 系统设置显示的数字同源。
func queryRaw(path string) (rawCapacity, error) {
	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))

	var (
		total     C.longlong
		available C.longlong
		important C.longlong
	)
	if rc := C.dcVolumeCapacity(cpath, &total, &available, &important); rc != 0 {
		return rawCapacity{}, fmt.Errorf("读取卷容量失败（%s，错误码 %d）", path, int(rc))
	}
	return rawCapacity{
		total:     int64(total),
		available: int64(available),
		important: int64(important),
	}, nil
}
