// Generates YagaAegis-res.syso in the module root so `go build` / `wails build -nopackage`
// embed the Windows icon + version resources into YagaAegis.exe.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tc-hib/winres"
	"github.com/tc-hib/winres/version"
)

func main() {
	root, err := os.Getwd()
	if err != nil {
		fatal(err)
	}
	if len(os.Args) > 1 {
		root = os.Args[1]
	}
	windowsDir := filepath.Join(root, "build", "windows")
	iconPath := filepath.Join(windowsDir, "icon.ico")
	iconFile, err := os.Open(iconPath)
	if err != nil {
		fatal(fmt.Errorf("open icon.ico: %w", err))
	}
	defer iconFile.Close()

	ico, err := winres.LoadICO(iconFile)
	if err != nil {
		fatal(fmt.Errorf("load icon.ico: %w", err))
	}

	rs := winres.ResourceSet{}
	if err := rs.SetIcon(winres.RT_ICON, ico); err != nil {
		fatal(err)
	}

	manifestPath := filepath.Join(windowsDir, "wails.exe.manifest")
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		fatal(err)
	}
	xmlData, err := winres.AppManifestFromXML(manifestData)
	if err != nil {
		fatal(fmt.Errorf("manifest: %w", err))
	}
	rs.SetManifest(xmlData)

	infoPath := filepath.Join(windowsDir, "info.json")
	infoRaw, err := os.ReadFile(infoPath)
	if err != nil {
		fatal(err)
	}
	wjPath := filepath.Join(root, "wails.json")
	wjRaw, err := os.ReadFile(wjPath)
	if err != nil {
		fatal(err)
	}
	var wj struct {
		Name string `json:"name"`
		Info struct {
			CompanyName    string `json:"companyName"`
			ProductName    string `json:"productName"`
			ProductVersion string `json:"productVersion"`
			Copyright      string `json:"copyright"`
			Comments       string `json:"comments"`
		} `json:"info"`
	}
	if err := json.Unmarshal(wjRaw, &wj); err != nil {
		fatal(err)
	}
	repl := map[string]string{
		"{{.Info.ProductVersion}}": wj.Info.ProductVersion,
		"{{.Info.CompanyName}}":    wj.Info.CompanyName,
		"{{.Info.ProductName}}":    wj.Info.ProductName,
		"{{.Info.Copyright}}":      wj.Info.Copyright,
		"{{.Info.Comments}}":       wj.Info.Comments,
	}
	infoStr := string(infoRaw)
	for a, b := range repl {
		infoStr = strings.ReplaceAll(infoStr, a, b)
	}
	var v version.Info
	if err := v.UnmarshalJSON([]byte(infoStr)); err != nil {
		fatal(fmt.Errorf("version info: %w", err))
	}
	rs.SetVersionInfo(v)

	name := wj.Name
	if name == "" {
		name = "CampusLink"
	}
	out := filepath.Join(root, strings.ReplaceAll(name, " ", "_")+"-res.syso")
	fout, err := os.Create(out)
	if err != nil {
		fatal(err)
	}
	defer fout.Close()
	if err := rs.WriteObject(fout, winres.ArchAMD64); err != nil {
		fatal(err)
	}
	fmt.Println("wrote", out)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
