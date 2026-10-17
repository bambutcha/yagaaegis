package runtimebin

import (
	"os"
	"path/filepath"
	"runtime"
)

// ClamscanPath ищет clamscan во вшитом runtime рядом с приложением, затем в PATH.
//
// Ожидаемая раскладка дистрибутива:
//
//	YagaAegis(.exe)
//	runtime/
//	  linux-amd64/clamscan   или  windows-amd64/clamscan.exe
//	  + нужные .so / .dll
//	seed/
//	  main.cvd daily.cvd bytecode.cvd   (опционально, для офлайн первого запуска)
func ClamscanPath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return "", err
	}
	root := filepath.Dir(exe)
	name := "clamscan"
	if runtime.GOOS == "windows" {
		name = "clamscan.exe"
	}
	platform := runtime.GOOS + "-" + runtime.GOARCH
	candidates := []string{
		filepath.Join(root, "runtime", platform, name),
		filepath.Join(root, "runtime", name),
		filepath.Join(root, name),
	}
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			return c, nil
		}
	}
	return "", os.ErrNotExist
}

// SeedDir — каталог CVD внутри установщика (рядом с бинарником).
func SeedDir() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	exe, _ = filepath.EvalSymlinks(exe)
	return filepath.Join(filepath.Dir(exe), "seed")
}
