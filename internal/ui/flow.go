package ui

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/kennethfan/disk-cleaner/internal/clean"
	"github.com/kennethfan/disk-cleaner/internal/model"
)

// confirmClean 弹出二次确认弹窗。
//
// 弹窗按原型要求给出三件事：后果说明（移入废纸篓而非彻底删除）、
// 完整清单摘要、绿色信任背书行。确认后才执行清理。
func (s *appShell) confirmClean(items []model.Item, title string) {
	if len(items) == 0 {
		return
	}

	var total int64
	for _, it := range items {
		total += it.Size
	}

	// 清单摘要：最多展示 8 行，超出折叠为省略，完整清单同时体现在进度页里。
	const maxRows = 8
	list := container.NewVBox()
	for i, it := range items {
		if i >= maxRows {
			list.Add(widget.NewLabel(fmt.Sprintf("…… 另有 %d 项", len(items)-maxRows)))
			break
		}
		name := widget.NewLabel(it.DisplayName)
		name.Truncation = fyne.TextTruncateEllipsis
		size := widget.NewLabel(model.HumanBytes(it.Size))
		size.Alignment = fyne.TextAlignTrailing
		list.Add(container.NewBorder(nil, nil, nil, size, name))
	}

	sub := widget.NewLabel(fmt.Sprintf("共 %d 个项目 · 合计 %s。将移入废纸篓（非彻底删除），可随时恢复。",
		len(items), model.HumanBytes(total)))
	sub.Wrapping = fyne.TextWrapWord

	trust := makeText("🗑 移入废纸篓 · 30 天内可恢复 · 不会立即释放磁盘", 11.5, colOK, true, fyne.TextAlignLeading)

	content := container.NewVBox(sub, trust, widget.NewSeparator(),
		container.NewVScroll(list))

	d := dialog.NewCustomConfirm(title, "移入废纸篓", "取消", content, func(ok bool) {
		if ok {
			s.beginClean(items)
		}
	}, s.win)
	d.Show()
}

// beginClean 进入清理进度流程态并逐项执行清理。
func (s *appShell) beginClean(items []model.Item) {
	if len(items) == 0 || s.scanning {
		return
	}

	s.cleaningV.reset(items)
	s.switchView(viewCleaning)
	s.updateHead()

	go func() {
		// onOne 在工作 goroutine 中回调，更新界面必须回到主线程。
		res := s.ctrl.CleanBatch(items, func(done int, o clean.Outcome) {
			fyne.Do(func() { s.cleaningV.mark(done-1, o) })
		})
		fyne.Do(func() { s.finishClean(res) })
	}()
}

// finishClean 清理完成后的 UI 线程收尾：进入完成态并同步各页数据。
func (s *appShell) finishClean(res CleanResult) {
	s.lastClean = res
	s.filterReportSucceeded(res)
	s.refreshAllViews()
	s.doneV.show(res)
	s.switchView(viewDone)
	s.updateHead()

	if res.Failed() > 0 {
		s.toast(fmt.Sprintf("清理完成：%d 项入废纸篓 · %d 项失败未改动", res.Succeeded(), res.Failed()))
	}
}

// filterReportSucceeded 从当前报告中移除已成功清理的项，让各页与磁盘状态一致。
func (s *appShell) filterReportSucceeded(res CleanResult) {
	if s.report == nil {
		return
	}
	removed := make(map[string]bool)
	for _, o := range res.Outcomes {
		if o.Err == nil && !o.Skipped {
			removed[o.Item.Path] = true
		}
	}
	kept := s.report.Items[:0:0]
	var total int64
	for _, it := range s.report.Items {
		if removed[it.Path] {
			continue
		}
		kept = append(kept, it)
		total += it.Size
	}
	s.report.Items = kept
	s.report.TotalBytes = total
}

// undoClean 把本批清理移入废纸篓的内容放回原位，随后重新扫描还原界面。
func (s *appShell) undoClean() {
	if len(s.lastClean.Outcomes) == 0 {
		return
	}
	s.undoBtnLocked(true)
	go func() {
		restored, failed := s.ctrl.Restore(s.lastClean)
		fyne.Do(func() {
			s.lastClean = CleanResult{}
			s.undoBtnLocked(false)
			if failed > 0 {
				s.toast(fmt.Sprintf("已恢复 %d 项 · %d 项恢复失败", restored, failed))
			} else {
				s.toast("已全部放回原位置")
			}
			// 重新扫描，让结果页与磁盘真实状态一致。
			s.startScan()
		})
	}()
}

// undoBtnLocked 控制「撤销」按钮的可用性，避免重复点击。
func (s *appShell) undoBtnLocked(lock bool) {
	if lock {
		s.doneV.undoBtn.Disable()
	} else {
		s.doneV.undoBtn.Enable()
	}
}

// backToOverviewFromDone 完成页的「查看最新概览」：回概览并重新扫描，
// 让磁盘数字与各页数据反映清理后的状态。
func (s *appShell) backToOverviewFromDone() {
	s.switchView(viewOverview)
	s.startScan()
}
