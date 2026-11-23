package main

import (
	"fmt"
	"os"
	"runtime"

	"eyeloveu/internal/reminder"

	"github.com/jwijenbergh/puregotk/v4/adw"
	"github.com/jwijenbergh/puregotk/v4/gdk"
	"github.com/jwijenbergh/puregotk/v4/gio"
	"github.com/jwijenbergh/puregotk/v4/glib"
	"github.com/jwijenbergh/puregotk/v4/gtk"
)

func main() {
	runtime.LockOSThread()
	app := adw.NewApplication(reminder.AppID, gio.GApplicationHandlesCommandLineValue)
	if app == nil {
		fmt.Fprintln(os.Stderr, "需要 GTK 4 和 libadwaita")
		os.Exit(1)
	}
	app.AddMainOption("settings", 0, glib.GOptionFlagNoneValue, glib.GOptionArgNoneValue, "打开提醒设置", "")
	app.AddMainOption("break-reminder", 0, glib.GOptionFlagNoneValue, glib.GOptionArgNoneValue, "打开休息提醒窗口", "")
	startup := func(gio.Application) {
		provider := gtk.NewCssProvider()
		provider.LoadFromData(stylesheet, len(stylesheet))
		gtk.StyleContextAddProviderForDisplay(gdk.DisplayGetDefault(), provider, 600)
	}
	app.ConnectStartup(&startup)
	var home, prompt *adw.ApplicationWindow
	var openSettings func()
	present := func(gio.Application) {
		if home == nil {
			home, openSettings = showWindow(&app.Application, func() { home = nil })
		}
		home.Present()
	}
	commandLine := func(application gio.Application, pointer uintptr) int32 {
		var command gio.ApplicationCommandLine
		command.SetGoPointer(pointer)
		options := command.GetOptionsDict()
		if options.Contains("break-reminder") {
			if prompt == nil {
				prompt = showBreakPrompt(&app.Application, func() { prompt = nil })
			}
			if prompt != nil {
				prompt.Present()
			}
			return 0
		}
		present(application)
		if options.Contains("settings") {
			openSettings()
		}
		return 0
	}
	app.ConnectActivate(&present)
	app.ConnectCommandLine(&commandLine)
	os.Exit(int(app.Run(int32(len(os.Args)), os.Args)))
}

func button(title, style string, clicked func()) *gtk.Button {
	widget := gtk.NewButtonWithLabel(title)
	if style != "" {
		widget.AddCssClass(style)
	}
	handler := func(gtk.Button) { clicked() }
	widget.ConnectClicked(&handler)
	return widget
}

func label(text, style string) *gtk.Label {
	widget := gtk.NewLabel(text)
	widget.SetXalign(0)
	widget.SetWrap(true)
	if style != "" {
		widget.AddCssClass(style)
	}
	return widget
}
