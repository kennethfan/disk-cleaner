// Package trash 负责把文件安全地移入 macOS 废纸篓，而不是直接 unlink。
//
// 移入废纸篓的意义：用户可随时从废纸篓恢复，误删代价低。
// 命名冲突时自动追加序号，绝不覆盖废纸篓中已有条目。
package trash

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// ErrSourceMissing 源路径不存在。
var ErrSourceMissing = errors.New("源路径不存在")

// Trasher 把目标移动到指定的废纸篓目录。
type Trasher struct {
	// Dir 是废纸篓目录，通常为 ~/.Trash。
	Dir string
}

// New 创建 Trasher。dir 为空时返回错误，避免调用方误用系统根目录。
func New(dir string) (*Trasher, error) {
	if strings.TrimSpace(dir) == "" {
		return nil, errors.New("废纸篓目录不能为空")
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("解析废纸篓目录: %w", err)
	}
	return &Trasher{Dir: filepath.Clean(abs)}, nil
}

// DefaultDir 返回当前用户的废纸篓目录。
func DefaultDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("获取用户主目录: %w", err)
	}
	return filepath.Join(home, ".Trash"), nil
}

// Move 把 src 移动到废纸篓，返回落点路径。
// 同名条目已存在时，依次尝试 "name 2"、"name 3"…… 直到找到空位。
func (t *Trasher) Move(src string) (string, error) {
	info, err := os.Lstat(src)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("%w: %s", ErrSourceMissing, src)
		}
		return "", fmt.Errorf("读取源路径 %s: %w", src, err)
	}

	if err := os.MkdirAll(t.Dir, 0o700); err != nil {
		return "", fmt.Errorf("准备废纸篓目录 %s: %w", t.Dir, err)
	}

	base := filepath.Base(src)
	dest, err := t.uniqueDest(base)
	if err != nil {
		return "", err
	}

	// 同卷优先用 rename：原子且不复制数据。
	renameErr := os.Rename(src, dest)
	if renameErr == nil {
		return dest, nil
	}
	// 只有跨卷错误才走"复制后删除"的回退路径；
	// 其他错误（权限不足等）直接上报，避免误判为成功。
	if !errors.Is(renameErr, syscall.EXDEV) {
		return "", fmt.Errorf("移动到废纸篓 %s: %w", dest, renameErr)
	}

	// 跨卷：复制后再删除源。
	if info.IsDir() {
		if err := copyDir(src, dest); err != nil {
			return "", fmt.Errorf("跨卷复制目录到废纸篓: %w", err)
		}
	} else {
		if err := copyFile(src, dest, info.Mode()); err != nil {
			return "", fmt.Errorf("跨卷复制文件到废纸篓: %w", err)
		}
	}
	if err := os.RemoveAll(src); err != nil {
		return "", fmt.Errorf("复制成功但删除源失败 %s: %w", src, err)
	}
	return dest, nil
}

// uniqueDest 在废纸篓中为 base 找一个不冲突的落点。
func (t *Trasher) uniqueDest(base string) (string, error) {
	candidate := filepath.Join(t.Dir, base)
	if !exists(candidate) {
		return candidate, nil
	}
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	for i := 2; i <= 10000; i++ {
		candidate = filepath.Join(t.Dir, fmt.Sprintf("%s %d%s", stem, i, ext))
		if !exists(candidate) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("废纸篓中同名条目过多，放弃: %s", base)
}

func exists(p string) bool {
	_, err := os.Lstat(p)
	return err == nil
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode.Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(dst)
		return err
	}
	return out.Close()
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode().Perm())
		}
		if !info.Mode().IsRegular() {
			// 符号链接等特殊文件按链接本身复制，避免跟随到目标之外。
			if info.Mode()&os.ModeSymlink != 0 {
				link, lerr := os.Readlink(path)
				if lerr != nil {
					return lerr
				}
				return os.Symlink(link, target)
			}
			return nil
		}
		return copyFile(path, target, info.Mode())
	})
}
