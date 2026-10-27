package main

import (
	"fmt"

	"github.com/kennethfan/disk-cleaner/internal/config"
	"github.com/kennethfan/disk-cleaner/internal/exclude"
)

// buildExcludeMatcher 合并配置文件与命令行给出的排除规则。
//
// 返回的 warning 非空表示配置读取失败；此时仍使用命令行规则，
// 不阻断扫描——排除列表只是便利功能，不应让整个命令失败。
func buildExcludeMatcher(cliRules []string) (*exclude.Matcher, string) {
	var warn string

	cfg, path, err := config.Load()
	if err != nil {
		warn = fmt.Sprintf("读取配置 %s 失败（仅使用命令行规则）: %v", path, err)
		cfg = &config.Config{}
	}

	all := make([]string, 0, len(cfg.Exclude)+len(cliRules))
	all = append(all, cfg.Exclude...)
	all = append(all, cliRules...)

	m, err := exclude.New(all, "")
	if err != nil {
		// 规则写错属于用户输入错误，必须明确告知，
		// 否则用户会以为排除已生效。
		warn = fmt.Sprintf("排除规则无效: %v", err)
		return &exclude.Matcher{}, warn
	}
	return m, warn
}
