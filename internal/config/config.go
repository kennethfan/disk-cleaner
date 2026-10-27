// Package config 读写 disk-cleaner 的用户配置。
//
// 配置文件位于 ~/.config/disk-cleaner/config.yaml。
// 读取失败时不会中断程序：配置只影响便利性，不应阻断核心功能。
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// FileName 是配置文件名。
const FileName = "config.yaml"

// Config 是持久化的用户配置。
type Config struct {
	// Exclude 是排除规则列表，语法见 exclude 包。
	Exclude []string `yaml:"exclude"`
}

// DefaultPath 返回配置文件的标准位置。
//
// 刻意不用 os.UserConfigDir()：它在 macOS 上返回
// ~/Library/Application Support，对命令行工具来说既深又难记。
// 这里统一用 ~/.config/disk-cleaner/config.yaml，与多数 CLI 工具
// （git、gh、ripgrep 等）的习惯一致，也便于用户手动编辑。
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("获取用户主目录: %w", err)
	}
	return filepath.Join(home, ".config", "disk-cleaner", FileName), nil
}

// Load 从默认位置读取配置。
//
// 文件不存在时返回空配置且不报错——首次运行属于正常情况。
// 解析失败时返回错误，由调用方决定是否忽略。
func Load() (*Config, string, error) {
	path, err := DefaultPath()
	if err != nil {
		return &Config{}, "", err
	}
	cfg, err := LoadFrom(path)
	return cfg, path, err
}

// LoadFrom 从指定路径读取配置。
func LoadFrom(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &Config{}, nil
		}
		return nil, fmt.Errorf("读取配置 %s: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("解析配置 %s: %w", path, err)
	}
	return &cfg, nil
}

// Save 写入默认位置的配置。
func Save(cfg *Config) (string, error) {
	path, err := DefaultPath()
	if err != nil {
		return "", err
	}
	return path, SaveTo(path, cfg)
}

// SaveTo 写入指定路径，自动创建父目录。
func SaveTo(path string, cfg *Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("创建配置目录: %w", err)
	}

	var sb strings.Builder
	sb.WriteString("# disk-cleaner 配置\n")
	sb.WriteString("#\n")
	sb.WriteString("# exclude: 扫描时排除的路径或名称，支持三种写法\n")
	sb.WriteString("#   - ~/Library/Caches/JetBrains   绝对路径（含其全部子项）\n")
	sb.WriteString("#   - node_modules                 任意层级中同名的目录\n")
	sb.WriteString("#   - \"*.log\"                      通配符（匹配文件名）\n")
	sb.WriteString("\n")

	body, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("序列化配置: %w", err)
	}
	sb.Write(body)

	// 先写临时文件再原子替换，避免写入中断导致配置损坏。
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(sb.String()), 0o644); err != nil {
		return fmt.Errorf("写入配置: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("保存配置: %w", err)
	}
	return nil
}

// AddExclude 追加一条排除规则并保存。已存在相同规则时不做改动。
func (c *Config) AddExclude(rule string) (bool, error) {
	rule = strings.TrimSpace(rule)
	if rule == "" {
		return false, errors.New("排除规则不能为空")
	}
	for _, e := range c.Exclude {
		if e == rule {
			return false, nil
		}
	}
	c.Exclude = append(c.Exclude, rule)
	return true, nil
}

// RemoveExclude 删除一条排除规则。返回是否真的删除了。
func (c *Config) RemoveExclude(rule string) bool {
	for i, e := range c.Exclude {
		if e == rule {
			c.Exclude = append(c.Exclude[:i], c.Exclude[i+1:]...)
			return true
		}
	}
	return false
}
