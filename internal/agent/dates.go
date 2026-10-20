package agent

import (
	"strings"
	"time"
)

func normalizePeriod(period string) string {
	p := foldYo(strings.TrimSpace(period))
	p = strings.ReplaceAll(p, "_", " ")
	p = strings.Join(strings.Fields(p), " ")
	return p
}

func ResolvePeriod(period string, loc *time.Location) (from, to *time.Time) {
	now := time.Now().In(loc)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	switch normalizePeriod(period) {
	case "today", "сегодня":
		t := today
		return &t, &t
	case "yesterday", "вчера":
		y := today.AddDate(0, 0, -1)
		return &y, &y
	case "week", "неделя", "этой неделе", "за неделю", "эта неделя":
		weekday := int(today.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		fromD := today.AddDate(0, 0, -(weekday - 1))
		return &fromD, &today
	case "last week", "прошлая неделя", "прошлой неделе", "за прошлую неделю", "lastweek":
		weekday := int(today.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		thisMonday := today.AddDate(0, 0, -(weekday - 1))
		fromD := thisMonday.AddDate(0, 0, -7)
		toD := thisMonday.AddDate(0, 0, -1)
		return &fromD, &toD
	case "month", "месяц", "этом месяце", "за месяц", "этот месяц":
		fromD := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, loc)
		return &fromD, &today
	case "last month", "прошлый месяц", "прошлом месяце", "за прошлый месяц", "lastmonth":
		firstThis := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, loc)
		lastPrev := firstThis.AddDate(0, 0, -1)
		firstPrev := time.Date(lastPrev.Year(), lastPrev.Month(), 1, 0, 0, 0, 0, loc)
		return &firstPrev, &lastPrev
	case "year", "год", "в этом году", "за год", "этот год":
		fromD := time.Date(today.Year(), 1, 1, 0, 0, 0, 0, loc)
		return &fromD, &today
	case "all", "все", "все время", "за все время":
		return nil, nil
	default:
		return nil, nil
	}
}

func ParseDate(s string, loc *time.Location) *time.Time {
	s = strings.TrimSpace(foldYo(s))
	if s == "" {
		return nil
	}
	now := time.Now().In(loc)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	switch s {
	case "today", "сегодня":
		return &today
	case "yesterday", "вчера":
		y := today.AddDate(0, 0, -1)
		return &y
	case "tomorrow", "завтра":
		t := today.AddDate(0, 0, 1)
		return &t
	}
	if t, err := time.ParseInLocation("2006-01-02", s, loc); err == nil {
		return &t
	}
	return nil
}

func PeriodFromArgs(args map[string]any, loc *time.Location) (from, to *time.Time) {
	if p, ok := args["period"].(string); ok && p != "" {
		return ResolvePeriod(p, loc)
	}
	if f, ok := args["from"].(string); ok {
		from = ParseDate(f, loc)
	}
	if t, ok := args["to"].(string); ok {
		to = ParseDate(t, loc)
	}
	return from, to
}

func PeriodHintFromText(text string) string {
	t := foldYo(text)
	switch {
	case strings.Contains(t, "позавчера"):
		return "" // from/to computed by the model; we don't have a named period
	case strings.Contains(t, "вчера") || strings.Contains(t, "yesterday"):
		return "yesterday"
	case strings.Contains(t, "сегодня") || strings.Contains(t, "today"):
		return "today"
	case strings.Contains(t, "прошлой недел") || strings.Contains(t, "прошлую недел") || strings.Contains(t, "last week"):
		return "last_week"
	case strings.Contains(t, "недел") || strings.Contains(t, "week"):
		return "week"
	case (strings.Contains(t, "прошл") && strings.Contains(t, "месяц")) || strings.Contains(t, "last month"):
		return "last_month"
	case strings.Contains(t, "месяц") || strings.Contains(t, "month"):
		return "month"
	case strings.Contains(t, "год") || strings.Contains(t, "year"):
		return "year"
	case strings.Contains(t, "все время") || strings.Contains(t, "за все"):
		return "all"
	default:
		return ""
	}
}
