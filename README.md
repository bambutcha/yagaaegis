# LocalGuard

Local-only malware / document scanner. **File bytes never leave your machine.**

Stack (Oct 2026): **Go 1.27** + **Wails v2** + **React/TypeScript** + local **ClamAV**.

## Privacy

| Action | Network |
|--------|---------|
| Scan a file | **None** |
| Update signatures (`freshclam`) | Downloads DB only — never uploads your files |

## Requirements

- **Go 1.27.2+** (`go version`)
- Node.js 20+
- Wails CLI v2: `go install github.com/wailsapp/wails/v2/cmd/wails@latest`
- ClamAV on `PATH` (`clamscan`)
- Linux GUI build: WebKitGTK (`libwebkit2gtk-4.1-dev` / distro equivalent)

## Develop / build

```bash
export PATH="$HOME/.local/go/bin:$PATH"   # if you installed Go to ~/.local/go
wails dev
wails build                                 # Linux
wails build -platform windows/amd64         # Windows
```

CI builds Linux + Windows artifacts on `main` / tags.

## License

MIT
