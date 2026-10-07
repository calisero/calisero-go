package calisero

import (
	"sync"
	"time"
)

// scheduleLayout is how the API reads schedule_at: YYYY-MM-DD HH:MM:SS, Romania time.
const scheduleLayout = "2006-01-02 15:04:05"

// romaniaLocation is Europe/Bucharest from the time zone database; nil when no
// database is available, e.g. in a container image without one. The library does
// not embed time/tzdata, which would add 450 KB to every program that imports it.
var romaniaLocation = sync.OnceValue(func() *time.Location {
	return loadLocation("Europe/Bucharest")
})

// loadLocation is a location of the time zone database; nil when the database is
// not available or does not have it.
func loadLocation(name string) *time.Location {
	location, err := time.LoadLocation(name)
	if err != nil {
		return nil
	}

	return location
}

// formatScheduleAt writes t as the API reads schedule_at: YYYY-MM-DD HH:MM:SS,
// Romania time, summer and winter time handled.
func formatScheduleAt(t time.Time) string {
	return inRomania(t, romaniaLocation()).Format(scheduleLayout)
}

// inRomania is t in Romania time: in location, else by the rules of the European
// Union that Romania follows.
func inRomania(t time.Time, location *time.Location) time.Time {
	if location != nil {
		return t.In(location)
	}

	if offset := romaniaOffset(t); offset == 3*60*60 {
		return t.In(time.FixedZone("EEST", offset))
	}

	return t.In(time.FixedZone("EET", 2*60*60))
}

// romaniaOffset is Romania's offset from UTC at t, in seconds, by the rules of the
// European Union: UTC+3 (summer time) from the last Sunday of March, 01:00 UTC, to
// the last Sunday of October, 01:00 UTC; UTC+2 otherwise.
func romaniaOffset(t time.Time) int {
	utc := t.UTC()
	summerStarts := lastSunday(utc.Year(), time.March).Add(time.Hour)
	summerEnds := lastSunday(utc.Year(), time.October).Add(time.Hour)

	if !utc.Before(summerStarts) && utc.Before(summerEnds) {
		return 3 * 60 * 60
	}

	return 2 * 60 * 60
}

// lastSunday is the last Sunday of a month, at midnight UTC.
func lastSunday(year int, month time.Month) time.Time {
	lastDay := time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC)

	return lastDay.AddDate(0, 0, -int(lastDay.Weekday()))
}
