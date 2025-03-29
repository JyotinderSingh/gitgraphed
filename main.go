package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ContributionDay represents a single day in the contribution graph
type ContributionDay struct {
	Date         string `json:"date"`
	Count        int    `json:"count"`
	Level        int    `json:"level"`
	DayOfWeek    int    `json:"dayOfWeek"`
	WeekOfYear   int    `json:"weekOfYear"`
	ContribLevel string `json:"contribLevel"` // none, first_quartile, second_quartile, third_quartile, fourth_quartile
}

// ContributionGraph represents the complete contribution data
type ContributionGraph struct {
	Username      string            `json:"username"`
	TotalContribs int               `json:"totalContributions"`
	Years         []int             `json:"years"`
	Days          []ContributionDay `json:"days"`
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: gitgraphed <username> [year]")
		os.Exit(1)
	}

	username := os.Args[1]
	year := time.Now().Year()

	if len(os.Args) >= 3 {
		parsedYear, err := strconv.Atoi(os.Args[2])
		if err == nil {
			year = parsedYear
		}
	}

	graph, err := fetchContributionGraph(username, year)
	if err != nil {
		fmt.Printf("Error fetching contribution data: %v\n", err)
		os.Exit(1)
	}

	// Output JSON to stdout
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(graph); err != nil {
		fmt.Printf("Error encoding JSON: %v\n", err)
		os.Exit(1)
	}
}

func fetchContributionGraph(username string, year int) (*ContributionGraph, error) {
	url := fmt.Sprintf("https://github.com/users/%s/contributions?from=%d-01-01&to=%d-12-31",
		username, year, year)

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	// Add headers to make it look like a browser request
	req.Header.Add("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	req.Header.Add("Accept", "text/html,application/xhtml+xml,application/xml")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP request failed with status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	htmlContent := string(body)

	// Extract total contributions
	totalRegex := regexp.MustCompile(`(\d+) contributions in the last year`)
	totalMatches := totalRegex.FindStringSubmatch(htmlContent)
	totalContribs := 0
	if len(totalMatches) > 1 {
		totalContribs, _ = strconv.Atoi(totalMatches[1])
	}

	// Find all the contribution days
	dayRegex := regexp.MustCompile(`data-date="([^"]+)"[^>]+data-level="([^"]+)"[^>]*>([^<]*)<\/td>`)
	dayMatches := dayRegex.FindAllStringSubmatch(htmlContent, -1)

	days := make([]ContributionDay, 0, len(dayMatches))

	for _, match := range dayMatches {
		dateStr := match[1]
		levelStr := match[2]
		countStr := strings.TrimSpace(match[3])

		// Parse date
		date, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			continue
		}

		// Parse count (GitHub shows "No contributions" or "X contributions")
		count := 0
		if countStr != "No contributions" && countStr != "" {
			countParts := strings.Fields(countStr)
			if len(countParts) > 0 {
				count, _ = strconv.Atoi(countParts[0])
			}
		}

		// Parse level
		level, _ := strconv.Atoi(levelStr)

		// Determine contribution level name
		var contribLevel string
		switch level {
		case 0:
			contribLevel = "none"
		case 1:
			contribLevel = "first_quartile"
		case 2:
			contribLevel = "second_quartile"
		case 3:
			contribLevel = "third_quartile"
		case 4:
			contribLevel = "fourth_quartile"
		}

		day := ContributionDay{
			Date:         dateStr,
			Count:        count,
			Level:        level,
			DayOfWeek:    int(date.Weekday()),
			WeekOfYear:   getWeekOfYear(date),
			ContribLevel: contribLevel,
		}

		days = append(days, day)
	}

	return &ContributionGraph{
		Username:      username,
		TotalContribs: totalContribs,
		Years:         []int{year},
		Days:          days,
	}, nil
}

func getWeekOfYear(date time.Time) int {
	_, week := date.ISOWeek()
	return week
}
