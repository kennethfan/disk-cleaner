package main

import (
	"fmt"
	"strings"

	"github.com/kennethfan/disk-cleaner/internal/model"
)

// allCategories 是允许通过 --category 指定的全部类别。
var allCategories = []model.Category{
	model.CategoryUserCache,
	model.CategorySystemCache,
	model.CategoryDevCache,
	model.CategoryLargeFile,
	model.CategoryJunk,
	model.CategorySnapshot,
}

// parseCategories 校验并转换用户输入的类别名。
// 拼错或未知类别会明确报错，而不是静默匹配为空。
func parseCategories(raw []string) ([]model.Category, error) {
	known := make(map[string]model.Category, len(allCategories))
	for _, c := range allCategories {
		known[string(c)] = c
	}

	var out []model.Category
	var unknown []string
	for _, r := range raw {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		if c, ok := known[r]; ok {
			out = append(out, c)
			continue
		}
		unknown = append(unknown, r)
	}

	if len(unknown) > 0 {
		names := make([]string, 0, len(allCategories))
		for _, c := range allCategories {
			names = append(names, string(c))
		}
		return nil, fmt.Errorf("未知类别 %s；可选: %s",
			strings.Join(unknown, ", "), strings.Join(names, " / "))
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("未指定任何有效类别")
	}
	return out, nil
}
