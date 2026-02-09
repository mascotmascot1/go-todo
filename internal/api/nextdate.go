package api

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/mascotmascot1/go-todo/internal/db"
)

const (
	maxDaysInterval         = 400
	sundayNum               = 7
	expectedPartsWithMonths = 3
	lastDayOfMonth          = -1
	beforeLastDayOfMonth    = -2
)

var (
	reDay     = regexp.MustCompile(`^d \d{1,3}$`)
	reYear    = regexp.MustCompile(`^y$`)
	reWeek    = regexp.MustCompile(`^w \d(,[\d])*$`)
	reMonth   = regexp.MustCompile(`^m -?\d{1,2}(,-?\d{1,2})*( \d{1,2}(,\d{1,2})*)?$`)
	allMonths = []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}
)

// NextDate computes the next date given a date and a repeat rule.
// The repeat rule can be one of the following formats:
//
// - "d <number>"           — daily repeat every <number> days
// - "y"                    — yearly repeat
// - "w <num1,num2,...>"    — weekly repeat on specified weekdays (1=Monday, 7=Sunday)
// - "m <day1,day2,...> <month1,month2,...>" — monthly repeat on specified days and months;
//                                             days can be 1..31 or negative (-1 for last day, -2 for second to last, etc.),
//                                             months can be 1..12
//
// If the repeat rule is empty, it returns a 400 error.
// If the initial date is invalid, it returns a 400 error.
// If the server fails to compute the next date, it returns a 400 error.
// The response is in plain text format and contains the next date in "YYYY-MM-DD".
func NextDate(now time.Time, dStart string, repeat string) (string, error) {
	repeat, dStart = strings.TrimSpace(repeat), strings.TrimSpace(dStart)
	if repeat == "" {
		return "", fmt.Errorf("repeat rule is empty")
	}
	startDate, err := time.Parse(db.DateLayoutDB, dStart)
	if err != nil {
		return "", fmt.Errorf("error parsing the initial date '%s': %w", dStart, err)
	}
	now = midnight(now)

	switch {
	case reDay.MatchString(repeat):
		return nextDaily(now, startDate, repeat)

	case reYear.MatchString(repeat):
		return nextYearly(now, startDate)

	case reWeek.MatchString(repeat):
		return nextWeekly(now, startDate, repeat)

	case reMonth.MatchString(repeat):
		return nextMonthly(now, startDate, repeat)

	default:
		return "", fmt.Errorf("unsupported interval format '%s'", repeat)
	}
}

// nextDaily computes the next date given a date and a daily repeat rule.
// The repeat rule is of the format "d <number>" where <number> is the number of days to repeat.
// The function will return an error if the repeat rule is invalid or if the server fails to compute the next date.
// The response is in plain text format and contains the next date in "YYYY-MM-DD".
func nextDaily(now, startDate time.Time, repeat string) (string, error) {
	parts := strings.Split(repeat, " ")
	days, err := strconv.Atoi(parts[1])
	if err != nil {
		return "", fmt.Errorf("invalid day value: %w", err)
	}
	if days <= 0 || days > maxDaysInterval {
		return "", fmt.Errorf("invalid day interval '%d'", days)
	}

	for {
		startDate = startDate.AddDate(0, 0, days)
		if afterNow(now, startDate) {
			break
		}
	}
	return startDate.Format(db.DateLayoutDB), nil
}

// nextYearly computes the next date given a date and a yearly repeat rule.
// The repeat rule is of the format "y".
// The function will return an error if the repeat rule is invalid or if the server fails to compute the next date.
// The response is in plain text format and contains the next date in "YYYY-MM-DD".
func nextYearly(now, startDate time.Time) (string, error) {
	for {
		startDate = startDate.AddDate(1, 0, 0)
		if afterNow(now, startDate) {
			break
		}
	}
	return startDate.Format(db.DateLayoutDB), nil
}

// nextWeekly computes the next date given a date and a weekly repeat rule.
// The repeat rule is of the format "w <number>,<number>,..."
// where <number> is the day of the week to repeat (1 for Monday, 2 for Tuesday, etc.).
// The function will return an error if the repeat rule is invalid or if the server fails to compute the next date.
// The response is in plain text format and contains the next date in "YYYY-MM-DD".
func nextWeekly(now, startDate time.Time, repeat string) (string, error) {
	parts := strings.Split(repeat, " ")
	weekDaysStr := strings.Split(parts[1], ",")

	weekDaysInt := make([]int, 0, len(weekDaysStr))
	for _, wd := range weekDaysStr {
		wdNum, err := strconv.Atoi(wd)
		if err != nil {
			return "", fmt.Errorf("invalid weekday value: %w", err)
		}
		if wdNum <= 0 || wdNum > 7 {
			return "", fmt.Errorf("invalid weekday interval: %w", err)
		}
		weekDaysInt = append(weekDaysInt, wdNum)
	}
	weekDaysInt = sortUniqueInts(weekDaysInt)

	baseTime := computeBaseTime(now, startDate)
	currentWeekDay := int(baseTime.Weekday())
	if currentWeekDay == 0 {
		currentWeekDay = sundayNum
	}

	var daysToAdd int
	for _, wd := range weekDaysInt {
		if currentWeekDay < wd {
			daysToAdd = wd - currentWeekDay
			break
		}
	}
	if daysToAdd == 0 {
		daysToAdd = 7 - currentWeekDay + weekDaysInt[0]
	}

	newDate := baseTime.AddDate(0, 0, daysToAdd)
	return newDate.Format(db.DateLayoutDB), nil
}

// nextMonthly computes the next date given a date and a monthly repeat rule.
// The repeat rule is of the format "m <day1,day2,...> <month1,month2,...>" where <day1,day2,...> is the comma-separated list of days in the month to repeat, and <month1,month2,...> is the comma-separated list of months to repeat.
// If the <month1,month2,...> part is not provided, the function will consider all months from 1 to 12.
// If the <day1,day2,...> part is not provided, the function will consider all days in the month from 1 to 31.
// If the repeat rule is invalid or if the server fails to compute the next date, the function will return an error.
// The response is in plain text format and contains the next date in "YYYY-MM-DD".

func nextMonthly(now, startDate time.Time, repeat string) (string, error) {
	parts := strings.Split(repeat, " ")
	monthDaysStr := strings.Split(parts[1], ",")

	monthDaysInt := make([]int, 0, len(monthDaysStr))
	for _, md := range monthDaysStr {
		mdNum, err := strconv.Atoi(md)
		if err != nil {
			return "", fmt.Errorf("invalid monthday value: %w", err)
		}
		if (mdNum != lastDayOfMonth && mdNum != beforeLastDayOfMonth) && (mdNum <= 0 || mdNum > 31) {
			return "", fmt.Errorf("invalid monthday interval '%d'", mdNum)
		}
		monthDaysInt = append(monthDaysInt, mdNum)
	}
	monthDaysInt = sortUniqueInts(monthDaysInt)

	monthsInt := make([]int, 0, 12)
	if len(parts) == expectedPartsWithMonths {
		monthsStr := strings.Split(parts[2], ",")

		for _, m := range monthsStr {
			mNum, err := strconv.Atoi(m)
			if err != nil {
				return "", fmt.Errorf("invalid monthd value: %w", err)
			}
			if mNum <= 0 || mNum > 12 {
				return "", fmt.Errorf("invalid month interval '%d'", mNum)
			}
			monthsInt = append(monthsInt, mNum)
		}
		monthsInt = sortUniqueInts(monthsInt)
	}
	if len(monthsInt) == 0 {
		monthsInt = allMonths
	}

	var (
		baseTime        = computeBaseTime(now, startDate)
		currentMonth    = int(baseTime.Month())
		currentMonthDay = baseTime.Day()
		currentYear     = baseTime.Year()
		nextYear        = currentYear + 1
	)
	for _, m := range monthsInt {
		if currentMonth <= m {
			monthDays := resolveDays(monthDaysInt, currentYear, m, baseTime.Location())

			for _, md := range monthDays {
				maxMonthDay := computeLastMonthDay(currentYear, m, baseTime.Location())
				if md > maxMonthDay {
					break
				}

				if currentMonth == m {
					if md <= currentMonthDay {
						continue
					}
					newDate := baseTime.AddDate(0, 0, md-currentMonthDay)
					return newDate.Format(db.DateLayoutDB), nil
				}
				newDate := time.Date(currentYear, time.Month(m), md, 0, 0, 0, 0, baseTime.Location())
				return newDate.Format(db.DateLayoutDB), nil
			}
		}
	}

	// if currentMonth > m logic below.
	for _, m := range monthsInt {
		monthDays := resolveDays(monthDaysInt, nextYear, m, baseTime.Location())

		for _, md := range monthDays {
			maxMonthDay := computeLastMonthDay(nextYear, m, baseTime.Location())
			if md > maxMonthDay {
				break
			}

			newDate := time.Date(nextYear, time.Month(m), md, 0, 0, 0, 0, baseTime.Location())
			return newDate.Format(db.DateLayoutDB), nil
		}
	}
	return "", fmt.Errorf("invalid repeat rule: there aren't these days in submitted months '%s'", repeat)
}

// afterNow checks if the given date is after the given now time.
// It returns true if the date is after now, and false otherwise.
func afterNow(now, date time.Time) bool {
	return now.Before(date)
}

// sortUniqueInts sorts the given array of integers and removes duplicates.
// It returns the sorted slice with unique integers.
func sortUniqueInts(in []int) []int {
	slices.Sort(in)
	return slices.Compact(in)
}

// computeBaseTime computes the base time to be used for the next date computation.
// If the startDate is after the now time, it returns the startDate.
// Otherwise, it returns the now time.
//
// This function is used to ensure that the next date computation is done relative to the current time if the startDate is not specified, or relative to the specified date if it is after the current time.
func computeBaseTime(now, startDate time.Time) time.Time {
	if afterNow(now, startDate) {
		return startDate
	}
	return now
}

// computeLastMonthDay computes the last day of the given month and year in the given location.
// It takes the year, month and location as arguments and returns the last day of the given month.
// If the month is December, it will consider the last day of January of the next year as the last day of December.
func computeLastMonthDay(year, month int, loc *time.Location) int {
	lastDay := time.Date(year, time.Month(month+1), 0, 0, 0, 0, 0, loc).Day()
	return lastDay
}

// resolveDays resolves the given month days relative to the given year and month in the given location.
// If the month day is negative, it is considered to be relative to the first day of the following month.
// For example, if the month day is -1, it will be resolved to the last day of the given month.
// The function takes the month days as a slice of integers, the year and month as integers, and the location as a pointer to time.Location.
// It returns a slice of resolved month days in ascending order with no duplicates.
func resolveDays(monthDaysInt []int, y, m int, loc *time.Location) []int {
	resolved := make([]int, len(monthDaysInt))

	for i, d := range monthDaysInt {
		if d < 0 {
			// d+1, потому что для -1 (последний день) нам нужно смещение 0 от первого числа следующего месяца
			realDay := time.Date(y, time.Month(m+1), d+1, 0, 0, 0, 0, loc).Day()
			resolved[i] = realDay
		} else {
			resolved[i] = d
		}
	}
	return sortUniqueInts(resolved)
}

// midnight returns a new time that represents midnight of the given time.
// It takes the given time as an argument and returns a new time with the same year, month and day, but with the hour, minute, second and timezone offset set to zero.
// The returned time is in the UTC timezone.
func midnight(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}
