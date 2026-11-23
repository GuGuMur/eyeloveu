package reminder

import (
	_ "embed"
	"log"

	"fyne.io/systray"
)

//go:embed tray.png
var trayIcon []byte

func startTray(done <-chan struct{}) func() {
	go systray.Run(func() {
		systray.SetTitle("EyeLoveU")
		systray.SetIcon(trayIcon)
		systray.SetOnTapped(func() { openTrayWindow() })
		home := systray.AddMenuItem("打开 EyeLoveU", "")
		settings := systray.AddMenuItem("设置", "")
		go func() {
			for {
				select {
				case <-home.ClickedCh:
					openTrayWindow()
				case <-settings.ClickedCh:
					openTrayWindow("--settings")
				case <-done:
					return
				}
			}
		}()
	}, nil)
	return systray.Quit
}

func openTrayWindow(args ...string) {
	if err := launch("eyeloveu", args...); err != nil {
		log.Printf("打开 EyeLoveU: %v", err)
	}
}
