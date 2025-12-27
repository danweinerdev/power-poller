package command

import "time"

// Note: GetTime and GetTimezone are defined in system.go

// SetTime returns a command to set the device's time.
func SetTime(t time.Time) Command {
	return New("time", "set_time", map[string]interface{}{
		"year":  t.Year(),
		"month": int(t.Month()),
		"mday":  t.Day(),
		"hour":  t.Hour(),
		"min":   t.Minute(),
		"sec":   t.Second(),
	})
}

// SetTimezone returns a command to set the device's timezone.
// The index corresponds to a timezone in the device's timezone list.
func SetTimezone(index int) Command {
	return New("time", "set_timezone", map[string]interface{}{
		"index": index,
	})
}
