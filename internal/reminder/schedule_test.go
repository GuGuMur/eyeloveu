package reminder

import (
	"testing"
	"time"
)

func TestIdleAtStartupDoesNotIncreaseCountdown(t *testing.T) {
	config := DefaultConfig()
	now := time.Now()
	s := newSchedule(config, now)
	s.markIdle(now.Add(time.Second), idleThreshold)
	status := s.snapshot(now.Add(time.Second))
	if !status.Away || status.EyeSeconds != 25*60 {
		t.Fatalf("initial idle state = %+v", status)
	}
}

func TestLongAbsenceResetsEyeAndDefersWater(t *testing.T) {
	now := time.Now()
	s := newSchedule(DefaultConfig(), now)
	s.markIdle(now.Add(24*time.Minute), idleThreshold)
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
