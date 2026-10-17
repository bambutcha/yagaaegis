package sigdb

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/bambutcha/localguard/internal/version"
)

// Зеркала CVD. Официальный CDN Cisco из РФ часто отвечает 403.
var defaultMirrors = []string{
	"https://packages.microsoft.com/clamav",
}

var dbFiles = []string{
	"main.cvd",
	"daily.cvd",
	"bytecode.cvd",
}

// Dir — каталог баз рядом с данными приложения.
func Dir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		home, herr := os.UserHomeDir()
		if herr != nil {
			return "", err
		}
		base = filepath.Join(home, ".config")
	}
	dir := filepath.Join(base, "localguard", "clamav")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// HasDatabases проверяет наличие CVD.
func HasDatabases(dir string) bool {
	for _, name := range dbFiles {
		if st, err := os.Stat(filepath.Join(dir, name)); err == nil && st.Size() > 0 {
			return true
		}
	}
	return false
}

// UpdateResult — итог обновления баз.
type UpdateResult struct {
	OK        bool     `json:"ok"`
	Dir       string   `json:"dir"`
	Mirror    string   `json:"mirror"`
	Message   string   `json:"message"`
	Files     []string `json:"files,omitempty"`
	UpToDate  bool     `json:"upToDate"`
	Changed   int      `json:"changed"`
}

// Update качает только изменившиеся CVD (по Content-Length с HEAD).
func Update(ctx context.Context, log *slog.Logger, mirrors []string) UpdateResult {
	if log == nil {
		log = slog.Default()
	}
	if len(mirrors) == 0 {
		mirrors = defaultMirrors
	}
	dir, err := Dir()
	if err != nil {
		return UpdateResult{OK: false, Message: err.Error()}
	}

	client := &http.Client{Timeout: 15 * time.Minute}
	var lastErr error
	for _, mirror := range mirrors {
		files, changed, err := syncAll(ctx, client, log, mirror, dir)
		if err == nil {
			if changed == 0 {
				return UpdateResult{
					OK:       true,
					Dir:      dir,
					Mirror:   mirror,
					UpToDate: true,
					Changed:  0,
					Message:  "Базы уже актуальны — скачивать нечего.",
					Files:    files,
				}
			}
			return UpdateResult{
				OK:      true,
				Dir:     dir,
				Mirror:  mirror,
				Changed: changed,
				Message: fmt.Sprintf("Обновлено файлов: %d. Ваши документы никуда не отправлялись.", changed),
				Files:   files,
			}
		}
		lastErr = err
		log.Warn("зеркало недоступно", "mirror", mirror, "err", err)
	}
	msg := "не удалось обновить базы"
	if lastErr != nil {
		msg = lastErr.Error()
	}
	return UpdateResult{OK: false, Dir: dir, Message: msg}
}

// SeedFrom копирует CVD из каталога установщика в Dir().
func SeedFrom(srcDir string) error {
	if srcDir == "" {
		return fmt.Errorf("пустой src")
	}
	dst, err := Dir()
	if err != nil {
		return err
	}
	if HasDatabases(dst) {
		return nil
	}
	copied := 0
	for _, name := range dbFiles {
		src := filepath.Join(srcDir, name)
		if _, err := os.Stat(src); err != nil {
			continue
		}
		in, err := os.Open(src)
		if err != nil {
			return err
		}
		outPath := filepath.Join(dst, name)
		out, err := os.Create(outPath)
		if err != nil {
			in.Close()
			return err
		}
		_, err = io.Copy(out, in)
		in.Close()
		out.Close()
		if err != nil {
			return err
		}
		copied++
	}
	if copied == 0 {
		return fmt.Errorf("во вшитом каталоге нет CVD: %s", srcDir)
	}
	return nil
}

func syncAll(ctx context.Context, client *http.Client, log *slog.Logger, mirror, dir string) (present []string, changed int, err error) {
	for _, name := range dbFiles {
		url := mirror + "/" + name
		dest := filepath.Join(dir, name)
		need, remoteLen, err := needsDownload(ctx, client, url, dest)
		if err != nil {
			return nil, 0, fmt.Errorf("%s: %w", name, err)
		}
		if !need {
			present = append(present, name)
			log.Info("CVD без изменений", "file", name, "size", remoteLen)
			continue
		}
		tmp := dest + ".tmp"
		if err := downloadFile(ctx, client, url, tmp); err != nil {
			_ = os.Remove(tmp)
			return nil, 0, fmt.Errorf("%s: %w", name, err)
		}
		if err := os.Rename(tmp, dest); err != nil {
			_ = os.Remove(tmp)
			return nil, 0, err
		}
		present = append(present, name)
		changed++
		log.Info("скачан CVD", "file", name, "mirror", mirror)
	}
	return present, changed, nil
}

func needsDownload(ctx context.Context, client *http.Client, url, dest string) (bool, int64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return false, 0, err
	}
	req.Header.Set("User-Agent", "LocalGuard/"+version.String+" (+https://github.com/bambutcha/localguard)")
	res, err := client.Do(req)
	if err != nil {
		return false, 0, err
	}
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		// некоторые зеркала плохо отдают HEAD — тогда качаем
		if res.StatusCode == http.StatusMethodNotAllowed || res.StatusCode == http.StatusNotImplemented {
			return true, 0, nil
		}
		return false, 0, fmt.Errorf("HTTP %d", res.StatusCode)
	}
	remote := res.ContentLength
	st, err := os.Stat(dest)
	if err != nil {
		return true, remote, nil
	}
	if remote > 0 && st.Size() == remote {
		return false, remote, nil
	}
	return true, remote, nil
}

func downloadFile(ctx context.Context, client *http.Client, url, dest string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "LocalGuard/"+version.String+" (+https://github.com/bambutcha/localguard)")
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
