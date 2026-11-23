package main

import (
	_ "embed"
	"fmt"
	"html"

	"eyeloveu/internal/reminder"

	"github.com/jwijenbergh/puregotk/v4/adw"
	"github.com/jwijenbergh/puregotk/v4/gdk"
	"github.com/jwijenbergh/puregotk/v4/glib"
	"github.com/jwijenbergh/puregotk/v4/gtk"
)

//go:embed style.css
var stylesheet string

type dashboard struct {
	client            *reminder.Client
	win               *adw.ApplicationWindow
	overlay           *adw.ToastOverlay
	status, restClock *gtk.Label
	eye, water        timerCard
	restPanel         *gtk.Box
	restNow, pause    *gtk.Button
	clientActions     []*gtk.Widget
}

func showWindow(app *gtk.Application) {
	win := adw.NewApplicationWindow(app)
	win.SetTitle("EyeLoveU")
	win.SetDefaultSize(610, 510)
	provider := gtk.NewCssProvider()
	provider.LoadFromData(stylesheet, len(stylesheet))
	gtk.StyleContextAddProviderForDisplay(gdk.DisplayGetDefault(), provider, 600)

	d := &dashboard{win: win, overlay: adw.NewToastOverlay()}
	root := gtk.NewBox(gtk.OrientationVerticalValue, 0)
	header := adw.NewHeaderBar()
	header.SetTitleWidget(&adw.NewWindowTitle("EyeLoveU", "").Widget)
	root.Append(&header.Widget)
	root.Append(&d.overlay.Widget)
	d.overlay.SetVexpand(true)
	win.SetContent(&root.Widget)

	d.status = label("正在连接提醒后台…", "dim-label")
	home := page()
	heading := label("下一次提醒", "dashboard-title")
	home.Append(&heading.Widget)
	home.Append(&d.status.Widget)

	cards := gtk.NewBox(gtk.OrientationHorizontalValue, 12)
	cards.SetHomogeneous(true)
	d.eye = newTimerCard("屏幕休息", "image-red-eye-symbolic", "eye-card")
	d.water = newTimerCard("喝水", "weather-showers-symbolic", "water-card")
	cards.Append(&d.eye.box.Widget)
	cards.Append(&d.water.box.Widget)
	cards.SetMarginTop(6)
	home.Append(&cards.Widget)

	d.restPanel = gtk.NewBox(gtk.OrientationHorizontalValue, 12)
	d.restPanel.AddCssClass("rest-panel")
	d.restPanel.SetVisible(false)
	restInfo := gtk.NewBox(gtk.OrientationVerticalValue, 4)
	restInfo.SetHexpand(true)
	restInfo.Append(&label("把视线移到远处", "heading").Widget)
	d.restClock = label("", "rest-clock")
	restInfo.Append(&d.restClock.Widget)
	d.restPanel.Append(&restInfo.Widget)
	finish := button("结束休息", "", func() { d.act("finish-rest") })
	finish.SetValign(gtk.AlignCenterValue)
	d.restPanel.Append(&finish.Widget)
	home.Append(&d.restPanel.Widget)

	actions := gtk.NewBox(gtk.OrientationHorizontalValue, 10)
	actions.SetMarginTop(8)
	d.restNow = button("现在休息", "suggested-action", func() { d.act("rest") })
	d.pause = button("暂停 1 小时", "", func() {
		state, err := d.client.Status()
		if err != nil {
			d.toast(err.Error())
			return
		}
		if state.PauseSeconds > 0 {
			d.act("resume")
		} else {
			d.act("pause")
		}
	})
	actions.Append(&d.restNow.Widget)
	actions.Append(&d.pause.Widget)
	home.Append(&actions.Widget)

	stack := gtk.NewStack()
	stack.SetTransitionType(gtk.StackTransitionTypeCrossfadeValue)
	stack.AddNamed(scroll(home), "home")
	d.overlay.SetChild(&stack.Widget)
	settingsToggle := gtk.NewButtonFromIconName("emblem-system-symbolic")
	settingsToggle.AddCssClass("flat")
	settingsToggle.SetTooltipText("提醒设置")
	header.PackStart(&settingsToggle.Widget)
	d.clientActions = []*gtk.Widget{&actions.Widget, &settingsToggle.Widget}
	d.setConnected(false)

	connecting := true
	retry := button("重新连接", "flat", func() {
		if d.client == nil {
			connecting = true
		}
	})
	retry.SetVisible(false)
	retry.SetHalign(gtk.AlignStartValue)
	home.Append(&retry.Widget)
	var settings *settingsForm
	showHome := func() {
		stack.SetVisibleChildName("home")
		settingsToggle.SetIconName("emblem-system-symbolic")
		settingsToggle.SetTooltipText("提醒设置")
		settings.save.SetVisible(false)
	}
	tab := func(gtk.Button) {
		if stack.GetVisibleChildName() == "settings" {
			showHome()
			return
		}
		state, err := d.client.Status()
		if err != nil {
			d.toast(err.Error())
			return
		}
		settings.load(state)
		stack.SetVisibleChildName("settings")
		settingsToggle.SetIconName("go-previous-symbolic")
		settingsToggle.SetTooltipText("返回")
		settings.save.SetVisible(true)
	}
	settingsToggle.ConnectClicked(&tab)
	win.Present()

	tick := glib.SourceFunc(func(uintptr) bool {
		if connecting {
			connecting = false
			client, err := reminder.Connect()
			if err != nil {
				d.status.SetLabel("提醒后台未连接")
				d.toast(err.Error())
				retry.SetVisible(true)
				return true
			}
			d.client = client
			d.setConnected(true)
			retry.SetVisible(false)
			if settings == nil {
				settings = d.settings()
				stack.AddNamed(scroll(settings.box), "settings")
				settings.save.SetVisible(false)
				header.PackEnd(&settings.save.Widget)
			}
		}
		if d.client != nil && !d.refresh() {
			d.client.Close()
			d.client = nil
			showHome()
			retry.SetVisible(true)
		}
		return true
	})
	timer := glib.TimeoutAdd(500, &tick, 0)
	closeRequest := func(gtk.Window) bool {
		glib.SourceRemove(timer)
		if d.client != nil {
			d.client.Close()
		}
		return false
	}
	win.ConnectCloseRequest(&closeRequest)
}

func page() *gtk.Box {
	box := gtk.NewBox(gtk.OrientationVerticalValue, 12)
	box.SetMarginTop(25)
	box.SetMarginBottom(25)
	box.SetMarginStart(24)
	box.SetMarginEnd(24)
	return box
}

func scroll(content *gtk.Box) *gtk.Widget {
	clamp := adw.NewClamp()
	clamp.SetMaximumSize(720)
	clamp.SetChild(&content.Widget)
	window := gtk.NewScrolledWindow()
	window.SetPolicy(gtk.PolicyNeverValue, gtk.PolicyAutomaticValue)
	window.SetChild(&clamp.Widget)
	return &window.Widget
}

type timerCard struct {
	box       *gtk.Box
	time, sub *gtk.Label
	progress  *gtk.ProgressBar
}

func newTimerCard(title, icon, style string) timerCard {
	box := gtk.NewBox(gtk.OrientationVerticalValue, 7)
	box.AddCssClass("timer-card")
	box.AddCssClass(style)
	image := gtk.NewImageFromIconName(icon)
	image.SetPixelSize(22)
	image.SetHalign(gtk.AlignStartValue)
	box.Append(&image.Widget)
	box.Append(&label(title, "heading").Widget)
	clock := label("—", "timer-clock")
	box.Append(&clock.Widget)
	sub := label("下次提醒", "dim-label")
	box.Append(&sub.Widget)
	progress := gtk.NewProgressBar()
	progress.SetMarginTop(7)
	box.Append(&progress.Widget)
	return timerCard{box: box, time: clock, sub: sub, progress: progress}
}

func (c timerCard) render(enabled bool, seconds, interval int, status string) {
	if !enabled {
		c.time.SetLabel("—")
		c.sub.SetLabel("已关闭")
		c.progress.SetFraction(0)
		return
	}
	c.time.SetLabel(clockText(seconds))
	c.sub.SetLabel(status)
	c.progress.SetFraction(1 - min(1, float64(seconds)/float64(interval)))
}

func clockText(seconds int) string {
	if seconds >= 3600 {
		return fmt.Sprintf("%d:%02d:%02d", seconds/3600, seconds/60%60, seconds%60)
	}
	return fmt.Sprintf("%02d:%02d", seconds/60, seconds%60)
}

func (d *dashboard) setConnected(connected bool) {
	for _, action := range d.clientActions {
		action.SetSensitive(connected)
	}
}

func (d *dashboard) refresh() bool {
	state, err := d.client.Status()
	if err != nil {
		d.status.SetLabel("提醒后台已断开")
		d.setConnected(false)
		d.eye.time.SetLabel("—")
		d.water.time.SetLabel("—")
		d.restPanel.SetVisible(false)
		return false
	}
	d.setConnected(true)
	eyeStatus, waterStatus := "下次提醒", "下次提醒"
	switch {
	case state.PauseSeconds > 0:
		d.status.SetLabel("已暂停 · " + clockText(state.PauseSeconds) + " 后恢复")
		eyeStatus, waterStatus = "提醒已暂停", "提醒已暂停"
		d.pause.SetLabel("恢复提醒")
	case state.Away:
		d.status.SetLabel("离开期间暂停提醒")
		eyeStatus, waterStatus = "计时暂停", "回来后提醒"
		d.pause.SetLabel("暂停 1 小时")
	default:
		d.pause.SetLabel("暂停 1 小时")
		if state.Config.Smart && state.SmartAvailable {
			d.status.SetLabel("提醒运行中 · 按屏幕活动计时")
		} else {
			d.status.SetLabel("提醒运行中 · 固定间隔")
		}
	}
	if !state.Config.EyeEnabled && !state.Config.WaterEnabled && state.PauseSeconds == 0 {
		d.status.SetLabel("两项提醒均已关闭")
	}
	d.eye.render(state.Config.EyeEnabled, state.EyeSeconds, state.Config.EyeMinutes*60, eyeStatus)
	d.water.render(state.Config.WaterEnabled, state.WaterSeconds, state.Config.WaterMinutes*60, waterStatus)
	d.restPanel.SetVisible(state.RestSeconds > 0)
	d.restNow.SetSensitive(state.RestSeconds == 0)
	d.restClock.SetLabel(clockText(state.RestSeconds) + " 后结束")
	return true
}

func (d *dashboard) toast(message string) {
	d.overlay.AddToast(adw.NewToast(html.EscapeString(message)))
}

func (d *dashboard) act(action string) {
	if err := d.client.Act(action); err != nil {
		d.toast(err.Error())
		return
	}
	d.refresh()
}
