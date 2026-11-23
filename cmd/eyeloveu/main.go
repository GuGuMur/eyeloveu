package main

import (
	"fmt"
	"os"
	"runtime"

	"eyeloveu/internal/reminder"

	"github.com/jwijenbergh/puregotk/v4/adw"
	"github.com/jwijenbergh/puregotk/v4/gio"
	"github.com/jwijenbergh/puregotk/v4/gtk"
)

func main() {
	runtime.LockOSThread()
	app := adw.NewApplication(reminder.AppID, gio.GApplicationFlagsNoneValue)
	if app == nil {
		fmt.Fprintln(os.Stderr, "需要 GTK 4 和 libadwaita")
		os.Exit(1)
	}
	activate := func(gio.Application) {
		if active := app.GetActiveWindow(); active != nil && active.Ptr != 0 {
			active.Present()
			return
		}
		showWindow(&app.Application)
	}
	app.ConnectActivate(&activate)
	os.Exit(int(app.Run(1, []string{os.Args[0]})))
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
