package main

import (
	"log"

	"eyeloveu/internal/reminder"

	"github.com/jwijenbergh/puregotk/v4/adw"
	"github.com/jwijenbergh/puregotk/v4/glib"
	"github.com/jwijenbergh/puregotk/v4/gtk"
)

func showBreakPrompt(app *gtk.Application, closed func()) *adw.ApplicationWindow {
	client, err := reminder.ConnectRunning()
	if err != nil {
		log.Printf("读取休息提醒: %v", err)
		return nil
	}
	state, err := client.Status()
	if err != nil || !breakVisible(state) {
		client.Close()
		return nil
	}

	win := adw.NewApplicationWindow(app)
	win.SetTitle("屏幕休息 · EyeLoveU")
	win.SetDefaultSize(480, 300)
	win.SetResizable(false)
	root := gtk.NewBox(gtk.OrientationVerticalValue, 0)
	header := adw.NewHeaderBar()
	header.SetTitleWidget(&adw.NewWindowTitle("EyeLoveU", "").Widget)
	root.Append(&header.Widget)
	win.SetContent(&root.Widget)

	content := page()
	content.AddCssClass("break-prompt")
	icon := gtk.NewImageFromIconName("image-red-eye-symbolic")
	icon.SetHalign(gtk.AlignStartValue)
	icon.SetPixelSize(32)
	content.Append(&icon.Widget)
	title := label("该休息眼睛了", "dashboard-title")
	content.Append(&title.Widget)
	content.Append(&label("把视线从屏幕移开，看看远处。", "").Widget)
	countdown := label("", "timer-clock")
	content.Append(&countdown.Widget)
	feedback := label("", "dim-label")
	content.Append(&feedback.Widget)
	feedback.SetVisible(false)
	root.Append(&content.Widget)

	var timer uint32
	closePrompt := func() {
		state.EyePending = false
		win.Close()
	}
	runAction := func(action string) bool {
		if err := client.Act(action); err != nil {
			feedback.SetLabel("无法更新休息提醒，请重试。")
			feedback.SetVisible(true)
			return false
		}
		feedback.SetVisible(false)
		return true
	}
	actions := gtk.NewBox(gtk.OrientationHorizontalValue, 10)
	actions.SetHalign(gtk.AlignEndValue)
	actions.SetMarginTop(6)
	later := button("延后 5 分钟", "", func() {
		if runAction("eye-later") {
			closePrompt()
		}
	})
	var primary *gtk.Button
	render := func() {
		resting := state.RestSeconds > 0
		later.SetVisible(!resting)
		countdown.SetVisible(resting)
		if resting {
			title.SetLabel("休息中")
			countdown.SetLabel(clockText(state.RestSeconds))
			primary.SetLabel("结束休息")
		} else {
			title.SetLabel("该休息眼睛了")
			primary.SetLabel("开始休息")
		}
	}
	update := func() bool {
		next, err := client.Status()
		if err != nil || !breakVisible(next) {
			closePrompt()
			return false
		}
		state = next
		render()
		return true
	}
	primary = button("开始休息", "suggested-action", func() {
		if state.RestSeconds > 0 {
			if runAction("finish-rest") {
				closePrompt()
			}
		} else if runAction("rest") {
			update()
		}
	})
	actions.Append(&later.Widget)
	actions.Append(&primary.Widget)
	content.Append(&actions.Widget)
	win.SetDefaultWidget(&primary.Widget)
	render()

	tick := glib.SourceFunc(func(uintptr) bool { return update() })
	timer = glib.TimeoutAdd(500, &tick, 0)
	closeRequest := func(gtk.Window) bool {
		if state.EyePending {
			runAction("eye-later")
		}
		glib.SourceRemove(timer)
		client.Close()
		closed()
		return false
	}
	win.ConnectCloseRequest(&closeRequest)
	return win
}

func breakVisible(state reminder.Status) bool {
	return !state.Away && state.PauseSeconds == 0 && (state.EyePending || state.RestSeconds > 0)
}
