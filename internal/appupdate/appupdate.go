package appupdate

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/bambutcha/localguard/internal/version"
)

const githubAPI = "https://api.github.com/repos/bambutcha/localguard/releases/latest"

// Result — ответ проверки обновлений приложения (не баз ClamAV).
type Result struct {
	OK          bool   `json:"ok"`
	Current     string `json:"current"`
	Latest      string `json:"latest,omitempty"`
	HasUpdate   bool   `json:"hasUpdate"`
	URL         string `json:"url,omitempty"`
	Message     string `json:"message"`
	CheckedAt   string `json:"checkedAt"`
}

type ghRelease struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
	Draft   bool   `json:"draft"`
}

// Check сравнивает version.String с latest GitHub Release.
func Check(ctx context.Context) Result {
	now := time.Now().UTC().Format(time.RFC3339)
	cur := version.String
	out := Result{Current: cur, CheckedAt: now}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, githubAPI, nil)
	if err != nil {
		out.Message = err.Error()
		return out
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "LocalGuard/"+cur)

	client := &http.Client{Timeout: 20 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		out.Message = "нет сети или GitHub недоступен: " + err.Error()
		return out
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotFound {
		out.OK = true
		out.Message = "релизов на GitHub пока нет"
		return out
	}
	if res.StatusCode != http.StatusOK {
		out.Message = fmt.Sprintf("GitHub API HTTP %d", res.StatusCode)
		return out
	}

	var rel ghRelease
	if err := json.NewDecoder(res.Body).Decode(&rel); err != nil {
		out.Message = err.Error()
		return out
	}
	if rel.Draft {
		out.OK = true
		out.Message = "актуальный релиз — черновик, пропуск"
		return out
	}

	latest := strings.TrimPrefix(strings.TrimSpace(rel.TagName), "v")
	out.OK = true
	out.Latest = latest
	out.URL = rel.HTMLURL
	out.HasUpdate = compareSemVer(latest, cur) > 0
	if out.HasUpdate {
		out.Message = fmt.Sprintf("Доступна %s (сейчас %s)", versionLabel(latest), version.Label())
	} else {
		out.Message = "Установлена актуальная версия " + version.Label()
	}
	return out
}

func versionLabel(s string) string {
	if strings.HasPrefix(s, "v") {
		return s
	}
	return "v" + s
}

// compareSemVer: 1 если a>b, -1 если a<b, 0 если равны (грубое сравнение MAJOR.MINOR.PATCH, суффиксы игнор).
func compareSemVer(a, b string) int {
	pa := parseCore(a)
	pb := parseCore(b)
	for i := 0; i < 3; i++ {
		if pa[i] > pb[i] {
			return 1
		}
		if pa[i] < pb[i] {
			return -1
		}
	}
	return 0
}

func parseCore(s string) [3]int {
	s = strings.TrimPrefix(s, "v")
	if i := strings.IndexAny(s, "-+"); i >= 0 {
		s = s[:i]
	}
	var out [3]int
	parts := strings.Split(s, ".")
	for i := 0; i < 3 && i < len(parts); i++ {
		n := 0
		for _, c := range parts[i] {
			if c < '0' || c > '9' {
				break
			}
			n = n*10 + int(c-'0')
		}
		out[i] = n
	}
	return out
}
