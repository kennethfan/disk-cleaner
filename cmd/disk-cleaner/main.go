// Command disk-cleaner 是 macOS 磁盘空间清理工具。
//
// 一个二进制两种形态：命令行（scan 只读预览，clean 需显式 --yes）
// 与 WebView 图形界面（无参数或 gui 启动），二者共用同一套引擎。
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"

	"github.com/kennethfan/disk-cleaner/internal/clean"
	"github.com/kennethfan/disk-cleaner/internal/guard"
	"github.com/kennethfan/disk-cleaner/internal/model"
	"github.com/kennethfan/disk-cleaner/internal/scan"
	"github.com/kennethfan/disk-cleaner/internal/trash"
	"github.com/kennethfan/disk-cleaner/internal/webui"
)

func main() {
	// 无参数时默认启动图形界面。
	// 这是双击 .app 的启动路径：macOS 通过 open 启动时不会传入任何参数，
	// 若此处打印用法并退出，双击就会表现为"闪退/无反应"。
	if len(os.Args) < 2 {
		if err := webui.Run(); err != nil {
			fail(err)
		}
		return
	}
	switch os.Args[1] {
	case "gui":
		if err := webui.Run(); err != nil {
			fail(err)
		}
	case "scan":
		if err := runScan(os.Args[2:]); err != nil {
			fail(err)
		}
	case "clean":
		if err := runClean(os.Args[2:]); err != nil {
			fail(err)
		}
	case "-h", "--help", "help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "未知命令: %s\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `disk-cleaner — macOS 磁盘空间清理工具

用法:
  disk-cleaner                 直接启动图形界面（等同 gui）
  disk-cleaner gui             启动图形界面
  disk-cleaner scan  [选项]   扫描并预览可清理内容（只读，不删除任何东西）
  disk-cleaner clean [选项]   清理选定类别（默认需二次确认）

scan 选项:
  --large <MiB>   大文件阈值，默认 500
  --json          以 JSON 输出
  --path <目录>   只扫描指定目录，可重复。指定后忽略预设扫描器
  --exclude <规则> 额外排除规则，可重复。与配置文件中的规则叠加

clean 选项:
  --category <名> 只清理指定类别，可重复。默认不选任何类别（必须先指定）
  --yes           跳过交互确认（危险，仅用于脚本）
  --dry-run       只打印将要执行的动作，不实际移动文件

排除规则支持三种写法（写在 ~/.config/disk-cleaner/config.yaml 或 --exclude）:
  ~/Library/Caches/JetBrains   绝对路径，该目录及其全部子项都排除
  node_modules                 目录名，排除任意层级中同名的目录
  "*.log"                      通配符，匹配文件名

示例:
  disk-cleaner gui
  disk-cleaner scan
  disk-cleaner scan --large 1024
  disk-cleaner scan --path ~/Downloads
  disk-cleaner scan --path ~/Downloads --path ~/Movies
  disk-cleaner scan --exclude node_modules --exclude "*.log"
  disk-cleaner clean --category 用户缓存 --dry-run
  disk-cleaner clean --category 开发缓存
`)
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "错误: %v\n", err)
	os.Exit(1)
}

// newContext 返回响应 Ctrl-C / SIGTERM 的上下文。
func newContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
}

func runScan(args []string) error {
	fs := flag.NewFlagSet("scan", flag.ExitOnError)
	largeMiB := fs.Int64("large", 500, "大文件阈值 (MiB)")
	asJSON := fs.Bool("json", false, "以 JSON 输出")
	var paths multiFlag
	fs.Var(&paths, "path", "只扫描指定目录，可重复")
	var extraExclude multiFlag
	fs.Var(&extraExclude, "exclude", "额外排除规则（叠加配置文件），可重复")
	if err := fs.Parse(args); err != nil {
		return err
	}

	opts := scan.DefaultOptions()
	opts.LargeFileThreshold = *largeMiB << 20
	opts.Paths = paths

	// 排除规则 = 配置文件中的规则 + 命令行追加的规则。
	matcher, warn := buildExcludeMatcher(extraExclude)
	if warn != "" {
		fmt.Fprintf(os.Stderr, "警告: %s\n", warn)
	}
	opts.Exclude = matcher

	ctx, cancel := newContext()
	defer cancel()

	fmt.Fprintln(os.Stderr, "正在扫描……")
	report := scan.ScanAll(ctx, scan.All(opts), opts)

	if *asJSON {
		return printJSON(report)
	}
	printReport(report)
	return nil
}

// printReport 以人类可读格式打印扫描结果。
func printReport(r *model.Report) {
	groups := r.ByCategory()

	// 固定类别顺序，保证输出稳定。
	order := []model.Category{
		model.CategoryUserCache,
		model.CategorySystemCache,
		model.CategoryDevCache,
		model.CategoryLargeFile,
		model.CategoryJunk,
		model.CategorySnapshot,
	}

	fmt.Printf("\n扫描完成，共发现 %d 项，合计 %s\n", len(r.Items), model.HumanBytes(r.TotalBytes))
	fmt.Printf("扫描时间: %s\n", r.ScannedAt.Format("2006-01-02 15:04:05"))

	// 明确告知排除规则生效了，避免用户误以为扫描漏掉了内容。
	if r.ExcludedCount > 0 {
		fmt.Printf("已排除 %d 项（%s）\n", r.ExcludedCount, model.HumanBytes(r.ExcludedBytes))
	}
	if len(r.TargetPaths) > 0 {
		fmt.Printf("指定目录扫描: %s（仅可查看，不可清理）\n", strings.Join(r.TargetPaths, ", "))
	}

	for _, cat := range order {
		g, ok := groups[cat]
		if !ok || len(g.Items) == 0 {
			continue
		}
		fmt.Printf("\n== %s ==  合计 %s\n", cat, model.HumanBytes(g.Bytes))
		for _, it := range g.Items {
			marker := ""
			if it.Permanent {
				marker = " [永久删除]"
			}
			fmt.Printf("  %-10s %-9s %s%s\n",
				model.HumanBytes(it.Size), "风险:"+it.Risk.String(), it.DisplayName, marker)
			if it.Note != "" {
				fmt.Printf("             %s\n", it.Note)
			}
		}
	}

	if len(r.Errors) > 0 {
		fmt.Fprintf(os.Stderr, "\n以下扫描器出错（其余结果有效）:\n")
		for _, e := range r.Errors {
			fmt.Fprintf(os.Stderr, "  - %v\n", e)
		}
	}
}

func runClean(args []string) error {
	fs := flag.NewFlagSet("clean", flag.ExitOnError)
	yes := fs.Bool("yes", false, "跳过交互确认")
	dryRun := fs.Bool("dry-run", false, "只打印动作，不实际移动")
	var cats multiFlag
	fs.Var(&cats, "category", "只清理指定类别，可重复")
	largeMiB := fs.Int64("large", 500, "大文件阈值 (MiB)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if len(cats) == 0 {
		return fmt.Errorf("必须用 --category 指定至少一个类别（可选: 用户缓存/系统缓存/开发缓存/大文件/垃圾文件/APFS 快照）")
	}
	wantedCats, err := parseCategories(cats)
	if err != nil {
		return err
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("获取用户主目录: %w", err)
	}

	opts := scan.DefaultOptions()
	opts.LargeFileThreshold = *largeMiB << 20

	// clean 也必须应用同一套排除规则，否则"排除掉的路径"仍会被删除。
	matcher, warn := buildExcludeMatcher(nil)
	if warn != "" {
		fmt.Fprintf(os.Stderr, "警告: %s\n", warn)
	}
	opts.Exclude = matcher

	ctx, cancel := newContext()
	defer cancel()

	fmt.Fprintln(os.Stderr, "正在扫描……")
	report := scan.ScanAll(ctx, scan.All(opts), opts)

	// 按类别筛选，并排除永久删除项（它们不走废纸篓流程）。
	wanted := make(map[model.Category]bool, len(wantedCats))
	for _, c := range wantedCats {
		wanted[c] = true
	}
	var selected []model.Item
	for _, it := range report.Items {
		if !wanted[it.Category] {
			continue
		}
		selected = append(selected, it)
	}

	if len(selected) == 0 {
		fmt.Println("所选类别下没有可清理的内容。")
		return nil
	}

	sort.SliceStable(selected, func(i, j int) bool { return selected[i].Size > selected[j].Size })

	var total int64
	for _, it := range selected {
		total += it.Size
	}

	fmt.Printf("\n将处理 %d 项，合计 %s:\n", len(selected), model.HumanBytes(total))
	for _, it := range selected {
		fmt.Printf("  %-10s %s\n", model.HumanBytes(it.Size), it.DisplayName)
	}

	if *dryRun {
		fmt.Println("\n[dry-run] 未做任何改动。")
		return nil
	}

	if !*yes {
		fmt.Print("\n确认把这些项目移入废纸篓？[y/N] ")
		reader := bufio.NewReader(os.Stdin)
		line, _ := reader.ReadString('\n')
		ans := strings.ToLower(strings.TrimSpace(line))
		if ans != "y" && ans != "yes" {
			fmt.Println("已取消。")
			return nil
		}
	}

	trashDir, err := trash.DefaultDir()
	if err != nil {
		return err
	}
	trasher, err := trash.New(trashDir)
	if err != nil {
		return err
	}
	g, err := guard.New(clean.DefaultRoots(home), home)
	if err != nil {
		return err
	}
	engine, err := clean.NewEngine(g, trasher)
	if err != nil {
		return err
	}

	res := engine.Run(selected)

	fmt.Printf("\n完成: 成功 %d 项，失败 %d 项，释放 %s\n",
		res.Succeeded(), res.Failed(), model.HumanBytes(res.FreedBytes))
	for _, o := range res.Outcomes {
		if o.Err != nil {
			fmt.Printf("  ! %s: %v\n", o.Item.DisplayName, o.Err)
		} else {
			fmt.Printf("  ✓ %s -> %s\n", o.Item.DisplayName, o.Dest)
		}
	}
	if res.Succeeded() > 0 {
		fmt.Println("\n提示: 内容已移入废纸篓，可从那里恢复。")
	}
	return nil
}

// multiFlag 支持重复出现的字符串参数。
type multiFlag []string

// String 实现 flag.Value。
func (m *multiFlag) String() string { return strings.Join(*m, ",") }

// Set 实现 flag.Value。
func (m *multiFlag) Set(v string) error {
	*m = append(*m, v)
	return nil
}
