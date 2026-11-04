#ifndef DISK_CLEANER_VOLUME_DARWIN_H
#define DISK_CLEANER_VOLUME_DARWIN_H

// dcVolumeCapacity 读取包含 path 的卷的容量（字节）。
//
// 输出参数依次为：总容量、普通可用空间（statfs 口径）、
// 与 Finder / 系统设置一致的可用空间（important usage）。
// 成功返回 0；卷信息不可读时返回非 0 错误码。
int dcVolumeCapacity(const char *path, long long *total, long long *available, long long *important);

#endif
