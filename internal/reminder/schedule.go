package reminder

import (
	"errors"
	"time"
)

const (
	AppID   = "local.eyeloveu.EyeLoveU"
	BusName = AppID + ".Daemon"
)

type Config struct {
	EyeEnabled   bool
	WaterEnabled bool
	EyeMinutes   int
	WaterMinutes int
	BreakSeconds int
	Smart        bool
}

func DefaultConfig() Config {
	return Config{EyeEnabled: true, WaterEnabled: true, EyeMinutes: 25, WaterMinutes: 60, BreakSeconds: 30, Smart: true}
}

func (c Config) Validate() error {
	if c.EyeMinutes < 1 || c.EyeMinutes > 180 ||
		c.WaterMinutes < 1 || c.WaterMinutes > 240 ||
		c.BreakSeconds < 20 || c.BreakSeconds > 300 {
		return errors.New("提醒间隔或休息时长超出范围")
	}
	return nil
}

func (c Config) eyeInterval() time.Duration   { return time.Duration(c.EyeMinutes) * time.Minute }
func (c Config) waterInterval() time.Duration { return time.Duration(c.WaterMinutes) * time.Minute }
func (c Config) breakLength() time.Duration   { return time.Duration(c.BreakSeconds) * time.Second }

type Status struct {
	Config         Config
	EyeSeconds     int
	WaterSeconds   int
	RestSeconds    int
	PauseSeconds   int
	Away           bool
	EyePending     bool
	WaterDeferred  bool
	SmartAvailable bool
	Autostart      bool
}

type due struct{ eye, water bool }

type schedule struct {
	config                                           Config
	eyeAt, waterAt, restUntil, pauseUntil, awaySince time.Time
	eyePending, waterIdle                            bool
}

func newSchedule(config Config, now time.Time) schedule {
	return schedule{config: config, eyeAt: now.Add(config.eyeInterval()), waterAt: now.Add(config.waterInterval())}
}

func (s *schedule) configure(config Config, now time.Time) {
	anchor := now
	if !s.awaySince.IsZero() {
		anchor = s.awaySince
	}
	if s.config.EyeMinutes != config.EyeMinutes || s.config.EyeEnabled != config.EyeEnabled || s.config.Smart != config.Smart {
		s.eyeAt = anchor.Add(config.eyeInterval())
		if s.restUntil.After(now) {
			s.eyeAt = s.restUntil.Add(config.eyeInterval())
		}
	}
	if s.config.WaterMinutes != config.WaterMinutes || s.config.WaterEnabled != config.WaterEnabled {
		s.waterAt = now.Add(config.waterInterval())
	}
	s.config = config
	if !config.EyeEnabled {
		s.eyePending = false
	}
}

func (s *schedule) setAway(away bool, now time.Time) {
	if away && s.awaySince.IsZero() {
		s.awaySince = now
		s.eyePending = false
	}
	if !away && !s.awaySince.IsZero() {
		elapsed := now.Sub(s.awaySince)
		if elapsed >= s.config.breakLength() {
			s.eyeAt = now.Add(s.config.eyeInterval())
			if s.restUntil.After(now) {
				s.eyeAt = s.restUntil.Add(s.config.eyeInterval())
			}
		} else if !s.restUntil.After(s.awaySince) {
			s.eyeAt = s.eyeAt.Add(elapsed)
		}
		s.awaySince = time.Time{}
	}
}

func (s *schedule) startRest(now time.Time) {
	s.eyePending = false
	if !s.restUntil.After(now) {
		s.restUntil = now.Add(s.config.breakLength())
		s.eyeAt = s.restUntil.Add(s.config.eyeInterval())
	}
}

func (s *schedule) snoozeEye(now time.Time) {
	if !s.eyePending {
		return
	}
	s.eyePending = false
	s.eyeAt = now.Add(5 * time.Minute)
}

func (s *schedule) finishRest(now time.Time) {
	if s.restUntil.After(now) {
		s.restUntil = time.Time{}
		s.eyeAt = now.Add(s.config.eyeInterval())
	}
}

func (s *schedule) expirePause(now time.Time) {
	if !s.pauseUntil.IsZero() && !s.pauseUntil.After(now) {
		s.resume(now)
	}
}

func (s *schedule) resume(now time.Time) {
	if !s.pauseUntil.IsZero() {
		s.pauseUntil = time.Time{}
		if s.eyeAt.Before(now) {
			s.eyeAt = now.Add(s.config.eyeInterval())
		}
		if s.waterAt.Before(now) {
			s.waterAt = now.Add(s.config.waterInterval())
		}
	}
}

func (s *schedule) takeDue(now time.Time) due {
	s.expirePause(now)
	if !s.awaySince.IsZero() || s.pauseUntil.After(now) {
		return due{}
	}
	eyeReady := s.config.EyeEnabled && !s.eyeAt.After(now)
	waterEnabled := s.config.WaterEnabled && !s.waterIdle
	waterReady := waterEnabled && !s.waterAt.After(now)
	if !eyeReady && !waterReady {
		return due{}
	}
	// Coalesce nearly simultaneous reminders even if their timers were set separately.
	cutoff := now.Add(time.Second)
	result := due{s.config.EyeEnabled && !s.eyeAt.After(cutoff), waterEnabled && !s.waterAt.After(cutoff)}
	if result.eye {
		s.eyePending = true
		s.eyeAt = now.Add(s.config.eyeInterval())
	}
	if result.water {
		s.waterAt = now.Add(s.config.waterInterval())
	}
	return result
}

func (s *schedule) next(now time.Time) time.Time {
	if s.pauseUntil.After(now) {
		return s.pauseUntil
	}
	if !s.awaySince.IsZero() {
		return time.Time{}
	}
	var next time.Time
	if s.config.EyeEnabled {
		next = s.eyeAt
	}
	if s.config.WaterEnabled && !s.waterIdle && (next.IsZero() || s.waterAt.Before(next)) {
		next = s.waterAt
	}
	return next
}

func remaining(at, now time.Time) int {
	if !at.After(now) {
		return 0
	}
	return int((at.Sub(now) + time.Second - 1) / time.Second)
}

func (s *schedule) snapshot(now time.Time) Status {
	eyeNow := now
	if !s.awaySince.IsZero() {
		eyeNow = s.awaySince
	}
	return Status{
		Config: s.config, EyeSeconds: remaining(s.eyeAt, eyeNow),
		WaterSeconds: remaining(s.waterAt, now), RestSeconds: remaining(s.restUntil, now),
		PauseSeconds: remaining(s.pauseUntil, now), Away: !s.awaySince.IsZero(),
		EyePending: s.eyePending, WaterDeferred: s.waterIdle && !s.waterAt.After(now),
	}
}
