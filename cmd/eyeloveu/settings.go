package main

import (
	"eyeloveu/internal/reminder"

	"github.com/jwijenbergh/puregotk/v4/adw"
	"github.com/jwijenbergh/puregotk/v4/gtk"
)

type settingsForm struct {
	box                                    *gtk.Box
	save                                   *gtk.Button
	eye, water, smart, login               *gtk.Switch
	eyeMinutes, waterMinutes, breakSeconds *gtk.SpinButton
	smartRow                               *adw.ActionRow
}

func (d *dashboard) settings() *settingsForm {
	form := &settingsForm{box: page()}
	form.box.Append(&label("提醒设置", "dashboard-title").Widget)

	reminders := adw.NewPreferencesGroup()
	reminders.SetTitle("提醒频率")
	form.eye, _ = switchRow(reminders, "屏幕休息", "到点弹出休息窗口并发送通知")
	form.eyeMinutes = numberRow(reminders, "屏幕休息间隔（分钟）", 1, 180)
	form.breakSeconds = numberRow(reminders, "休息时长（秒）", 20, 300)
	form.water, _ = switchRow(reminders, "喝水提醒", "")
	form.waterMinutes = numberRow(reminders, "喝水间隔（分钟）", 1, 240)
	form.box.Append(&reminders.Widget)

	behavior := adw.NewPreferencesGroup()
	behavior.SetTitle("后台行为")
	form.smart, form.smartRow = switchRow(behavior, "无操作时延后喝水", "长时间没有键鼠操作时，等你回来再提醒喝水")
	form.login, _ = switchRow(behavior, "登录时启动", "在后台运行，打开窗口可调整提醒")
	form.box.Append(&behavior.Widget)

	form.save = button("保存设置", "suggested-action", func() {
		if err := d.client.Save(form.config()); err != nil {
			d.toast("保存设置失败 · " + err.Error())
			return
		}
		if err := d.client.SetAutostart(form.login.GetActive()); err != nil {
			d.toast("提醒设置已保存，登录启动设置失败 · " + err.Error())
			return
		}
		d.toast("设置已保存")
		d.refresh()
	})
	preview := button("发送测试通知", "", func() { d.act("preview") })
	preview.SetHalign(gtk.AlignStartValue)
	form.box.Append(&preview.Widget)

	quit := button("停止后台并退出", "flat", func() {
		if err := d.client.Stop(); err != nil {
			d.toast(err.Error())
			return
		}
		d.win.Close()
	})
	quit.SetHalign(gtk.AlignStartValue)
	form.box.Append(&quit.Widget)
	return form
}

func (f *settingsForm) config() reminder.Config {
	for _, field := range []*gtk.SpinButton{f.eyeMinutes, f.waterMinutes, f.breakSeconds} {
		field.Update()
	}
	return reminder.Config{
		EyeEnabled: f.eye.GetActive(), WaterEnabled: f.water.GetActive(),
		EyeMinutes: int(f.eyeMinutes.GetValueAsInt()), WaterMinutes: int(f.waterMinutes.GetValueAsInt()),
		BreakSeconds: int(f.breakSeconds.GetValueAsInt()), Smart: f.smart.GetActive(),
	}
}

func (f *settingsForm) load(state reminder.Status) {
	f.eye.SetActive(state.Config.EyeEnabled)
	f.water.SetActive(state.Config.WaterEnabled)
	f.eyeMinutes.SetValue(float64(state.Config.EyeMinutes))
	f.waterMinutes.SetValue(float64(state.Config.WaterMinutes))
	f.breakSeconds.SetValue(float64(state.Config.BreakSeconds))
	f.smart.SetActive(state.Config.Smart)
	f.smart.SetSensitive(state.SmartAvailable)
	if !state.SmartAvailable {
		f.smartRow.SetSubtitle("当前桌面未提供空闲检测，喝水按固定间隔提醒")
	}
	f.login.SetActive(state.Autostart)
}

func controlRow(group *adw.PreferencesGroup, title, subtitle string, control *gtk.Widget) *adw.ActionRow {
	row := adw.NewActionRow()
	row.SetTitle(title)
	row.SetSubtitle(subtitle)
	control.SetValign(gtk.AlignCenterValue)
	row.AddSuffix(control)
	row.SetActivatableWidget(control)
	group.Add(&row.Widget)
	return row
}

func switchRow(group *adw.PreferencesGroup, title, subtitle string) (*gtk.Switch, *adw.ActionRow) {
	control := gtk.NewSwitch()
	return control, controlRow(group, title, subtitle, &control.Widget)
}

func numberRow(group *adw.PreferencesGroup, title string, min, max float64) *gtk.SpinButton {
	control := gtk.NewSpinButtonWithRange(min, max, 1)
	control.SetNumeric(true)
	controlRow(group, title, "", &control.Widget)
	return control
}
