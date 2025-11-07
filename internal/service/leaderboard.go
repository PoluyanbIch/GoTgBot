package service

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

type LeaderboardEntry struct {
	UserID     int64  `json:"user_id"`
	Username   string `json:"username"`
	FirstName  string `json:"first_name"`
	Score      int    `json:"score"`
	Total      int    `json:"total"`
	Percentage int    `json:"percentage"`
	Date       string `json:"date"`
}

type Leaderboard struct {
	Entries []LeaderboardEntry `json:"entries"`
	mu      sync.RWMutex
}

type LeaderboardService interface {
	AddEntry(userID int64, username, firstName string, score, total int) bool
	GetTop(limit int) []LeaderboardEntry
	GetUserPosition(userID int64) (int, *LeaderboardEntry)
}

// GistLeaderboardService использует GitHub Gist для хранения
type GistLeaderboardService struct {
	gistID      string
	githubToken string
	filename    string
}

func NewLeaderboardService() LeaderboardService {
	gistID := os.Getenv("GITHUB_GIST_ID")
	githubToken := os.Getenv("GITHUB_TOKEN")

	if gistID != "" && githubToken != "" {
		return NewGistLeaderboardService(gistID, githubToken)
	}

	// Fallback - in-memory (данные теряются при рестарте)
	return NewMemoryLeaderboardService()
}

func NewGistLeaderboardService(gistID, githubToken string) *GistLeaderboardService {
	return &GistLeaderboardService{
		gistID:      gistID,
		githubToken: githubToken,
		filename:    "leaderboard.json",
	}
}

func (gs *GistLeaderboardService) loadFromGist() (*Leaderboard, error) {
	url := fmt.Sprintf("https://api.github.com/gists/%s", gs.gistID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	if gs.githubToken != "" {
		req.Header.Set("Authorization", "token "+gs.githubToken)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var gist struct {
		Files map[string]struct {
			Content string `json:"content"`
		} `json:"files"`
	}

	if err := json.Unmarshal(body, &gist); err != nil {
		return nil, err
	}

	leaderboard := &Leaderboard{}
	file, exists := gist.Files[gs.filename]
	if exists && file.Content != "" {
		if err := json.Unmarshal([]byte(file.Content), &leaderboard.Entries); err != nil {
			return nil, err
		}
	}

	return leaderboard, nil
}

func (gs *GistLeaderboardService) saveToGist(leaderboard *Leaderboard) error {
	content, err := json.MarshalIndent(leaderboard.Entries, "", "  ")
	if err != nil {
		return err
	}

	payload := map[string]interface{}{
		"files": map[string]interface{}{
			gs.filename: map[string]interface{}{
				"content": string(content),
			},
		},
	}

	jsonPayload, _ := json.Marshal(payload)

	url := fmt.Sprintf("https://api.github.com/gists/%s", gs.gistID)
	req, err := http.NewRequest("PATCH", url, strings.NewReader(string(jsonPayload)))
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "token "+gs.githubToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	return nil
}

func (gs *GistLeaderboardService) AddEntry(userID int64, username, firstName string, score, total int) bool {
	leaderboard, err := gs.loadFromGist()
	if err != nil {
		fmt.Printf("Error loading from gist: %v\n", err)
		return false
	}

	percentage := (score * 100) / total
	newEntry := LeaderboardEntry{
		UserID:     userID,
		Username:   username,
		FirstName:  firstName,
		Score:      score,
		Total:      total,
		Percentage: percentage,
		Date:       time.Now().Format("02.01.2006 15:04"),
	}

	// Ищем существующую запись
	found := false
	for i, entry := range leaderboard.Entries {
		if entry.UserID == userID {
			found = true
			// Обновляем если результат лучше
			if percentage > entry.Percentage || (percentage == entry.Percentage && score > entry.Score) {
				leaderboard.Entries[i] = newEntry
			}
			break
		}
	}

	// Если не нашли - добавляем новую запись
	if !found {
		leaderboard.Entries = append(leaderboard.Entries, newEntry)
	}

	if err := gs.saveToGist(leaderboard); err != nil {
		fmt.Printf("Error saving to gist: %v\n", err)
		return false
	}

	return true
}

func (gs *GistLeaderboardService) GetTop(limit int) []LeaderboardEntry {
	leaderboard, err := gs.loadFromGist()
	if err != nil {
		fmt.Printf("Error loading leaderboard: %v\n", err)
		return nil
	}

	// Сортируем по проценту и количеству очков
	sorted := make([]LeaderboardEntry, len(leaderboard.Entries))
	copy(sorted, leaderboard.Entries)

	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Percentage == sorted[j].Percentage {
			return sorted[i].Score > sorted[j].Score
		}
		return sorted[i].Percentage > sorted[j].Percentage
	})

	if limit > len(sorted) {
		limit = len(sorted)
	}

	return sorted[:limit]
}

func (gs *GistLeaderboardService) GetUserPosition(userID int64) (int, *LeaderboardEntry) {
	top := gs.GetTop(len(gs.GetTop(1000))) // Получаем все записи
	for i, entry := range top {
		if entry.UserID == userID {
			return i + 1, &entry
		}
	}
	return -1, nil
}

// MemoryLeaderboardService - fallback вариант
type MemoryLeaderboardService struct {
	leaderboard *Leaderboard
}

func NewMemoryLeaderboardService() *MemoryLeaderboardService {
	return &MemoryLeaderboardService{
		leaderboard: &Leaderboard{
			Entries: make([]LeaderboardEntry, 0),
		},
	}
}

func (ms *MemoryLeaderboardService) AddEntry(userID int64, username, firstName string, score, total int) bool {
	ms.leaderboard.mu.Lock()
	defer ms.leaderboard.mu.Unlock()

	percentage := (score * 100) / total
	newEntry := LeaderboardEntry{
		UserID:     userID,
		Username:   username,
		FirstName:  firstName,
		Score:      score,
		Total:      total,
		Percentage: percentage,
		Date:       time.Now().Format("02.01.2006 15:04"),
	}

	for i, entry := range ms.leaderboard.Entries {
		if entry.UserID == userID {
			if percentage > entry.Percentage || (percentage == entry.Percentage && score > entry.Score) {
				ms.leaderboard.Entries[i] = newEntry
			}
			return true
		}
	}

	ms.leaderboard.Entries = append(ms.leaderboard.Entries, newEntry)
	return true
}

func (ms *MemoryLeaderboardService) GetTop(limit int) []LeaderboardEntry {
	ms.leaderboard.mu.RLock()
	defer ms.leaderboard.mu.RUnlock()

	sorted := make([]LeaderboardEntry, len(ms.leaderboard.Entries))
	copy(sorted, ms.leaderboard.Entries)

	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Percentage == sorted[j].Percentage {
			return sorted[i].Score > sorted[j].Score
		}
		return sorted[i].Percentage > sorted[j].Percentage
	})

	if limit > len(sorted) {
		limit = len(sorted)
	}

	return sorted[:limit]
}

func (ms *MemoryLeaderboardService) GetUserPosition(userID int64) (int, *LeaderboardEntry) {
	top := ms.GetTop(len(ms.leaderboard.Entries))
	for i, entry := range top {
		if entry.UserID == userID {
			return i + 1, &entry
		}
	}
	return -1, nil
}
