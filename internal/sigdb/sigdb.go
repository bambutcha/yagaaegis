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

// Dir — каталог баз рядом с данными приложения (без root / без /var/lib/clamav).
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
	OK      bool     `json:"ok"`
	Dir     string   `json:"dir"`
	Mirror  string   `json:"mirror"`
	Message string   `json:"message"`
	Files   []string `json:"files,omitempty"`
}

// Update качает CVD с зеркала. Файлы пользователя не уходят в сеть.
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
		files, err := downloadAll(ctx, client, log, mirror, dir)
		if err == nil {
			return UpdateResult{
				OK:      true,
				Dir:     dir,
				Mirror:  mirror,
				Message: "Базы обновлены с зеркала. Ваши файлы никуда не отправлялись.",
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

// SeedFrom копирует CVD из каталога установщика (вшитые базы) в Dir().
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

func downloadAll(ctx context.Context, client *http.Client, log *slog.Logger, mirror, dir string) ([]string, error) {
	var saved []string
	for _, name := range dbFiles {
		url := mirror + "/" + name
		dest := filepath.Join(dir, name)
		tmp := dest + ".tmp"
		if err := downloadFile(ctx, client, url, tmp); err != nil {
			_ = os.Remove(tmp)
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		if err := os.Rename(tmp, dest); err != nil {
			_ = os.Remove(tmp)
			return nil, err
		}
		saved = append(saved, name)
		log.Info("скачан CVD", "file", name, "mirror", mirror)
	}
	return saved, nil
}

func downloadFile(ctx context.Context, client *http.Client, url, dest string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "LocalGuard/0.1 (+https://github.com/bambutcha/localguard)")
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
