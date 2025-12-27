package types

import (
	"fmt"
	"time"
)

// TimeResponse is the response from a get_time command.
type TimeResponse struct {
	Time struct {
		GetTime TimeInfo `json:"get_time"`
	} `json:"time"`
}

// TimeInfo represents the device's current time.
type TimeInfo struct {
	Year    int `json:"year"`
	Month   int `json:"month"`
	Day     int `json:"mday"`
	Hour    int `json:"hour"`
	Min     int `json:"min"`
	Sec     int `json:"sec"`
	ErrCode int `json:"err_code"`
}

// ToTime converts TimeInfo to a time.Time value.
func (t TimeInfo) ToTime() time.Time {
	return time.Date(t.Year, time.Month(t.Month), t.Day, t.Hour, t.Min, t.Sec, 0, time.Local)
}

// String returns a formatted time string.
func (t TimeInfo) String() string {
	return fmt.Sprintf("%04d-%02d-%02d %02d:%02d:%02d",
		t.Year, t.Month, t.Day, t.Hour, t.Min, t.Sec)
}

// TimezoneResponse is the response from a get_timezone command.
type TimezoneResponse struct {
	Time struct {
		GetTimezone TimezoneInfo `json:"get_timezone"`
	} `json:"time"`
}

// TimezoneInfo represents the device's timezone setting.
type TimezoneInfo struct {
	Index   int `json:"index"`
	ErrCode int `json:"err_code"`
}

// SetTimeResponse is the response from a set_time command.
type SetTimeResponse struct {
	Time struct {
		SetTime struct {
			ErrCode int `json:"err_code"`
		} `json:"set_time"`
	} `json:"time"`
}

// SetTimezoneResponse is the response from a set_timezone command.
type SetTimezoneResponse struct {
	Time struct {
		SetTimezone struct {
			ErrCode int `json:"err_code"`
		} `json:"set_timezone"`
	} `json:"time"`
}
