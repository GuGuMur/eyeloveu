package reminder

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"time"

	"github.com/godbus/dbus/v5"
)

const (
	idleName      = "org.gnome.Mutter.IdleMonitor"
	idlePath      = "/org/gnome/Mutter/IdleMonitor/Core"
	screenName    = "org.gnome.ScreenSaver"
	screenPath    = "/org/gnome/ScreenSaver"
	notifyName    = "org.freedesktop.Notifications"
	notifyPath    = "/org/freedesktop/Notifications"
	loginName     = "org.freedesktop.login1"
	loginManager  = loginName + ".Manager"
	loginPath     = "/org/freedesktop/login1"
	idleThreshold = 2 * time.Minute
)

type request struct {
	apply  func() error
	result chan error
}

type daemon struct {
	bus        *dbus.Conn
	configFile string
	schedule   schedule
	requests   chan request
	done       chan struct{}
	running    bool

	idleWatch, activeWatch uint32
	idle, locked, sleeping bool
	notifications          map[uint32]due
}

func Run(ctx context.Context) error {
	bus, err := dbus.ConnectSessionBus()
	if err != nil {
		return err
	}
	defer bus.Close()
	path, err := configPath()
	if err != nil {
		return err
	}
	config, err := loadConfig(path)
	if err != nil {
		log.Printf("读取设置: %v，使用默认值", err)
		config = DefaultConfig()
	}
	d := &daemon{
		bus: bus, configFile: path, schedule: newSchedule(config, time.Now()),
		requests: make(chan request), done: make(chan struct{}), running: true,
		notifications: make(map[uint32]due),
	}
	defer close(d.done)
	if err := bus.Export(&endpoint{daemon: d}, objectPath, BusName); err != nil {
		return err
	}
	reply, err := bus.RequestName(BusName, dbus.NameFlagDoNotQueue)
	if err != nil || reply != dbus.RequestNameReplyPrimaryOwner {
		return err
	}

	signals := make(chan *dbus.Signal, 32)
	bus.Signal(signals)
	defer bus.RemoveSignal(signals)
	for _, service := range []struct{ name, path string }{
		{screenName, screenPath}, {idleName, idlePath}, {notifyName, notifyPath},
	} {
		if err := watchSignals(bus, service.name, service.name, service.path); err != nil {
			return err
		}
	}
	system, err := dbus.ConnectSystemBus()
	if err == nil {
		defer system.Close()
		system.Signal(signals)
		defer system.RemoveSignal(signals)
		if err := watchSignals(system, loginName, loginManager, loginPath); err != nil {
			log.Printf("订阅休眠事件: %v", err)
		}
	}
	d.refreshPresence()
	stopTray := startTray(d.done)
	defer stopTray()
	d.loop(ctx, signals)
	d.closeNotifications()
	return nil
}

func watchSignals(bus *dbus.Conn, sender, iface, path string) error {
	return bus.AddMatchSignal(dbus.WithMatchSender(sender), dbus.WithMatchInterface(iface), dbus.WithMatchObjectPath(dbus.ObjectPath(path)))
}

func dbusCall(bus *dbus.Conn, name string, path dbus.ObjectPath, method string, args ...any) *dbus.Call {
	ctx, cancel := context.WithTimeout(context.Background(), 700*time.Millisecond)
	defer cancel()
	return bus.Object(name, path).CallWithContext(ctx, method, dbus.FlagNoAutoStart, args...)
}

func (d *daemon) loop(ctx context.Context, signals <-chan *dbus.Signal) {
	timer := time.NewTimer(time.Hour)
	timer.Stop()
	defer timer.Stop()
	for d.running {
		now := time.Now()
		if due := d.schedule.takeDue(now); due.eye || due.water {
			if err := d.notify(due, false); err != nil {
				log.Printf("发送提醒: %v", err)
			}
		}
		var alarm <-chan time.Time
		if next := d.schedule.next(now); !next.IsZero() {
			timer.Reset(time.Until(next))
			alarm = timer.C
		}
		select {
		case <-ctx.Done():
			return
		case <-d.bus.Context().Done():
			d.running = false
		case <-alarm:
		case signal := <-signals:
			if signal != nil {
				d.handleSignal(signal)
			}
		case req := <-d.requests:
			req.result <- req.apply()
		}
		timer.Stop()
	}
}

func (d *daemon) refreshPresence() {
	_ = dbusCall(d.bus, screenName, screenPath, screenName+".GetActive").Store(&d.locked)
	if d.idleWatch == 0 {
		_ = dbusCall(d.bus, idleName, idlePath, idleName+".AddIdleWatch", uint64(idleThreshold.Milliseconds())).Store(&d.idleWatch)
	}
	var idleMS uint64
	if d.idleWatch != 0 && dbusCall(d.bus, idleName, idlePath, idleName+".GetIdletime").Store(&idleMS) == nil {
		d.setIdle(idleMS >= uint64(idleThreshold.Milliseconds()))
	}
	d.updatePresence()
}

func (d *daemon) setIdle(idle bool) {
	d.idle = idle
	if idle && d.activeWatch == 0 {
		_ = dbusCall(d.bus, idleName, idlePath, idleName+".AddUserActiveWatch").Store(&d.activeWatch)
	}
}

func (d *daemon) updatePresence() {
	now := time.Now()
	if d.schedule.config.Smart && d.idle && d.schedule.awaySince.IsZero() {
		d.schedule.markIdle(now, idleThreshold)
	}
	d.schedule.setAway(d.locked || d.sleeping || d.schedule.config.Smart && d.idle, now)
}

func (d *daemon) handleSignal(signal *dbus.Signal) {
	switch signal.Name {
	case idleName + ".WatchFired":
		var id uint32
		if dbus.Store(signal.Body, &id) == nil {
			switch id {
			case d.activeWatch:
				d.activeWatch = 0
				d.setIdle(false)
			case d.idleWatch:
				d.setIdle(true)
			}
			d.updatePresence()
		}
	case screenName + ".ActiveChanged":
		if dbus.Store(signal.Body, &d.locked) == nil {
			d.updatePresence()
		}
	case loginManager + ".PrepareForSleep":
		if dbus.Store(signal.Body, &d.sleeping) == nil {
			if !d.sleeping {
				now := time.Now()
				// Suspend counts as a break even if the monotonic clock stopped.
				d.schedule.awaySince = now.Add(-d.schedule.config.breakLength())
				d.refreshPresence()
			} else {
				d.updatePresence()
			}
		}
	case notifyName + ".ActionInvoked":
		var id uint32
		var action string
		if dbus.Store(signal.Body, &id, &action) == nil {
			d.handleNotificationAction(id, action)
		}
	case notifyName + ".NotificationClosed":
		var id, reason uint32
		if dbus.Store(signal.Body, &id, &reason) == nil {
			delete(d.notifications, id)
		}
	}
}

func (d *daemon) notify(item due, preview bool) error {
	var title, body string
	actions := []string{"default", "打开 EyeLoveU"}
	switch {
	case item.eye && item.water:
		title, body = "休息一下，也喝口水", "看看远处，放松眼睛，顺便喝点水。"
	case item.eye:
		title, body = "该让眼睛休息一下了", "把视线从屏幕移开，看看远处。"
	case item.water:
		title, body = "该喝口水了", "放下手头的工作，喝几口水。"
	}
	if item.eye {
		actions = append(actions, "rest", "开始休息", "eye-later", "休息延后")
	}
	if item.water {
		if !item.eye {
			actions = append(actions, "water-done", "已喝水")
		}
		actions = append(actions, "water-later", "喝水延后")
	}
	if preview {
		title = "测试通知 · " + title
		actions = actions[:2]
		item = due{}
	}
	var replacement uint32
	for id, outstanding := range d.notifications {
		if outstanding == item {
			replacement = id
			break
		}
	}
	hints := map[string]dbus.Variant{
		"desktop-entry": dbus.MakeVariant(AppID),
		"urgency":       dbus.MakeVariant(byte(1)),
	}
	var id uint32
	err := dbusCall(d.bus, notifyName, notifyPath, notifyName+".Notify",
		"EyeLoveU", replacement, "preferences-desktop-display-symbolic", title, body, actions, hints, int32(-1)).Store(&id)
	if err == nil {
		delete(d.notifications, replacement)
		d.notifications[id] = item
	}
	return err
}

func (d *daemon) withdraw(item due) {
	for id, current := range d.notifications {
		if item.eye && current.eye || item.water && current.water {
			d.closeNotification(id)
		}
	}
}

func (d *daemon) closeNotification(id uint32) {
	d.bus.Object(notifyName, notifyPath).Call(notifyName+".CloseNotification", dbus.FlagNoReplyExpected|dbus.FlagNoAutoStart, id)
	delete(d.notifications, id)
}

func (d *daemon) closeNotifications() {
	for id := range d.notifications {
		d.closeNotification(id)
	}
}

func (d *daemon) handleNotificationAction(id uint32, action string) {
	item, exists := d.notifications[id]
	if !exists {
		return
	}
	if action == "default" {
		if err := launch("eyeloveu"); err != nil {
			log.Print(err)
		}
		return
	}
	if item.eye && (action == "rest" || action == "eye-later") ||
		item.water && (action == "water-done" || action == "water-later") {
		if err := d.act(action); err != nil {
			log.Print(err)
		} else if action == "rest" {
			if err := launch("eyeloveu"); err != nil {
				log.Print(err)
			}
		}
	}
}

func (d *daemon) act(action string) error {
	now := time.Now()
	switch action {
	case "rest":
		d.schedule.startRest(now)
		d.withdraw(due{eye: true})
	case "finish-rest":
		d.schedule.finishRest(now)
	case "eye-later":
		if !d.schedule.restUntil.After(now) {
			d.schedule.eyeAt = now.Add(5 * time.Minute)
		}
		d.withdraw(due{eye: true})
	case "water-later":
		d.schedule.waterAt = now.Add(5 * time.Minute)
		d.withdraw(due{water: true})
	case "water-done":
		d.schedule.waterAt = now.Add(d.schedule.config.waterInterval())
		d.withdraw(due{water: true})
	case "pause":
		d.schedule.pauseUntil = now.Add(time.Hour)
		d.closeNotifications()
	case "resume":
		d.schedule.resume(now)
	case "preview":
		return d.notify(due{eye: true, water: true}, true)
	default:
		return errors.New("未知操作")
	}
	return nil
}

type endpoint struct{ daemon *daemon }

func (e *endpoint) dispatch(apply func() error) *dbus.Error {
	result := make(chan error, 1)
	select {
	case e.daemon.requests <- request{apply: apply, result: result}:
	case <-e.daemon.done:
		return dbus.MakeFailedError(errors.New("提醒后台正在退出"))
	}
	if err := <-result; err != nil {
		return dbus.MakeFailedError(err)
	}
	return nil
}

func (e *endpoint) Status() (string, *dbus.Error) {
	var payload []byte
	dbusErr := e.dispatch(func() error {
		status := e.daemon.schedule.snapshot(time.Now())
		status.SmartAvailable = e.daemon.idleWatch != 0
		status.Autostart = autostartEnabled()
		var err error
		payload, err = json.Marshal(status)
		return err
	})
	return string(payload), dbusErr
}

func (e *endpoint) Save(payload string) *dbus.Error {
	var config Config
	if err := json.Unmarshal([]byte(payload), &config); err != nil {
		return dbus.MakeFailedError(err)
	}
	if err := config.Validate(); err != nil {
		return dbus.MakeFailedError(err)
	}
	return e.dispatch(func() error {
		if err := atomicWrite(e.daemon.configFile, []byte(payload)); err != nil {
			return err
		}
		e.daemon.schedule.configure(config, time.Now())
		e.daemon.updatePresence()
		if !config.EyeEnabled || !config.WaterEnabled {
			e.daemon.withdraw(due{eye: !config.EyeEnabled, water: !config.WaterEnabled})
		}
		return nil
	})
}

func (e *endpoint) Act(action string) *dbus.Error {
	return e.dispatch(func() error { return e.daemon.act(action) })
}

func (e *endpoint) Quit() *dbus.Error {
	return e.dispatch(func() error {
		e.daemon.running = false
		return nil
	})
}

func (e *endpoint) SetAutostart(enabled bool) *dbus.Error {
	return e.dispatch(func() error { return setAutostart(enabled) })
}
