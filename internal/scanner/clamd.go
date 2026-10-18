package scanner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bambutcha/yagaaegis/internal/winproc"
)

// errScanInfected — итог параллельного скана папки с находками (как exit 1 у clamdscan).
var errScanInfected = errors.New("clam: infected")

// Управление локальным clamd: базы грузятся один раз → повторные сканы заметно быстрее.
// После закрытия UI демон можно оставить живым — следующий запуск приложения без повторной загрузки CVD.

func (e *Engine) clamdscanPath() string {
	if e.Clamdscan != "" {
		return e.Clamdscan
	}
	dir := filepath.Dir(e.Binary)
	name := "clamdscan"
	if runtime.GOOS == "windows" {
		name = "clamdscan.exe"
	}
	p := filepath.Join(dir, name)
	if st, err := os.Stat(p); err == nil && !st.IsDir() {
		e.Clamdscan = p
		return p
	}
	if p, err := exec.LookPath(name); err == nil {
		e.Clamdscan = p
		return p
	}
	return ""
}

func (e *Engine) clamdPath() string {
	if e.Clamd != "" {
		return e.Clamd
	}
	dir := filepath.Dir(e.Binary)
	name := "clamd"
	if runtime.GOOS == "windows" {
		name = "clamd.exe"
	}
	p := filepath.Join(dir, name)
	if st, err := os.Stat(p); err == nil && !st.IsDir() {
		e.Clamd = p
		return p
	}
	if p, err := exec.LookPath(name); err == nil {
		e.Clamd = p
		return p
	}
	return ""
}

func (e *Engine) initClamdPaths() error {
	base, err := os.UserConfigDir()
	if err != nil {
		base = os.TempDir()
	}
	dir := filepath.Join(base, "yagaaegis", "clamd")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	e.confPath = filepath.Join(dir, "clamd.conf")
	e.pidPath = filepath.Join(dir, "clamd.pid")
	if runtime.GOOS == "windows" {
		e.socketPath = "127.0.0.1:3318"
	} else {
		e.socketPath = filepath.Join(dir, "clamd.sock")
	}
	return nil
}

func (e *Engine) ensureClamd(ctx context.Context) error {
	e.clamdMu.Lock()
	defer e.clamdMu.Unlock()

	if e.clamdscanPath() == "" || e.clamdPath() == "" {
		return fmt.Errorf("clamd/clamdscan нет в runtime")
	}
	if err := e.initClamdPaths(); err != nil {
		return err
	}

	desired := e.desiredClamdConf()
	if e.socketReady() {
		if old, err := os.ReadFile(e.confPath); err == nil && string(old) == desired {
			if e.log != nil {
				e.log.Info("clamd уже работает (тёплый)", "socket", e.socketPath)
			}
			return nil
		}
		// конфиг сменился (лимиты и т.п.) — перезапуск
		e.killClamdLocked()
		time.Sleep(300 * time.Millisecond)
	}

	if runtime.GOOS != "windows" {
		_ = os.Remove(e.socketPath)
	}
	if err := os.WriteFile(e.confPath, []byte(desired), 0o644); err != nil {
		return err
	}

	cmd := exec.CommandContext(context.Background(), e.clamdPath(), "-c", e.confPath)
	cmd.Dir = filepath.Dir(e.clamdPath())
	cmd.Env = e.clamEnv()
	cmd.Stdout = nil
	cmd.Stderr = nil
	winproc.HideConsole(cmd)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("старт clamd: %w", err)
	}
	e.clamdCmd = cmd
	// демон в фоне (Foreground false) — не держим Wait в UI-процессе
	go func() { _ = cmd.Wait() }()

	deadline := time.Now().Add(45 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			e.killClamdLocked()
			return ctx.Err()
		default:
		}
		if e.socketReady() {
			if e.log != nil {
				e.log.Info("clamd готов", "socket", e.socketPath)
			}
			return nil
		}
		time.Sleep(150 * time.Millisecond)
	}
	e.killClamdLocked()
	return fmt.Errorf("clamd не поднялся за 45с")
}

func (e *Engine) socketReady() bool {
	if e.socketPath == "" {
		return false
	}
	network := "unix"
	addr := e.socketPath
	if runtime.GOOS == "windows" || strings.Contains(e.socketPath, ":") {
		network = "tcp"
		addr = e.socketPath
	}
	c, err := net.DialTimeout(network, addr, 200*time.Millisecond)
	if err != nil {
		return false
	}
	_ = c.Close()
	return true
}

func (e *Engine) killClamdLocked() {
	if e.clamdCmd != nil && e.clamdCmd.Process != nil {
		_ = e.clamdCmd.Process.Kill()
		e.clamdCmd = nil
	}
	if b, err := os.ReadFile(e.pidPath); err == nil {
		if pid, err := strconv.Atoi(strings.TrimSpace(string(b))); err == nil && pid > 1 {
			if p, err := os.FindProcess(pid); err == nil {
				_ = p.Kill()
			}
		}
	}
	if e.socketPath != "" && !strings.Contains(e.socketPath, ":") {
		_ = os.Remove(e.socketPath)
	}
	_ = os.Remove(e.pidPath)
}

// DetachClamd — при закрытии UI не убиваем демон (следующий старт тёплый).
func (e *Engine) DetachClamd() {
	e.clamdMu.Lock()
	defer e.clamdMu.Unlock()
	e.clamdCmd = nil // процесс живёт сам по PidFile
}

// StopClamd убивает демон (uninstall / явный сброс).
func (e *Engine) StopClamd() {
	e.clamdMu.Lock()
	defer e.clamdMu.Unlock()
	_ = e.initClamdPaths()
	e.killClamdLocked()
}

// StopPersistentClamd — для Uninstall без Engine.
func StopPersistentClamd() {
	base, err := os.UserConfigDir()
	if err != nil {
		return
	}
	dir := filepath.Join(base, "yagaaegis", "clamd")
	pidPath := filepath.Join(dir, "clamd.pid")
	sock := filepath.Join(dir, "clamd.sock")
	if b, err := os.ReadFile(pidPath); err == nil {
		if pid, err := strconv.Atoi(strings.TrimSpace(string(b))); err == nil && pid > 1 {
			if p, err := os.FindProcess(pid); err == nil {
				_ = p.Kill()
			}
		}
	}
	_ = os.Remove(sock)
	_ = os.Remove(pidPath)
}

func (e *Engine) scanViaClamd(ctx context.Context, abs string, isDir bool) (out []byte, err error) {
	if err := e.ensureClamd(ctx); err != nil {
		return nil, err
	}
	if isDir {
		return e.clamdscanDirParallel(ctx, abs)
	}
	return e.clamdscanOne(ctx, abs)
}

func (e *Engine) clamdscanOne(ctx context.Context, path string) ([]byte, error) {
	args := []string{"--fdpass", "--infected", "--no-summary", "--config-file=" + e.confPath, path}
	cmd := exec.CommandContext(ctx, e.clamdscanPath(), args...)
	cmd.Dir = filepath.Dir(e.clamdscanPath())
	cmd.Env = e.clamEnv()
	winproc.HideConsole(cmd)
	return cmd.CombinedOutput()
}

// clamdscanDirParallel — горутины шлют файлы в clamd параллельно (лимит = MaxThreads).
func (e *Engine) clamdscanDirParallel(ctx context.Context, root string) ([]byte, error) {
	files, err := listScanFiles(root, 50000)
	if err != nil {
		return nil, err
	}
	workers := scanWorkerCount()
	jobs := make(chan string, workers*2)
	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		found    bytes.Buffer
		hardErr  error
		infected atomic.Int32
		done     atomic.Bool
	)

	worker := func() {
		defer wg.Done()
		for path := range jobs {
			if done.Load() {
				return
			}
			out, err := e.clamdscanOne(ctx, path)
			code := exitCode(err)
			if code == 1 {
				infected.Add(1)
				mu.Lock()
				found.Write(out)
				if len(out) > 0 && out[len(out)-1] != '\n' {
					found.WriteByte('\n')
				}
				mu.Unlock()
				continue
			}
			if err != nil && code != 0 {
				mu.Lock()
				if hardErr == nil {
					hardErr = fmt.Errorf("%s: %w", path, err)
					done.Store(true)
				}
				mu.Unlock()
				return
			}
		}
	}

	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go worker()
	}
send:
	for _, f := range files {
		if done.Load() {
			break
		}
		select {
		case <-ctx.Done():
			hardErr = ctx.Err()
			done.Store(true)
			break send
		case jobs <- f:
		}
	}
	close(jobs)
	wg.Wait()

	if hardErr != nil {
		return found.Bytes(), hardErr
	}
	nInf := int(infected.Load())
	summary := fmt.Sprintf("\n----------- SCAN SUMMARY -----------\nScanned files: %d\nInfected files: %d\n", len(files), nInf)
	out := append(found.Bytes(), []byte(summary)...)
	if nInf > 0 {
		return out, errScanInfected
	}
	return out, nil
}

func listScanFiles(root string, limit int) ([]string, error) {
	var files []string
	err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		// пропускаем сокеты/устройства
		if !info.Mode().IsRegular() {
			return nil
		}
		files = append(files, p)
		if len(files) >= limit {
			return filepath.SkipAll
		}
		return nil
	})
	return files, err
}

func scanWorkerCount() int {
	n := runtime.NumCPU()
	if n < 2 {
		n = 2
	}
	if n > 12 {
		n = 12
	}
	return n
}

// PreferClamd — есть ли бинарники для ускоренного режима.
func (e *Engine) PreferClamd() bool {
	return e.clamdPath() != "" && e.clamdscanPath() != ""
}

// Лимиты ClamAV: по умолчанию ~100M — большие установщики «тихо» пропускались.
// AlertExceedsMax: если всё же упёрлись — FOUND Heuristics.Limits.Exceeded.* (не «чисто»).
func (e *Engine) desiredClamdConf() string {
	db := e.DatabaseDir
	certs := e.CertsDir
	pid := e.pidPath
	if runtime.GOOS == "windows" {
		db = filepath.ToSlash(db)
		certs = filepath.ToSlash(certs)
		pid = filepath.ToSlash(pid)
	}
	var b strings.Builder
	threads := scanWorkerCount()
	b.WriteString("# generated by YagaAegis limits-v3 — do not edit\n")
	b.WriteString("Foreground false\n")
	if runtime.GOOS == "windows" {
		b.WriteString("TCPSocket 3318\nTCPAddr 127.0.0.1\n")
	} else {
		fmt.Fprintf(&b, "LocalSocket %s\nFixStaleSocket true\n", e.socketPath)
	}
	fmt.Fprintf(&b, "PidFile %s\n", pid)
	fmt.Fprintf(&b, "DatabaseDirectory %s\n", db)
	if certs != "" {
		fmt.Fprintf(&b, "CVDCertsDirectory %s\n", certs)
	}
	fmt.Fprintf(&b, "MaxThreads %d\n", threads)
	fmt.Fprintf(&b, "MaxQueue %d\n", threads*4)
	b.WriteString(`MaxDirectoryRecursion 25
ExitOnOOM true
AlertExceedsMax true
MaxFileSize 4096M
MaxScanSize 8192M
StreamMaxLength 4096M
MaxFiles 200000
MaxEmbeddedPE 2048M
MaxHTMLNormalize 512M
MaxHTMLNoTags 64M
MaxScriptNormalize 256M
MaxZipTypeRcg 128M
PCREMaxFileSize 4096M
`)
	return b.String()
}

// Warmup поднимает clamd заранее (старт приложения).
func (e *Engine) Warmup(ctx context.Context) {
	if !e.PreferClamd() {
		return
	}
	e.warmOnce.Do(func() {
		_ = e.ensureClamd(ctx)
	})
}
