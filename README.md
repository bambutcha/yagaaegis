# Stillroom

Локальный сканер файлов и документов на вредоносное ПО.  
**Содержимое файлов никуда не уходит** — проверка только на твоём компьютере.

Имя: *still room* — «тихая комната», где файл остаётся с тобой.

Стек: **Go 1.27** + **Wails v2** + **React/TypeScript** + локальный **ClamAV**.

## Приватность

| Действие | Сеть |
|----------|------|
| Сканирование файла | **Нет** — читаем только с диска |
| Обновление баз (`freshclam`) | Только скачивание сигнатур. Твои файлы **не** загружаются |

## Что нужно

- Go **1.27.2+** (`go version`)
- Node.js 20+
- Wails CLI v2: `go install github.com/wailsapp/wails/v2/cmd/wails@latest`
- ClamAV в `PATH` (`clamscan`)
- Для GUI на Linux: WebKitGTK

## Запуск и сборка

```bash
export PATH="$HOME/.local/go/bin:$PATH"
export GOROOT="$HOME/.local/go"

cd frontend && npm ci && npm run build && cd ..
go test ./...

wails dev
wails build                        # Linux
wails build -platform windows/amd64  # Windows
```

> `go test` / `go build` требуют `frontend/dist` (собирается командой выше или в CI).

## Лицензия

MIT
