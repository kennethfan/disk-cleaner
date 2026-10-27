package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadMissingFileReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nope.yaml")

	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("文件不存在不应报错, got %v", err)
	}
	if len(cfg.Exclude) != 0 {
		t.Errorf("空配置不应有规则, got %v", cfg.Exclude)
	}
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "config.yaml")

	cfg := &Config{Exclude: []string{"~/Library/Caches/JetBrains", "node_modules", "*.log"}}
	if err := SaveTo(path, cfg); err != nil {
		t.Fatalf("SaveTo: %v", err)
	}

	got, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}
	if len(got.Exclude) != 3 {
		t.Fatalf("规则数 = %d, 期望 3", len(got.Exclude))
	}
	for i, want := range cfg.Exclude {
		if got.Exclude[i] != want {
			t.Errorf("规则[%d] = %q, 期望 %q", i, got.Exclude[i], want)
		}
	}
}

// TestSaveWritesCommentHeader 验证生成的配置文件带说明注释。
func TestSaveWritesCommentHeader(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := SaveTo(path, &Config{Exclude: []string{"node_modules"}}); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "# disk-cleaner 配置") {
		t.Error("缺少配置说明注释")
	}
	if !strings.Contains(content, "exclude") {
		t.Error("缺少 exclude 键")
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(path, []byte("exclude: [unclosed"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := LoadFrom(path); err == nil {
		t.Error("无效 YAML 应返回错误")
	}
}

func TestAddExclude(t *testing.T) {
	cfg := &Config{}

	added, err := cfg.AddExclude("node_modules")
	if err != nil {
		t.Fatal(err)
	}
	if !added {
		t.Error("首次添加应返回 true")
	}

	// 重复添加应被拒绝且不报错。
	added, err = cfg.AddExclude("node_modules")
	if err != nil {
		t.Fatal(err)
	}
	if added {
		t.Error("重复添加应返回 false")
	}
	if len(cfg.Exclude) != 1 {
		t.Errorf("规则数 = %d, 期望 1", len(cfg.Exclude))
	}

	// 空规则应报错。
	if _, err := cfg.AddExclude("  "); err == nil {
		t.Error("空规则应报错")
	}
}

func TestRemoveExclude(t *testing.T) {
	cfg := &Config{Exclude: []string{"a", "b", "c"}}

	if !cfg.RemoveExclude("b") {
		t.Error("删除已存在规则应返回 true")
	}
	if len(cfg.Exclude) != 2 {
		t.Errorf("规则数 = %d, 期望 2", len(cfg.Exclude))
	}
	if cfg.RemoveExclude("zzz") {
		t.Error("删除不存在规则应返回 false")
	}
}

// TestSaveIsAtomic 验证写入使用临时文件替换，不留下 .tmp。
func TestSaveIsAtomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	if err := SaveTo(path, &Config{Exclude: []string{"x"}}); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Error("不应残留 .tmp 文件")
	}
}
