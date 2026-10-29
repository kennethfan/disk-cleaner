package webui

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/kennethfan/disk-cleaner/internal/clean"
	"github.com/kennethfan/disk-cleaner/internal/controller"
	"github.com/kennethfan/disk-cleaner/internal/gate"
	"github.com/kennethfan/disk-cleaner/internal/model"
)

// Service 是暴露给 WebView 前端的首个绑定服务。
//
// 它是 controller.Controller 的薄包装：扫描、排除、清理与撤销都走与
// CLI 完全相同的引擎路径。长耗时动作（扫描/清理）在后台 goroutine
// 中执行，进度通过 CustomEvent 推送给前端，暂停/取消仍然由
// Controller 维护的唯一 Gate 控制。
type Service struct {
	ctrl *controller.Controller

	// mu 保证同一时间只有一次扫描；一次扫描的生命周期是
	// StartScan 启动到 Scan 返回并派发完 scan:done。
	mu sync.Mutex

	// viewsMu 保护下面的报表状态：扫描在后台 goroutine 写入，
	// 而 Init/GetReport/Clean/ExcludeFromFile/Undo 都可能在另一条
	// 绑定调用线程同时读，必须加锁。
	viewsMu sync.RWMutex

	// report 是最近一次扫描的结果；keys 把前端 key 映射回真实 Item。
	report *model.Report
	keys   map[string]model.Item

	// lastResult 是最近一次清理的结果，供「撤销」放回废纸篓。
	lastResult *controller.CleanResult
}

// NewService 构造绑定服务。
func NewService() (*Service, error) {
	ctrl, err := controller.NewController()
	if err != nil {
		return nil, err
	}
	return &Service{ctrl: ctrl}, nil
}

// emit 向前端推送事件；应用未就绪时静默跳过。
func emit(name string, data any) {
	if app := application.Get(); app != nil {
		app.Event.EmitEvent(&application.CustomEvent{Name: name, Data: data})
	}
}

// latestReport 返回最近一次扫描结果，可能为 nil。
func (s *Service) latestReport() *model.Report {
	s.viewsMu.RLock()
	defer s.viewsMu.RUnlock()
	return s.report
}

// setLatest 写入扫描结果与由它生成的 key 索引。
func (s *Service) setLatest(report *model.Report) {
	s.viewsMu.Lock()
	defer s.viewsMu.Unlock()
	s.report = report
	s.keys = buildKeyIndex(report)
}

// keyOf 用前端 key 安全取回真实 Item。
func (s *Service) keyOf(k string) (model.Item, bool) {
	s.viewsMu.RLock()
	defer s.viewsMu.RUnlock()
	it, ok := s.keys[k]
	return it, ok
}

// ==================== 初始状态 ====================

// DiskInfo 是概览页需要的磁盘总/已用/可用容量。
type DiskInfo struct {
	Total int64  `json:"total"`
	Used  int64  `json:"used"`
	Free  int64  `json:"free"`
	Label string `json:"label"`
}

// InitResult 是前端加载时的初始状态。
type InitResult struct {
	Home          string   `json:"home"`
	Disk          DiskInfo `json:"disk"`
	Excludes      []string `json:"excludes"`
	TargetPaths   []string `json:"targetPaths"`
	ScannerNames  []string `json:"scannerNames"`
	ConfigPath    string   `json:"configPath"`
	ConfigWarning string   `json:"configWarning"`
	// ScannedAt/HasReport 描述内存中是否已有一次扫描结果：
	// 页面刷新（Cmd+R）后会按此恢复视图，而不是退回「从未扫描」。
	// 跨进程重启的持久化是另一档功能，最新报表返回 nil 时这两项为空值。
	ScannedAt time.Time `json:"scannedAt"`
	HasReport bool      `json:"hasReport"`
}

// Init 返回启动所需的初始状态；扫描范围从配置恢复。
func (s *Service) Init() InitResult {
	total, used, free, ok := statfs(s.ctrl.Home())
	if !ok {
		// statfs 失败极罕见（本机 home 必在本地卷）；
		// 返回零值让前端按“未知”处理，而不是阻塞启动。
		total, used, free = 0, 0, 0
	}
	res := InitResult{}
	res.Home = s.ctrl.Home()
	res.Disk = DiskInfo{Total: total, Used: used, Free: free, Label: volumeLabel(s.ctrl.Home())}
	res.Excludes = s.ctrl.ExcludeRules()
	res.TargetPaths = s.ctrl.TargetPaths()
	res.ScannerNames = s.ctrl.ScannerNames()
	res.ConfigPath = s.ctrl.ConfigPath()
	res.ConfigWarning = s.ctrl.ConfigWarning()
	if latest := s.latestReport(); latest != nil {
		res.HasReport = true
		res.ScannedAt = latest.ScannedAt
	}
	return res
}

// ==================== 扫描 ====================

// ProgressPayload 是 scan:progress 事件的数据。
type ProgressPayload struct {
	Done  int `json:"done"`
	Total int `json:"total"`
}

// ScanDonePayload 是 scan:done 事件的数据。
type ScanDonePayload struct {
	// Canceled 为 true 表示被用户取消；此时 Report 为 nil。
	Canceled bool    `json:"canceled"`
	Report   *Report `json:"report"`
}

// StartScan 启动一次完整扫描（后台执行）。
//
// 进度以扫描器完成数为粒度，经 "scan:progress" 周期推送；
// 结束（含取消）后推送一次 "scan:done"。前端不对扫描并发上锁，
// Service 内部用 mu 保证第二次 StartScan 直接报错。
func (s *Service) StartScan() error {
	if !s.mu.TryLock() {
		return fmt.Errorf("扫描正在进行中")
	}

	g := s.ctrl.NewGate()
	stop := make(chan struct{})
	go emitScanProgress(g, stop)

	go func() {
		defer s.mu.Unlock()
		defer close(stop)

		report := s.ctrl.Scan()
		canceled := g.Canceled()
		if !canceled {
			s.setLatest(report)
		}
		emit("scan:done", ScanDonePayload{Canceled: canceled, Report: s.view(nil)})
	}()
	return nil
}

// emitScanProgress 每 250ms 推送一次门闩进度，直到 stop 关闭。
// 最终进度以 "scan:done" 为准，最后两次推送与事件的窗口期差异可接受。
func emitScanProgress(g *gate.Gate, stop <-chan struct{}) {
	t := time.NewTicker(250 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
			done, total := g.Progress()
			emit("scan:progress", ProgressPayload{Done: done, Total: total})
		}
	}
}

// PauseScan 暂停当前扫描（引擎用门闩逐点挂起）。
func (s *Service) PauseScan() {
	if g := s.ctrl.CurrentGate(); g != nil {
		g.Pause()
	}
}

// ResumeScan 恢复被暂停的扫描。
func (s *Service) ResumeScan() {
	if g := s.ctrl.CurrentGate(); g != nil {
		g.Resume()
	}
}

// CancelScan 取消当前扫描；结束会经 scan:done(canceled=true) 通知前端。
func (s *Service) CancelScan() {
	if g := s.ctrl.CurrentGate(); g != nil {
		g.Cancel()
	}
}

// ==================== 报表视图 ====================

// FileView 是一条可清理项的前端视图。
type FileView struct {
	Key       string `json:"key"`
	N         string `json:"n"` // 展示名
	P         string `json:"p"` // 路径（绝对）
	S         int64  `json:"s"` // 字节数
	Note      string `json:"note"`
	Permanent bool   `json:"permanent"`
	ReadOnly  bool   `json:"readOnly"`
	IsDir     bool   `json:"isDir"`
}

// CatView 是一个分类卡片。
type CatView struct {
	ID    string     `json:"id"`
	Name  string     `json:"name"`
	Color string     `json:"color"`
	Icon  string     `json:"icon"`
	Desc  string     `json:"desc"`
	Safe  bool       `json:"safe"`
	Files []FileView `json:"files"`
}

// Report 是 scan:done 的完整前端视图。
type Report struct {
	ScannedAt     time.Time  `json:"scannedAt"`
	TargetPaths   []string   `json:"targetPaths"`
	Errors        []string   `json:"errors"`
	ExcludedCount int        `json:"excludedCount"`
	ExcludedBytes int64      `json:"excludedBytes"`
	Cats          []CatView  `json:"cats"`
	Large         []FileView `json:"large"`
}

// 给每个真实类别的展示元数据：与原型一致（图标/颜色/警告语气）。
var catMeta = map[model.Category]struct {
	id, name, color, icon, desc string
	safe                        bool
}{
	model.CategoryUserCache:   {"app", "应用缓存", "#0a84ff", "📦", "各 App 的 Library/Caches 目录，重建后无影响", true},
	model.CategorySystemCache: {"log", "系统日志与诊断", "#8e8e93", "📄", "统一日志归档与崩溃报告，删除后自动重建", true},
	model.CategoryDevCache:    {"dev", "开发缓存", "#af52de", "🛠️", "Xcode DerivedData、npm / pip / Gradle 本地缓存", true},
	model.CategoryLargeFile:   {"large", "大文件", "#30b0c7", "🗂️", "超过阈值的单个大文件，需人工确认", false},
	model.CategoryJunk:        {"junk", "垃圾文件", "#ff9f0a", "🗑️", ".DS_Store、失效日志等杂项", false},
	model.CategorySnapshot:    {"snap", "APFS 快照", "#ff453a", "🧊", "Time Machine 本地快照，可能影响系统恢复能力", false},
}

// buildKeyIndex 生成 "类别:序号" → Item 的映射。
// 前端只见过 key；清理时按键取回真实路径，避免传任意路径。
func buildKeyIndex(r *model.Report) map[string]model.Item {
	keys := make(map[string]model.Item)
	i := 0
	for _, it := range r.Items {
		k := fmt.Sprintf("f%d", i)
		keys[k] = it
		i++
	}
	return keys
}

// view 把当前 report 转成前端视图。unusedKeys 非空时把这些 key 从视图
// 剔除（行内排除后立即反映，无需重新扫描）；为 nil 表示全部展示。
func (s *Service) view(skip map[string]bool) *Report {
	src := s.latestReport()
	if src == nil {
		return nil
	}
	r := &Report{
		ScannedAt:     src.ScannedAt,
		TargetPaths:   src.TargetPaths,
		ExcludedCount: src.ExcludedCount,
		ExcludedBytes: src.ExcludedBytes,
	}
	for _, e := range src.Errors {
		if e.Scanner != "" {
			r.Errors = append(r.Errors, fmt.Sprintf("%s: %v", e.Scanner, e.Err))
		} else {
			r.Errors = append(r.Errors, e.Err.Error())
		}
	}

	byCat := map[model.Category][]FileView{}
	for i, it := range src.Items {
		k := fmt.Sprintf("f%d", i)
		if skip != nil && skip[k] {
			continue
		}
		if it.Category == model.CategoryLargeFile {
			r.Large = append(r.Large, s.fileView(k, it))
			continue
		}
		byCat[it.Category] = append(byCat[it.Category], s.fileView(k, it))
	}

	r.Cats = make([]CatView, 0, len(byCat))
	for _, cat := range scanSegmentOrder() {
		files := byCat[cat]
		if len(files) == 0 {
			continue
		}
		meta, ok := catMeta[cat]
		if !ok {
			meta = struct {
				id, name, color, icon, desc string
				safe                        bool
			}{strings.ToLower(string(cat)), string(cat), "#8e8e93", "📦", "", false}
		}
		r.Cats = append(r.Cats, CatView{
			ID: meta.id, Name: meta.name, Color: meta.color,
			Icon: meta.icon, Desc: meta.desc, Safe: meta.safe,
			Files: files,
		})
	}
	return r
}

// scanSegmentOrder 按固定顺序返回全部已知类别，保证界面分组顺序稳定；
// 未知类别（将来新增的扫描器）不会出现在这里，会落到 catMeta 的兜底分支。
func scanSegmentOrder() []model.Category {
	return []model.Category{
		model.CategoryUserCache,
		model.CategorySystemCache,
		model.CategoryDevCache,
		model.CategoryLargeFile,
		model.CategoryJunk,
		model.CategorySnapshot,
	}
}

// fileView 生成单项视图。key 就是索引对应的编号。
func (s *Service) fileView(k string, it model.Item) FileView {
	return FileView{
		Key:       k,
		N:         it.DisplayName,
		P:         it.Path,
		S:         it.Size,
		Note:      it.Note,
		Permanent: it.Permanent,
		ReadOnly:  it.ReadOnly,
		IsDir:     it.IsDir,
	}
}

// GetReport 返回最近一次扫描的视图（刷新页面时用）。
func (s *Service) GetReport() *Report { return s.view(nil) }

// CleanOne 是 clean:one 事件的逐项结果。
type CleanOne struct {
	// Key 是这条进度对应的原始项目 key；前端按它定位行，
	// 与展示列表的顺序无关。
	Key     string `json:"key"`
	Done    int    `json:"done"`
	Total   int    `json:"total"`
	Name    string `json:"name"`
	Size    int64  `json:"size"`
	Dest    string `json:"dest"`
	Err     string `json:"err"`
	Skipped bool   `json:"skipped"`
}

// CleanDonePayload 是 clean:done 事件的数据。
type CleanDonePayload struct {
	Succeeded int   `json:"succeeded"`
	Failed    int   `json:"failed"`
	Freed     int64 `json:"freed"`
}

// keyedItem 把前端 key 和真实 Item 配对，清理时按 key 回报进度。
type keyedItem struct {
	key string
	it  model.Item
}

// selectItems 把前端 key 解析成可清理的项。
//
// 两类项在这里就被挡住，而不是等引擎运行后才失败：
//   - Permanent（废纸篓内容、APFS 快照）不走废纸篓流程，需要 sudo/专用流程
//   - ReadOnly（指定目录扫描结果）仅可浏览，删除仍受 guard 白名单约束
//
// 按大小降序排序，先清大项；进度通过 key 对齐，与前端行序无关。
func (s *Service) selectItems(keys []string) ([]keyedItem, error) {
	pairs := make([]keyedItem, 0, len(keys))
	for _, k := range keys {
		it, ok := s.keyOf(k)
		if !ok {
			return nil, fmt.Errorf("无法识别的项目 key: %s", k)
		}
		if it.Permanent {
			return nil, fmt.Errorf("该项不支持移入废纸篓: %s", it.DisplayName)
		}
		if it.ReadOnly {
			return nil, fmt.Errorf("该项仅可查看，不能清理: %s", it.DisplayName)
		}
		pairs = append(pairs, keyedItem{key: k, it: it})
	}
	sort.SliceStable(pairs, func(i, j int) bool { return pairs[i].it.Size > pairs[j].it.Size })
	return pairs, nil
}

// Clean 把 keys 指到的项目逐个移入废纸篓（后台执行）。
// 每完成一项推送 "clean:one"，全部结束后推送 "clean:done"。
//
// guard 拒绝的项会被标记为 skipped，不会计入失败。
func (s *Service) Clean(keys []string) error {
	pairs, err := s.selectItems(keys)
	if err != nil {
		return err
	}
	items := make([]model.Item, 0, len(pairs))
	for _, p := range pairs {
		items = append(items, p.it)
	}

	go func() {
		total := len(items)
		results := s.ctrl.CleanBatch(items, func(done int, o clean.Outcome) {
			// done 从 1 计数，且每项一次 Run 只产出一个 outcome，
			// 因此 done-1 正好指向 items 里的对应位置（keyedItem 同序）。
			one := CleanOne{
				Key:     pairs[done-1].key,
				Done:    done,
				Total:   total,
				Name:    o.Item.DisplayName,
				Size:    o.Item.Size,
				Dest:    o.Dest,
				Skipped: o.Skipped,
			}
			if o.Err != nil {
				one.Err = o.Err.Error()
			}
			emit("clean:one", one)
		})
		s.viewsMu.Lock()
		s.lastResult = &results
		s.viewsMu.Unlock()
		emit("clean:done", CleanDonePayload{
			Succeeded: results.Succeeded(),
			Failed:    results.Failed(),
			Freed:     results.FreedBytes,
		})
	}()
	return nil
}

// ==================== 撤销 ====================

// RestoreResult 是撤销操作的结果。
type RestoreResult struct {
	Restored int `json:"restored"`
	Failed   int `json:"failed"`
}

// Undo 把上一次清理移入废纸篓的内容放回原位置。
func (s *Service) Undo() RestoreResult {
	s.viewsMu.Lock()
	last := s.lastResult
	s.lastResult = nil
	s.viewsMu.Unlock()
	if last == nil {
		return RestoreResult{}
	}
	restored, failed := s.ctrl.Restore(*last)
	return RestoreResult{Restored: restored, Failed: failed}
}

// ==================== 排除列表 ====================

// AddExclude 追加一条排除规则并持久化。三种写法：
// 绝对路径、目录名、通配符（与 CLI 完全一致）。
func (s *Service) AddExclude(rule string) error {
	return s.ctrl.AddExcludeRule(rule)
}

// RemoveExclude 删除一条排除规则并持久化。
func (s *Service) RemoveExclude(rule string) error {
	return s.ctrl.RemoveExcludeRule(rule)
}

// Excludes 返回当前生效的排除规则列表。
func (s *Service) Excludes() []string { return s.ctrl.ExcludeRules() }

// ExcludeFromFile 把某一项所在的目录加入排除列表，
// 并立刻从当前报表中过滤掉该项（对应原型的“行内排除”）。
// 返回更新后的报表视图，前端无需重新扫描。
func (s *Service) ExcludeFromFile(key string) (*Report, error) {
	it, ok := s.keyOf(key)
	if !ok {
		return nil, fmt.Errorf("无法识别的项目 key: %s", key)
	}
	dir := filepath.Dir(it.Path)
	if err := s.ctrl.AddExcludeRule(dir); err != nil {
		return nil, err
	}
	skip := map[string]bool{key: true}
	return s.view(skip), nil
}

// ==================== 扫描范围 ====================

// SetScope 设置扫描范围。paths 为空 = 全盘；
// 非空 = 指定目录模式（仅可浏览，项目会被标为 ReadOnly）。
// 范围写入配置文件持久化，重开应用不丢失。
func (s *Service) SetScope(paths []string) error {
	s.ctrl.SetTargetPaths(paths)
	return nil
}

// ==================== 磁盘工具 ====================

// statfs 返回包含 path 的卷的总/已用/可用容量（字节）。
func statfs(path string) (total, used, free int64, ok bool) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, 0, 0, false
	}
	bsize := int64(st.Bsize)
	if bsize <= 0 {
		bsize = 4096
	}
	total = int64(st.Blocks) * bsize
	free = int64(st.Bavail) * bsize
	used = total - int64(st.Bfree)*bsize
	if used < 0 {
		used = 0
	}
	if free < 0 {
		free = 0
	}
	return total, used, free, true
}

// volumeLabel 返回包含 path 的卷的展示名。
// APFS 在 Catalina 后把数据卷挂到 /System/Volumes/...，
// 根卷统一显示「Macintosh HD」，其余显示挂载点末段。
func volumeLabel(path string) string {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return "磁盘"
	}
	mnt := int8SliceToString(st.Mntonname[:])
	if mnt == "" || mnt == "/" {
		return "Macintosh HD"
	}
	if base := filepath.Base(mnt); base != "." && base != "/" {
		return base
	}
	return mnt
}

// int8SliceToString 把定长 int8 数组（Statfs_t 的字符串字段）里 NUL 结尾的
// 字符串转为 Go 字符串。Mntonname 在 darwin 上是 []int8。
func int8SliceToString(b []int8) string {
	buf := make([]byte, len(b))
	for i, v := range b {
		buf[i] = byte(v)
	}
	return strings.TrimRight(string(buf), "\x00")
}
