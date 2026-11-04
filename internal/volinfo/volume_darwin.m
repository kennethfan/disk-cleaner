// 卷容量查询的 Objective-C 实现。
//
// 放在独立的 .m 文件里而不是 cgo 注释块中：cgo 默认按 C 编译注释块，
// Foundation 头文件含 Objective-C 语法（nullable、协议声明等），
// 直接内联会报上千条语法错误；独立 .m 文件由 clang 按语言自动识别。

#import <Foundation/Foundation.h>

#include "volume_darwin.h"

int dcVolumeCapacity(const char *path, long long *total, long long *available, long long *important) {
    if (path == NULL || total == NULL || available == NULL || important == NULL) {
        return 1;
    }

    // autoreleasepool 必不可少：cgo 调进来的线程不一定有活动的 pool，
    // 临时对象会一直累积到线程结束。
    @autoreleasepool {
        NSString *p = [NSString stringWithUTF8String:path];
        if (p == nil) {
            return 2;
        }

        NSError *error = nil;
        NSDictionary *values = [[NSURL fileURLWithPath:p] resourceValuesForKeys:@[
            NSURLVolumeTotalCapacityKey,
            NSURLVolumeAvailableCapacityKey,
            NSURLVolumeAvailableCapacityForImportantUsageKey,
        ] error:&error];
        if (values == nil) {
            return 3;
        }

        NSNumber *totalNum = [values objectForKey:NSURLVolumeTotalCapacityKey];
        NSNumber *availNum = [values objectForKey:NSURLVolumeAvailableCapacityKey];
        NSNumber *importantNum = [values objectForKey:NSURLVolumeAvailableCapacityForImportantUsageKey];

        // 总容量与普通可用空间缺失就当作失败；important usage 在个别卷上
        // 可能不存在，交给 Go 侧回退到普通可用空间。
        if (totalNum == nil || availNum == nil) {
            return 4;
        }

        *total = [totalNum longLongValue];
        *available = [availNum longLongValue];
        *important = importantNum == nil ? 0 : [importantNum longLongValue];
        return 0;
    }
}
