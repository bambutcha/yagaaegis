package scanner

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/bambutcha/yagaaegis/internal/runtimebin"
	"github.com/bambutcha/yagaaegis/internal/sigdb"
)

const (
	StatusClean       = "clean"
	StatusCaution     = "caution" // нет сигнатуры вируса, но есть скрипты/макросы в документе
	StatusInfected    = "infected"
	StatusError       = "error"
	StatusUnavailable = "unavailable"
)

// Hit — одна находка при скане папки.
type Hit struct {
	Path   string `json:"path"`
	Threat string `json:"threat"`
	Kind   string `json:"kind,omitempty"` // virus | caution
}

// Result — ответ UI после локального скана.
type Result struct {
	Path          string `json:"path"`
	SHA256        string `json:"sha256,omitempty"`
	Status        string `json:"status"`
	Threat        string `json:"threat,omitempty"`
	Hits          []Hit  `json:"hits,omitempty"`
	IsDir         bool   `json:"isDir"`
	Engine        string `json:"engine"`
	ScannedAt     string `json:"scannedAt"`
	DurationMs    int64  `json:"durationMs"`
	FilesScanned  int    `json:"filesScanned,omitempty"`
	InfectedCount int    `json:"infectedCount,omitempty"`
	Message       string `json:"message,omitempty"`
	Offline       bool   `json:"offline"`
}

// Engine вызывает clamscan / clamdscan. Сначала вшитый runtime, потом PATH.
type Engine struct {
	Binary      string
	Clamd       string
	Clamdscan   string
	DatabaseDir string
	CertsDir    string
	log         *slog.Logger

	clamdMu    sync.Mutex
	clamdCmd   *exec.Cmd
	socketPath string
	confPath   string
	warmOnce   sync.Once
}

// DetectEngine находит движок и каталог баз приложения.
func DetectEngine(log *slog.Logger) (*Engine, error) {
	if log == nil {
		log = slog.Default()
	}

	if seed := runtimebin.SeedDir(); seed != "" {
		if err := sigdb.SeedFrom(seed); err != nil {
			log.Debug("seed баз пропущен", "err", err)
		}
	}

	dbDir, err := sigdb.Dir()
	if err != nil {
		return nil, err
	}

	binary, err := runtimebin.ClamscanPath()
	if err != nil {
		for _, name := range []string{"clamscan", "clamdscan"} {
			path, lookErr := exec.LookPath(name)
			if lookErr == nil {
				binary = path
				err = nil
				break
			}
		}
	}
	if binary == "" {
		return nil, errors.New("движок не найден: нужен вшитый runtime/…/clamscan или системный ClamAV")
	}

	eng := &Engine{
		Binary:      binary,
		DatabaseDir: dbDir,
		CertsDir:    certsDirNextTo(binary),
		log:         log,
	}
	if !sigdb.HasDatabases(dbDir) {
		log.Warn("базы сигнатур отсутствуют", "dir", dbDir)
	}
	log.Info("движок готов", "binary", binary, "db", dbDir, "certs", eng.CertsDir)
	return eng, nil
}

func certsDirNextTo(binary string) string {
	dir := filepath.Join(filepath.Dir(binary), "certs")
	if st, err := os.Stat(dir); err == nil && st.IsDir() {
		return dir
	}
	return ""
}

func (e *Engine) clamEnv() []string {
	libDir := filepath.Dir(e.Binary)
	env := append(os.Environ(), "LD_LIBRARY_PATH="+libDir+string(os.PathListSeparator)+os.Getenv("LD_LIBRARY_PATH"))
	if e.CertsDir != "" {
		env = append(env, "CVD_CERTS_DIR="+e.CertsDir)
	}
	return env
}

func (e *Engine) Version(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, e.Binary, "--version")
	cmd.Dir = filepath.Dir(e.Binary)
	cmd.Env = e.clamEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		detail := strings.TrimSpace(string(out))
		if detail != "" {
			return "", fmt.Errorf("версия clamav: %v (%s)", err, firstLine(detail))
		}
		return "", fmt.Errorf("версия clamav: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func FileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// ScanPath проверяет файл или папку (рекурсивно) только локально.
func (e *Engine) ScanPath(ctx context.Context, path string) Result {
	start := time.Now()
	abs, err := filepath.Abs(path)
	res := Result{
		Path:      abs,
		Engine:    filepath.Base(e.Binary),
		ScannedAt: time.Now().UTC().Format(time.RFC3339),
		Offline:   true,
	}
	if err != nil {
		res.Status = StatusError
		res.Message = err.Error()
		return res
	}

	info, err := os.Stat(abs)
	if err != nil {
		res.Status = StatusError
		res.Message = err.Error()
		return res
	}
	res.IsDir = info.IsDir()

	if e.DatabaseDir == "" || !sigdb.HasDatabases(e.DatabaseDir) {
		res.Status = StatusUnavailable
		res.Message = "Нет баз сигнатур. Нажмите «Обновить базы» или переустановите с seed."
		return res
	}

	if !res.IsDir {
		hash, err := FileSHA256(abs)
		if err != nil {
			res.Status = StatusError
			res.Message = fmt.Sprintf("хэш: %v", err)
			return res
		}
		res.SHA256 = hash
	}

	// эвристика документов параллельно с ClamAV
	type riskOut struct {
		risks []Risk
		err   error
	}
	riskCh := make(chan riskOut, 1)
	go func() {
		r, err := InspectDocRisks(abs)
		riskCh <- riskOut{r, err}
	}()

	var (
		out       []byte
		scanErr   error
		usedClamd bool
	)
	if e.PreferClamd() {
		out, scanErr = e.scanViaClamd(ctx, abs, res.IsDir)
		if scanErr == nil || exitCode(scanErr) == 1 {
			usedClamd = true
			res.Engine = "clamdscan"
		} else if e.log != nil {
			e.log.Warn("clamd недоступен, fallback clamscan", "err", scanErr)
		}
	}
	if !usedClamd {
		args := []string{"--infected", "-d", e.DatabaseDir}
		if e.CertsDir != "" {
			args = append(args, "--cvdcertsdir="+e.CertsDir)
		}
		if res.IsDir {
			args = append(args, "-r")
		}
		args = append(args, abs)
		cmd := exec.CommandContext(ctx, e.Binary, args...)
		cmd.Dir = filepath.Dir(e.Binary)
		cmd.Env = e.clamEnv()
		out, scanErr = cmd.CombinedOutput()
	}
	res.DurationMs = time.Since(start).Milliseconds()
	text := strings.TrimSpace(string(out))
	res.FilesScanned, res.InfectedCount = parseScanCounts(text)
	if res.FilesScanned == 0 {
		res.FilesScanned = countScanTargets(abs, res.IsDir)
	}
	pre := <-riskCh
	var pred []Risk
	if pre.err == nil {
		pred = pre.risks
	}

	if scanErr == nil {
		if res.FilesScanned == 0 && !res.IsDir {
			res.FilesScanned = 1
		}
		applyDocCaution(&res, pred)
		return res
	}

	var exitErr *exec.ExitError
	if errors.As(scanErr, &exitErr) {
		switch exitErr.ExitCode() {
		case 1:
			res.Status = StatusInfected
			res.Hits = parseHits(text)
			if res.InfectedCount == 0 {
				res.InfectedCount = len(res.Hits)
			}
			if len(res.Hits) > 0 {
				res.Threat = folderThreatLabel(virusHits(res.Hits))
			} else {
				res.Threat = parseThreat(text, abs)
			}
			mergeDocCautions(&res, pred)
			res.Message = infectedMessage(res)
			return res
		default:
			res.Status = StatusError
			res.Message = firstLine(text)
			if res.Message == "" {
				res.Message = scanErr.Error()
			}
			return res
		}
	}

	res.Status = StatusError
	res.Message = scanErr.Error()
	return res
}

func exitCode(err error) int {
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode()
	}
	return -1
}

// ScanFile — алиас для одного файла (тесты / совместимость).
func (e *Engine) ScanFile(ctx context.Context, path string) Result {
	return e.ScanPath(ctx, path)
}

// applyDocCaution: ClamAV чист, но документ содержит скрипты/макросы → предупреждение.
func applyDocCaution(res *Result, pred []Risk) {
	n := mergeDocCautions(res, pred)
	if n == 0 {
		res.Status = StatusClean
		res.Message = cleanMessage(*res)
		return
	}
	res.Status = StatusCaution
	res.Message = risksSummary(hitsToRisks(res.Hits))
	if res.FilesScanned > 0 {
		res.Message = fmt.Sprintf("Проверено файлов: %d. %s", res.FilesScanned, res.Message)
	}
	if res.Threat == "" {
		for _, h := range res.Hits {
			if h.Kind == "caution" {
				res.Threat = h.Threat
				break
			}
		}
	}
}

// mergeDocCautions добавляет caution-хиты (в т.ч. к infected-папке). Возвращает число добавленных.
func mergeDocCautions(res *Result, pred []Risk) int {
	risks := pred
	if len(risks) == 0 {
		var err error
		risks, err = InspectDocRisks(res.Path)
		if err != nil || len(risks) == 0 {
			return 0
		}
	}
	infected := map[string]struct{}{}
	for _, h := range res.Hits {
		if h.Kind == "" || h.Kind == "virus" {
			infected[h.Path] = struct{}{}
		}
	}
	added := 0
	for _, r := range risks {
		p := r.Path
		if p == "" {
			p = res.Path
		}
		if _, ok := infected[p]; ok {
			continue
		}
		res.Hits = append(res.Hits, Hit{Path: p, Threat: r.Detail, Kind: "caution"})
		added++
	}
	return added
}

func hitsToRisks(hits []Hit) []Risk {
	var out []Risk
	for _, h := range hits {
		if h.Kind == "caution" {
			out = append(out, Risk{Path: h.Path, Detail: h.Threat})
		}
	}
	return out
}

func virusHits(hits []Hit) []Hit {
	var out []Hit
	for _, h := range hits {
		if h.Kind == "" || h.Kind == "virus" {
			out = append(out, h)
		}
	}
	return out
}

func cautionCount(hits []Hit) int {
	n := 0
	for _, h := range hits {
		if h.Kind == "caution" {
			n++
		}
	}
	return n
}

func cleanMessage(res Result) string {
	if res.IsDir {
		if res.FilesScanned > 0 {
			return fmt.Sprintf("Проверено файлов: %d — угроз не найдено", res.FilesScanned)
		}
		return "В папке угроз не найдено"
	}
	if res.FilesScanned > 0 {
		return fmt.Sprintf("Проверено файлов: %d — угроз не найдено (локальный движок)", res.FilesScanned)
	}
	return "Угроз не найдено (локальный движок)"
}

func infectedMessage(res Result) string {
	n := res.InfectedCount
	if n == 0 {
		n = len(virusHits(res.Hits))
	}
	caut := cautionCount(res.Hits)
	if res.FilesScanned > 0 {
		clean := res.FilesScanned - n
		if clean < 0 {
			clean = 0
		}
		msg := fmt.Sprintf("Проверено файлов: %d · угроз: %d · без вируса: %d", res.FilesScanned, n, clean)
		if caut > 0 {
			msg += fmt.Sprintf(" · со скриптами/макросами: %d", caut)
		}
		return msg
	}
	if res.IsDir {
		msg := fmt.Sprintf("Обнаружено угроз: %d", n)
		if caut > 0 {
			msg += fmt.Sprintf(" · скрипты/макросы: %d", caut)
		}
		return msg
	}
	return "Обнаружена угроза"
}

// folderThreatLabel — для папки не подменять все находки первой сигнатурой.
func folderThreatLabel(hits []Hit) string {
	if len(hits) == 0 {
		return ""
	}
	uniq := map[string]struct{}{}
	var order []string
	for _, h := range hits {
		if _, ok := uniq[h.Threat]; ok {
			continue
		}
		uniq[h.Threat] = struct{}{}
		order = append(order, h.Threat)
	}
	if len(order) == 1 {
		if len(hits) == 1 {
			return order[0]
		}
		return fmt.Sprintf("%s (×%d)", order[0], len(hits))
	}
	if len(order) <= 3 {
		return strings.Join(order, " · ")
	}
	return fmt.Sprintf("%s · %s и ещё %d", order[0], order[1], len(order)-2)
}

func parseScanCounts(output string) (scanned, infected int) {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "Scanned files:"):
			fmt.Sscanf(line, "Scanned files: %d", &scanned)
		case strings.HasPrefix(line, "Infected files:"):
			fmt.Sscanf(line, "Infected files: %d", &infected)
		}
	}
	return scanned, infected
}

func countScanTargets(path string, isDir bool) int {
	if !isDir {
		return 1
	}
	n := 0
	_ = filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		n++
		if n > 10000 {
			return filepath.SkipAll
		}
		return nil
	})
	return n
}

func parseHits(output string) []Hit {
	var hits []Hit
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasSuffix(line, "FOUND") {
			continue
		}
		// /path/to/file: Threat.Name FOUND
		i := strings.LastIndex(line, ": ")
		if i < 0 {
			continue
		}
		p := strings.TrimSpace(line[:i])
		threat := strings.TrimSpace(strings.TrimSuffix(line[i+2:], "FOUND"))
		hits = append(hits, Hit{Path: p, Threat: threat, Kind: "virus"})
	}
	return hits
}

func parseThreat(output, path string) string {
	hits := parseHits(output)
	for _, h := range hits {
		if h.Path == path || strings.HasSuffix(h.Path, path) {
			return h.Threat
		}
	}
	if len(hits) > 0 {
		return hits[0].Threat
	}
	return "unknown"
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return s
}
