package scanner

import (
	"archive/zip"
	"bytes"
	"compress/zlib"
	"encoding/ascii85"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Risk — эвристика «в документе есть скрипт/макрос» (не сигнатура вируса).
type Risk struct {
	Path   string // файл с риском (для папок)
	Kind   string // pdf-js | office-macro | rtf-script | html-script | script-file
	Detail string
}

const maxDocInspectBytes = 4 << 20 // 4 MiB на файл

var scriptExt = map[string]string{
	".js":  "JavaScript",
	".mjs": "JavaScript",
	".vbs": "VBScript",
	".vbe": "VBScript",
	".wsf": "Windows Script",
	".wsh": "Windows Script",
	".ps1": "PowerShell",
	".psm1": "PowerShell",
	".hta": "HTML Application",
	".jse": "JScript encoded",
}

var docExt = map[string]bool{
	".pdf": true,
	".doc": true, ".dot": true, ".docx": true, ".docm": true, ".dotm": true,
	".xls": true, ".xlt": true, ".xlsx": true, ".xlsm": true, ".xlsb": true, ".xltm": true,
	".ppt": true, ".pps": true, ".pptx": true, ".pptm": true, ".ppsm": true,
	".odt": true, ".ods": true, ".odp": true,
	".rtf": true,
	".htm": true, ".html": true, ".xhtml": true,
}

// InspectDocRisks ищет признаки скриптов/макросов в документах.
func InspectDocRisks(path string) ([]Risk, error) {
	st, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if st.IsDir() {
		return inspectDirRisks(path)
	}
	return inspectFileRisks(path)
}

func inspectDirRisks(root string) ([]Risk, error) {
	var out []Risk
	n := 0
	_ = filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		n++
		if n > 800 {
			return filepath.SkipAll
		}
		ext := strings.ToLower(filepath.Ext(p))
		if !docExt[ext] && scriptExt[ext] == "" {
			return nil
		}
		risks, err := inspectFileRisks(p)
		if err != nil || len(risks) == 0 {
			return nil
		}
		for _, r := range risks {
			out = append(out, Risk{
				Path:   p,
				Kind:   r.Kind,
				Detail: r.Detail,
			})
		}
		if len(out) >= 40 {
			return filepath.SkipAll
		}
		return nil
	})
	return out, nil
}

func inspectFileRisks(path string) ([]Risk, error) {
	ext := strings.ToLower(filepath.Ext(path))
	if label, ok := scriptExt[ext]; ok {
		return []Risk{{Path: path, Kind: "script-file", Detail: "файл-скрипт (" + label + ")"}}, nil
	}
	if !docExt[ext] {
		return nil, nil
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	head := make([]byte, maxDocInspectBytes)
	n, _ := io.ReadFull(f, head)
	if n <= 0 {
		return nil, nil
	}
	data := head[:n]

	var risks []Risk
	switch ext {
	case ".pdf":
		risks = pdfRisks(data)
	case ".rtf":
		risks = rtfRisks(data)
	case ".htm", ".html", ".xhtml":
		risks = htmlRisks(data)
	case ".docx", ".docm", ".dotm", ".xlsx", ".xlsm", ".xltm", ".pptx", ".pptm", ".ppsm",
		".odt", ".ods", ".odp":
		risks = ooxmlRisks(path, data)
	case ".doc", ".dot", ".xls", ".xlt", ".ppt", ".pps", ".xlsb":
		risks = oleRisks(data)
	default:
		return nil, nil
	}
	for i := range risks {
		risks[i].Path = path
	}
	return risks, nil
}

func pdfRisks(data []byte) []Risk {
	hay := append([]byte(nil), data...)
	hay = append(hay, pdfInflatedPayload(data)...)
	lower := bytes.ToLower(hay)
	var kinds []string
	checks := []struct {
		needle []byte
		label  string
	}{
		{[]byte("/javascript"), "JavaScript (/JavaScript)"},
		{[]byte("/js"), "JavaScript (/JS)"},
		{[]byte("/openaction"), "автодействие (/OpenAction)"},
		{[]byte("/aa"), "доп. действия (/AA)"},
		{[]byte("/richmedia"), "RichMedia"},
		{[]byte("/launch"), "запуск внешнего (/Launch)"},
		{[]byte("/embeddedfile"), "вложенный файл"},
		{[]byte("/xfa"), "XFA-формы"},
	}
	seen := map[string]bool{}
	for _, c := range checks {
		if !bytes.Contains(lower, c.needle) || seen[c.label] {
			continue
		}
		if bytes.Equal(c.needle, []byte("/aa")) && !pdfNameToken(lower, []byte("/aa")) {
			continue
		}
		if bytes.Equal(c.needle, []byte("/js")) && !pdfNameToken(lower, []byte("/js")) {
			continue
		}
		seen[c.label] = true
		kinds = append(kinds, c.label)
	}
	if len(kinds) == 0 {
		return nil
	}
	return []Risk{{Kind: "pdf-js", Detail: "PDF: " + strings.Join(kinds, ", ")}}
}

// pdfInflatedPayload — распаковка stream (Flate / ASCII85), где часто прячут /JS.
func pdfInflatedPayload(data []byte) []byte {
	var out []byte
	rest := data
	for len(out) < maxDocInspectBytes {
		i := bytes.Index(rest, []byte("stream"))
		if i < 0 {
			break
		}
		// объект перед stream — фильтры
		objStart := i - 512
		if objStart < 0 {
			objStart = 0
		}
		header := bytes.ToLower(rest[objStart:i])
		body := rest[i+6:]
		if len(body) > 0 && (body[0] == '\n' || body[0] == '\r') {
			if body[0] == '\r' && len(body) > 1 && body[1] == '\n' {
				body = body[2:]
			} else {
				body = body[1:]
			}
		}
		j := bytes.Index(body, []byte("endstream"))
		if j < 0 {
			break
		}
		payload := body[:j]
		if len(payload) > 0 && payload[len(payload)-1] == '\n' {
			payload = payload[:len(payload)-1]
		}
		if len(payload) > 0 && payload[len(payload)-1] == '\r' {
			payload = payload[:len(payload)-1]
		}
		decoded := decodePDFStream(header, payload)
		if len(decoded) > 0 {
			if len(decoded) > 256<<10 {
				decoded = decoded[:256<<10]
			}
			out = append(out, decoded...)
			out = append(out, '\n')
		}
		rest = body[j+9:]
	}
	return out
}

func decodePDFStream(headerLower, payload []byte) []byte {
	hasFlate := bytes.Contains(headerLower, []byte("/flatedecode"))
	hasA85 := bytes.Contains(headerLower, []byte("/ascii85decode"))
	data := payload
	if hasA85 {
		if d, err := decodeASCII85PDF(payload); err == nil {
			data = d
		}
	}
	if hasFlate {
		zr, err := zlib.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil
		}
		defer zr.Close()
		out, err := io.ReadAll(io.LimitReader(zr, 512<<10))
		if err != nil && len(out) == 0 {
			return nil
		}
		return out
	}
	if hasA85 {
		return data
	}
	return nil
}

func decodeASCII85PDF(payload []byte) ([]byte, error) {
	data := bytes.TrimSpace(payload)
	if bytes.HasSuffix(data, []byte("~>")) {
		data = data[:len(data)-2]
	}
	// убрать пробелы (PDF допускает)
	clean := make([]byte, 0, len(data))
	for _, c := range data {
		if c == ' ' || c == '\n' || c == '\r' || c == '\t' {
			continue
		}
		clean = append(clean, c)
	}
	dec := ascii85.NewDecoder(bytes.NewReader(clean))
	return io.ReadAll(io.LimitReader(dec, 512<<10))
}

// pdfNameToken — имя PDF вида /JS или /AA (не подстрока другого имени).
func pdfNameToken(lower, name []byte) bool {
	idx := 0
	for {
		i := bytes.Index(lower[idx:], name)
		if i < 0 {
			return false
		}
		i += idx
		end := i + len(name)
		if end >= len(lower) {
			return true
		}
		r, _ := utf8.DecodeRune(lower[end:])
		// следующий символ не буква/цифра → конец имени
		if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_') {
			return true
		}
		idx = end
	}
}

func rtfRisks(data []byte) []Risk {
	lower := bytes.ToLower(data)
	var parts []string
	if bytes.Contains(lower, []byte(`\objdata`)) || bytes.Contains(lower, []byte(`\object`)) {
		parts = append(parts, "вложенный OLE-объект")
	}
	if bytes.Contains(lower, []byte("javascript")) || bytes.Contains(lower, []byte(`\js`)) {
		parts = append(parts, "JavaScript")
	}
	if len(parts) == 0 {
		return nil
	}
	return []Risk{{Kind: "rtf-script", Detail: "RTF: " + strings.Join(parts, ", ")}}
}

func htmlRisks(data []byte) []Risk {
	lower := bytes.ToLower(data)
	var parts []string
	if bytes.Contains(lower, []byte("<script")) {
		parts = append(parts, "<script>")
	}
	if bytes.Contains(lower, []byte("javascript:")) {
		parts = append(parts, "javascript:")
	}
	if bytes.Contains(lower, []byte("vbscript:")) {
		parts = append(parts, "vbscript:")
	}
	if bytes.Contains(lower, []byte("onload=")) || bytes.Contains(lower, []byte("onerror=")) {
		parts = append(parts, "обработчики событий")
	}
	if len(parts) == 0 {
		return nil
	}
	return []Risk{{Kind: "html-script", Detail: "HTML: " + strings.Join(parts, ", ")}}
}

func ooxmlRisks(path string, head []byte) []Risk {
	// OOXML/ODF — ZIP
	if !(len(head) >= 2 && head[0] == 'P' && head[1] == 'K') {
		return oleRisks(head)
	}
	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil
	}
	defer zr.Close()

	var parts []string
	seen := map[string]bool{}
	add := func(s string) {
		if !seen[s] {
			seen[s] = true
			parts = append(parts, s)
		}
	}
	for _, f := range zr.File {
		n := strings.ToLower(f.Name)
		switch {
		case strings.Contains(n, "vbaproject.bin"), strings.Contains(n, "_vba_project"):
			add("макросы VBA")
		case strings.Contains(n, "macrosheets/"):
			add("Excel macrosheets")
		case strings.HasSuffix(n, ".bas"), strings.HasSuffix(n, ".frm"), strings.HasSuffix(n, ".cls"):
			add("модули VBA")
		case strings.Contains(n, "basic/script") || strings.Contains(n, "/scripts/"):
			add("скрипты ODF")
		}
	}
	if len(parts) == 0 {
		return nil
	}
	return []Risk{{Kind: "office-macro", Detail: "Office/ODF: " + strings.Join(parts, ", ")}}
}

func oleRisks(data []byte) []Risk {
	// Compound File Binary (OLE)
	if len(data) < 8 || data[0] != 0xD0 || data[1] != 0xCF || data[2] != 0x11 || data[3] != 0xE0 {
		return nil
	}
	upper := bytes.ToUpper(data)
	var parts []string
	if bytes.Contains(upper, []byte("_VBA_PROJECT")) || bytes.Contains(upper, []byte("VBA PROJECT")) {
		parts = append(parts, "макросы VBA")
	}
	if bytes.Contains(upper, []byte("MACROS")) && bytes.Contains(upper, []byte("VBA")) {
		if len(parts) == 0 {
			parts = append(parts, "признаки макросов")
		}
	}
	if bytes.Contains(data, []byte("Javascript")) || bytes.Contains(bytes.ToLower(data), []byte("javascript")) {
		parts = append(parts, "JavaScript")
	}
	if len(parts) == 0 {
		return nil
	}
	return []Risk{{Kind: "office-macro", Detail: "OLE-документ: " + strings.Join(parts, ", ")}}
}

func risksSummary(risks []Risk) string {
	if len(risks) == 0 {
		return ""
	}
	parts := make([]string, 0, len(risks))
	for _, r := range risks {
		parts = append(parts, r.Detail)
	}
	if len(parts) == 1 {
		return "Сигнатур вируса нет, но найдены скрипты/макросы — проверьте вручную: " + parts[0]
	}
	n := len(parts)
	if n > 3 {
		n = 3
	}
	return "Сигнатур вируса нет, но в документах есть скрипты/макросы (" +
		strconv.Itoa(len(parts)) + "): " + strings.Join(parts[:n], "; ")
}
