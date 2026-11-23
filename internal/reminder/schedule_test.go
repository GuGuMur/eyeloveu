package reminder

import (
	"testing"
	"time"
)

func TestKeyboardIdleDoesNotDelayScreenBreak(t *testing.T) {
	config := DefaultConfig()
	config.WaterMinutes = config.EyeMinutes
	now := time.Now()
	s := newSchedule(config, now)
	s.waterIdle = true
	deadline := now.Add(config.eyeInterval())
	if next := s.next(now); !next.Equal(deadline) {
		t.Fatalf("next alarm = %v, want %v", next, deadline)
	}
	if got := s.takeDue(deadline); got != (due{eye: true}) {
		t.Fatalf("reminders without keyboard input = %+v", got)
	}
	if status := s.snapshot(deadline); !status.EyePending || !status.WaterDeferred || status.Away {
		t.Fatalf("reminders without keyboard input = %+v", status)
	}
	s.waterIdle = false
	if got := s.takeDue(deadline); got != (due{water: true}) {
		t.Fatalf("reminders after input resumes = %+v", got)
	}
}

func TestLongAbsenceResetsEyeAndDefersWater(t *testing.T) {
	now := time.Now()
	s := newSchedule(DefaultConfig(), now)
	s.setAway(true, now.Add(24*time.Minute))
	returned := now.Add(65 * time.Minute)
	if got := s.takeDue(returned.Add(-time.Second)); got != (due{}) {
		t.Fatalf("reminders while away = %+v", got)
	}
	s.setAway(false, returned)
	if got := s.takeDue(returned); got != (due{water: true}) {
		t.Fatalf("reminders on return = %+v", got)
	}
	if got := s.snapshot(returned).EyeSeconds; got != 25*60 {
		t.Fatalf("eye seconds on return = %d", got)
	}
}

func TestBreakPromptSnoozeAndStart(t *testing.T) {
	now := time.Now()
	s := newSchedule(DefaultConfig(), now)
	now = s.eyeAt
	s.takeDue(now)
	s.snoozeEye(now)
	if status := s.snapshot(now); status.EyePending || status.EyeSeconds != 300 {
		t.Fatalf("snoozed break = %+v", status)
	}
	now = now.Add(5 * time.Minute)
	s.takeDue(now)
	s.startRest(now)
	if status := s.snapshot(now); status.EyePending || status.RestSeconds != DefaultConfig().BreakSeconds {
		t.Fatalf("started break = %+v", status)
	}
}

func TestCoincidentRemindersAdvanceTogether(t *testing.T) {
	now := time.Now()
	config := DefaultConfig()
	config.WaterMinutes = config.EyeMinutes
	s := newSchedule(config, now)
	s.waterAt = s.waterAt.Add(250 * time.Millisecond)
	deadline := s.eyeAt
	if got := s.takeDue(deadline); got != (due{eye: true, water: true}) {
		t.Fatalf("reminders at shared deadline = %+v", got)
	}
	if got := s.takeDue(deadline); got != (due{}) {
		t.Fatalf("duplicate reminders = %+v", got)
	}
}
