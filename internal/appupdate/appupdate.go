package appupdate

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/bambutcha/localguard/internal/version"
)

const githubAPI = "https://api.github.com/repos/bambutcha/localguard/releases/latest"

// Result — ответ проверки / установки обновлений приложения (не баз ClamAV).
type Result struct {
	OK           bool   `json:"ok"`
	Current      string `json:"current"`
	Latest       string `json:"latest,omitempty"`
	HasUpdate    bool   `json:"hasUpdate"`
	URL          string `json:"url,omitempty"`
	AssetURL     string `json:"assetUrl,omitempty"`
	AssetName    string `json:"assetName,omitempty"`
	Message      string `json:"message"`
	CheckedAt    string `json:"checkedAt"`
	NeedRestart  bool   `json:"needRestart,omitempty"`
}

type ghAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

type ghRelease struct {
	TagName string    `json:"tag_name"`
	HTMLURL string    `json:"html_url"`
	Draft   bool      `json:"draft"`
	Assets  []ghAsset `json:"assets"`
}

// Check сравнивает version.String с latest GitHub Release и подбирает asset для ОС.
func Check(ctx context.Context) Result {
	now := time.Now().UTC().Format(time.RFC3339)
	cur := version.String
	out := Result{Current: cur, CheckedAt: now}

	rel, err := fetchLatest(ctx)
	if err != nil {
		out.Message = err.Error()
		return out
	}
	if rel == nil {
		out.OK = true
		out.Message = "релизов на GitHub пока нет"
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
	if name, url := pickAsset(rel.Assets); url != "" {
		out.AssetName = name
		out.AssetURL = url
	}
	if out.HasUpdate {
		out.Message = fmt.Sprintf("Доступна %s (сейчас %s)", versionLabel(latest), version.Label())
		if out.AssetURL == "" {
			out.Message += " — скачайте вручную со страницы релиза"
		}
	} else {
		out.Message = "Установлена актуальная версия " + version.Label()
	}
	return out
}

// Apply скачивает portable-пакет и распаковывает поверх installDir.
// Текущий YagaAegis переименовывается в .old — нужен перезапуск.
func Apply(ctx context.Context, installDir string) Result {
	res := Check(ctx)
	if !res.OK {
		return res
	}
	if !res.HasUpdate {
		res.Message = "обновление не требуется"
		return res
	}
	if res.AssetURL == "" {
		res.OK = false
		res.Message = "нет подходящего архива для " + runtime.GOOS + "/" + runtime.GOARCH
		return res
	}
	installDir = filepath.Clean(installDir)
	if installDir == "" || installDir == "." {
		res.OK = false
		res.Message = "пустой каталог установки"
		return res
	}

	tmp, err := os.MkdirTemp("", "yagaaegis-upd-*")
	if err != nil {
		res.OK = false
		res.Message = err.Error()
		return res
	}
	defer os.RemoveAll(tmp)

	archive := filepath.Join(tmp, res.AssetName)
	if err := downloadFile(ctx, res.AssetURL, archive); err != nil {
		res.OK = false
		res.Message = "скачивание: " + err.Error()
		return res
	}

	extractDir := filepath.Join(tmp, "extract")
	if err := os.MkdirAll(extractDir, 0o755); err != nil {
		res.OK = false
		res.Message = err.Error()
		return res
	}
	if strings.HasSuffix(strings.ToLower(res.AssetName), ".zip") {
		err = unzip(archive, extractDir)
	} else {
		err = untarGz(archive, extractDir)
	}
	if err != nil {
		res.OK = false
		res.Message = "распаковка: " + err.Error()
		return res
	}

	payload, err := findPayloadRoot(extractDir)
	if err != nil {
		res.OK = false
		res.Message = err.Error()
		return res
	}

	if err := mergePayload(payload, installDir); err != nil {
		res.OK = false
		res.Message = "установка файлов: " + err.Error()
		return res
	}

	res.OK = true
	res.NeedRestart = true
	res.HasUpdate = false
	res.Message = fmt.Sprintf("Установлена %s. Закройте YagaAegis и запустите снова.", versionLabel(res.Latest))
	return res
}

func fetchLatest(ctx context.Context) (*ghRelease, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, githubAPI, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "YagaAegis/"+version.String)

	client := &http.Client{Timeout: 45 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("нет сети или GitHub недоступен: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API HTTP %d", res.StatusCode)
	}
	var rel ghRelease
	if err := json.NewDecoder(res.Body).Decode(&rel); err != nil {
		return nil, err
	}
	return &rel, nil
}

func pickAsset(assets []ghAsset) (name, url string) {
	wantZip := runtime.GOOS == "windows"
	platform := runtime.GOOS + "-" + runtime.GOARCH // linux-amd64 / windows-amd64
	for _, a := range assets {
		n := strings.ToLower(a.Name)
		if strings.Contains(n, "installer") {
			continue
		}
		if !strings.Contains(n, platform) && !strings.Contains(n, runtime.GOOS) {
			continue
		}
		if wantZip && strings.HasSuffix(n, ".zip") {
			return a.Name, a.BrowserDownloadURL
		}
		if !wantZip && (strings.HasSuffix(n, ".tar.gz") || strings.HasSuffix(n, ".tgz")) {
			return a.Name, a.BrowserDownloadURL
		}
	}
	return "", ""
}

func downloadFile(ctx context.Context, url, dest string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "YagaAegis/"+version.String)
	client := &http.Client{Timeout: 30 * time.Minute}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", res.StatusCode)
	}
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, res.Body)
	return err
}

func findPayloadRoot(extractDir string) (string, error) {
	// архив: linux-amd64/YagaAegis или сразу файлы
	candidates := []string{
		filepath.Join(extractDir, "linux-amd64"),
		filepath.Join(extractDir, "windows-amd64"),
		extractDir,
	}
	for _, c := range candidates {
		if hasAppBinary(c) {
			return c, nil
		}
	}
	// один подкаталог
	entries, err := os.ReadDir(extractDir)
	if err != nil {
		return "", err
	}
	for _, e := range entries {
		if e.IsDir() {
			p := filepath.Join(extractDir, e.Name())
			if hasAppBinary(p) {
				return p, nil
			}
		}
	}
	return "", fmt.Errorf("в архиве не найден YagaAegis")
}

func hasAppBinary(dir string) bool {
	for _, n := range []string{"YagaAegis", "YagaAegis.exe"} {
		if st, err := os.Stat(filepath.Join(dir, n)); err == nil && !st.IsDir() {
			return true
		}
	}
	return false
}

func mergePayload(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		base := filepath.Base(path)
		// запущенный бинарник — пишем во временный и атомарно подменяем
		if base == "YagaAegis" || strings.EqualFold(base, "YagaAegis.exe") {
			return replaceRunningBinary(path, target)
		}
		return copyFile(path, target, info.Mode())
	})
}

func replaceRunningBinary(src, dst string) error {
	dir := filepath.Dir(dst)
	tmp := filepath.Join(dir, filepath.Base(dst)+".new")
	old := dst + ".old"
	if err := copyFile(src, tmp, 0o755); err != nil {
		return err
	}
	_ = os.Remove(old)
	if _, err := os.Stat(dst); err == nil {
		if err := os.Rename(dst, old); err != nil {
			// Windows может держать lock — оставим .new рядом
			return fmt.Errorf("не удалось убрать старый бинарник (закройте приложение?): %w", err)
		}
	}
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.Rename(old, dst)
		return err
	}
	return nil
}

func copyFile(src, dst string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Chmod(mode)
}

func untarGz(archive, dest string) error {
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		target := filepath.Join(dest, hdr.Name)
		if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(dest)+string(os.PathSeparator)) &&
			filepath.Clean(target) != filepath.Clean(dest) {
			return fmt.Errorf("небезопасный путь в архиве: %s", hdr.Name)
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			out.Close()
		}
	}
	return nil
}

func unzip(archive, dest string) error {
	r, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer r.Close()
	for _, f := range r.File {
		target := filepath.Join(dest, f.Name)
		if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("небезопасный путь в zip: %s", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, f.Mode())
		if err != nil {
			rc.Close()
			return err
		}
		_, err = io.Copy(out, rc)
		out.Close()
		rc.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func versionLabel(s string) string {
	if strings.HasPrefix(s, "v") {
		return s
	}
	return "v" + s
}

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
