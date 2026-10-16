package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

var heatmapLevels = []lipgloss.Style{
	lipgloss.NewStyle().Foreground(lipgloss.Color("238")), // no backup that day
	lipgloss.NewStyle().Foreground(lipgloss.Color("22")),  // small change
	lipgloss.NewStyle().Foreground(lipgloss.Color("28")),
	lipgloss.NewStyle().Foreground(lipgloss.Color("34")),
	lipgloss.NewStyle().Foreground(lipgloss.Color("40")), // large change
}

const heatmapWeeks = 52
const columnWidth = 3 // glyph + 2 spaces; matches a full 3-letter month abbreviation exactly

// dayKey returns a YYYY-MM-DD key in local time for grouping.
func dayKey(t time.Time) string {
	return t.Local().Format("2006-01-02")
}

// bucketForBytes maps a data-added volume to an intensity level index (1..len(heatmapLevels)-1).
// Thresholds are coarse: this is a heatmap, not a precise chart.
func bucketForBytes(b uint64) int {
	switch {
	case b == 0:
		return 0
	case b < 50*1024*1024: // <50MB
		return 1
	case b < 500*1024*1024: // <500MB
		return 2
	case b < 5*1024*1024*1024: // <5GB
		return 3
	default:
		return 4
	}
}

// renderHeatmap draws a GitHub-style contribution grid: rows = weekdays (Su..Sa),
// columns = weeks, most recent week last. Intensity reflects summary.data_added
// summed per calendar day across all snapshots that day.
func renderHeatmap(snaps []Snapshot, width int) string {
	byDay := map[string]uint64{}
	for _, s := range snaps {
		byDay[dayKey(s.Time)] += s.Summary.DataAdded
	}

	weeks := heatmapWeeks
	// each column is 3 chars wide (glyph + 2 spaces), to exactly fit a full
	// 3-letter month abbreviation with no truncation; leave room for the
	// "XX " weekday label prefix (4 chars).
	if width > 0 {
		maxWeeks := (width - 4) / columnWidth
		if maxWeeks < weeks && maxWeeks > 4 {
			weeks = maxWeeks
		}
	}

	today := time.Now().Local()
	todayDate := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, today.Location())
	// Anchor from the end (Saturday of the current week) so the grid always
	// reaches today, then derive start from there — start lands on a Sunday
	// automatically since weeks*7 is a multiple of 7.
	weekEnd := todayDate.AddDate(0, 0, 6-int(todayDate.Weekday()))
	start := weekEnd.AddDate(0, 0, -(weeks*7 - 1))

	// Precompute month label row: for each column, does a new month start within it?
	monthLabel := make([]string, weeks)
	lastMonth := ""
	for c := 0; c < weeks; c++ {
		colStart := start.AddDate(0, 0, c*7)
		for d := 0; d < 7; d++ {
			day := colStart.AddDate(0, 0, d)
			dm := day.Format("Jan")
			if day.Day() == 1 && dm != lastMonth {
				monthLabel[c] = dm
				lastMonth = dm
				break
			}
		}
	}

	var b strings.Builder
	b.WriteString("    ")
	for c := 0; c < weeks; c++ {
		// monthLabel[c] is always either "" or a full 3-letter abbreviation
		// (e.g. "Jan"), which exactly fills columnWidth — no truncation needed.
		b.WriteString(fmt.Sprintf("%-*s", columnWidth, monthLabel[c]))
	}
	b.WriteString("\n")

	weekdayLabels := []string{"Su", "Mo", "Tu", "We", "Th", "Fr", "Sa"}
	for row := 0; row < 7; row++ {
		b.WriteString(fmt.Sprintf("%-3s ", weekdayLabels[row]))
		for c := 0; c < weeks; c++ {
			day := start.AddDate(0, 0, c*7+row)
			if day.After(todayDate) {
				b.WriteString(strings.Repeat(" ", columnWidth))
				continue
			}
			bytesAdded := byDay[dayKey(day)]
			level := bucketForBytes(bytesAdded)
			glyph := "■"
			if level == 0 {
				glyph = "·" // distinct shape, not just color, for no-backup days
			}
			b.WriteString(heatmapLevels[level].Render(glyph) + strings.Repeat(" ", columnWidth-1))
		}
		b.WriteString("\n")
	}

	return b.String()
}
