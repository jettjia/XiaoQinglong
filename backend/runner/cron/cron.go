package cron

import (
	"strings"
	"time"
)

// ========== Cron Types ==========

// CronFields represents expanded cron expression fields
type CronFields struct {
	Minute     []int
	Hour       []int
	DayOfMonth []int
	Month      []int
	DayOfWeek  []int
}

// FieldRange represents min/max values for a cron field
type FieldRange struct {
	Min int
	Max int
}

var fieldRanges = []FieldRange{
	{0, 59},   // minute
	{0, 23},   // hour
	{1, 31},   // dayOfMonth
	{1, 12},   // month
	{0, 6},    // dayOfWeek (0=Sunday; 7 accepted as Sunday alias)
}

// ========== Cron Parsing ==========

// ParseCronExpression parses a 5-field cron expression into expanded number arrays.
// Returns nil if invalid or unsupported syntax.
// Format: "M H DoM Mon DoW" (minute hour day-of-month month day-of-week)
func ParseCronExpression(expr string) *CronFields {
	parts := strings.Fields(expr)
	if len(parts) != 5 {
		return nil
	}

	expanded := make([][]int, 5)
	for i := 0; i < 5; i++ {
		result := expandField(parts[i], fieldRanges[i])
		if result == nil {
			return nil
		}
		expanded[i] = result
	}

	return &CronFields{
		Minute:     expanded[0],
		Hour:       expanded[1],
		DayOfMonth: expanded[2],
		Month:      expanded[3],
		DayOfWeek:  expanded[4],
	}
}

// expandField parses a single cron field into sorted matching values.
// Supports: wildcard, N, star-slash-N (step), N-M (range), and comma-lists.
func expandField(field string, rangeInfo FieldRange) []int {
	out := make([]int, 0)
	min, max := rangeInfo.Min, rangeInfo.Max

	for _, part := range strings.Split(field, ",") {
		// wildcard or star-slash-N
		if strings.HasPrefix(part, "*") {
			step := 1
			if strings.Contains(part, "/") {
				_, after, found := strings.Cut(part, "/")
				if !found {
					return nil
				}
				n, err := parseInt(after)
				if err != nil || n < 1 {
					return nil
				}
				step = n
			}
			for i := min; i <= max; i += step {
				out = append(out, i)
			}
			continue
		}

		// N-M or N-M/S (range with step)
		if strings.Contains(part, "-") {
			before, after, found := strings.Cut(part, "-")
			if !found {
				return nil
			}
			lo, err := parseInt(before)
			if err != nil {
				return nil
			}

			step := 1
			if strings.Contains(after, "/") {
				rangePart, stepPart, found := strings.Cut(after, "/")
				if !found {
					return nil
				}
				hi, err := parseInt(rangePart)
				if err != nil {
					return nil
				}
				n, err := parseInt(stepPart)
				if err != nil || n < 1 {
					return nil
				}
				step = n
				lo, hi = hi, lo // swap: in range, lo > hi is invalid

				// Special handling for dayOfWeek: accept 7 as Sunday alias
				isDow := min == 0 && max == 6
				effMax := max
				if isDow {
					effMax = 7
				}
				if lo < min || hi > effMax || lo > hi || step < 1 {
					return nil
				}
				for i := lo; i <= hi; i += step {
					if isDow && i == 7 {
						i = 0 // normalize 7 -> 0 for Sunday
					}
					out = append(out, i)
				}
			} else {
				hi, err := parseInt(after)
				if err != nil {
					return nil
				}
				// Special handling for dayOfWeek: accept 7 as Sunday alias
				isDow := min == 0 && max == 6
				effMax := max
				if isDow {
					effMax = 7
				}
				if lo < min || hi > effMax || lo > hi {
					return nil
				}
				for i := lo; i <= hi; i += step {
					if isDow && i == 7 {
						i = 0 // normalize 7 -> 0 for Sunday
					}
					out = append(out, i)
				}
			}
			continue
		}

		// plain N
		n, err := parseInt(part)
		if err != nil {
			return nil
		}
		// dayOfWeek: accept 7 as Sunday alias -> 0
		if min == 0 && max == 6 && n == 7 {
			n = 0
		}
		if n < min || n > max {
			return nil
		}
		out = append(out, n)
	}

	if len(out) == 0 {
		return nil
	}

	// Sort ascending and remove duplicates
	unique := make([]int, 0, len(out))
	seen := make(map[int]bool)
	for _, v := range out {
		if !seen[v] {
			seen[v] = true
			unique = append(unique, v)
		}
	}
	return unique
}

// ========== Next Run Calculation ==========

// ComputeNextCronRun computes the next Date strictly after `from` that matches the cron fields.
// Returns nil if no match within 366 days.
func ComputeNextCronRun(fields *CronFields, from time.Time) *time.Time {
	// Create minute sets for O(1) lookup
	minuteSet := make(map[int]bool)
	hourSet := make(map[int]bool)
	domSet := make(map[int]bool)
	monthSet := make(map[int]bool)
	dowSet := make(map[int]bool)

	for _, m := range fields.Minute {
		minuteSet[m] = true
	}
	for _, h := range fields.Hour {
		hourSet[h] = true
	}
	for _, d := range fields.DayOfMonth {
		domSet[d] = true
	}
	for _, m := range fields.Month {
		monthSet[m] = true
	}
	for _, d := range fields.DayOfWeek {
		dowSet[d] = true
	}

	// Check if fields are wildcarded (full range)
	domWild := len(fields.DayOfMonth) == 31
	dowWild := len(fields.DayOfWeek) == 7

	// Round up to the next whole minute (strictly after `from`)
	t := from.Truncate(time.Minute).Add(time.Minute)

	maxIter := 366 * 24 * 60
	for i := 0; i < maxIter; i++ {
		month := int(t.Month())
		if !monthSet[month] {
			// Jump to start of next month
			t = time.Date(t.Year(), t.Month()+1, 1, 0, 0, 0, 0, t.Location())
			continue
		}

		dom := t.Day()
		dow := int(t.Weekday())
		// When both dom/dow are constrained, either match is sufficient (OR semantics)
		var dayMatches bool
		if domWild && dowWild {
			dayMatches = true
		} else if domWild {
			dayMatches = dowSet[dow]
		} else if dowWild {
			dayMatches = domSet[dom]
		} else {
			dayMatches = domSet[dom] || dowSet[dow]
		}

		if !dayMatches {
			// Jump to start of next day
			t = t.AddDate(0, 0, 1)
			t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
			continue
		}

		if !hourSet[t.Hour()] {
			// Jump to next hour
			t = t.Add(time.Hour * time.Duration((t.Hour()/24)*-1+1))
			t = time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, t.Location())
			continue
		}

		if !minuteSet[t.Minute()] {
			// Jump to next minute
			t = t.Add(time.Minute)
			continue
		}

		result := t
		return &result
	}

	return nil
}

// ========== Cron to Human ==========

var dayNames = []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}

// CronToHuman converts a cron expression to a human-readable string
func CronToHuman(cron string) string {
	parts := strings.Fields(cron)
	if len(parts) != 5 {
		return cron
	}

	minute, hour, dayOfMonth, month, dayOfWeek := parts[0], parts[1], parts[2], parts[3], parts[4]

	// Every N minutes: */N * * * *
	if strings.HasPrefix(minute, "*/") && hour == "*" && dayOfMonth == "*" && month == "*" && dayOfWeek == "*" {
		n := strings.TrimPrefix(minute, "*/")
		if n == "1" {
			return "Every minute"
		}
		return "Every " + n + " minutes"
	}

	// Every hour: 0 * * * *
	if minute == "0" && hour == "*" && dayOfMonth == "*" && month == "*" && dayOfWeek == "*" {
		return "Every hour"
	}

	// Daily at specific time: M H * * *
	if dayOfMonth == "*" && month == "*" && dayOfWeek == "*" {
		m, err := parseInt(minute)
		if err != nil {
			return cron
		}
		h, err := parseInt(hour)
		if err != nil {
			return cron
		}
		t := time.Date(2000, 1, 1, h, m, 0, 0, time.Local)
		return "Every day at " + t.Format("3:04 PM")
	}

	// Specific day of week: M H * * D
	if dayOfMonth == "*" && month == "*" && strings.TrimSpace(dayOfWeek) != "*" {
		m, err := parseInt(minute)
		if err != nil {
			return cron
		}
		h, err := parseInt(hour)
		if err != nil {
			return cron
		}
		d, err := parseInt(dayOfWeek)
		if err != nil {
			return cron
		}
		d = d % 7 // normalize 7 (Sunday alias) -> 0
		if d >= 0 && d < len(dayNames) {
			t := time.Date(2000, 1, 1, h, m, 0, 0, time.Local)
			return "Every " + dayNames[d] + " at " + t.Format("3:04 PM")
		}
	}

	return cron
}

// parseInt parses a string to int
func parseInt(s string) (int, error) {
	var n int
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, &parseError{s}
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}

type parseError struct {
	s string
}

func (e *parseError) Error() string {
	return "invalid number: " + e.s
}
