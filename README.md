# LocalGuard

Локальный сканер файлов и документов на вредоносное ПО.  
**Содержимое файлов никуда не уходит** — проверка только на твоём компьютере.

Стек: **Go 1.27** + **Wails v2** + **React + TypeScript + Tailwind** + локальный **ClamAV**.

## Почему не Next.js?

Это **десктоп** на Wails, не сайт.

| | Wails (у нас) | Next.js |
|--|---------------|---------|
| Где крутится | Окно приложения на ПК | Браузер / сервер |
| UI | React + **Vite** | React + свой сервер |
| `index.html` | Точка входа Vite (оболочка, внутри монтируется React) | Не так же устроен |

`frontend/index.html` — обычный файл Vite: пустой `<div id="root">`, дальше React. Это не «сайт на голом HTML».

## Приватность

| Действие | Сеть |
|----------|------|
| Сканирование файла | **Нет** |
| Обновление баз (`freshclam`) | Только скачивание сигнатур |

## Что нужно

- Go **1.27.2+**
- Node.js 20+
- Wails CLI v2
- ClamAV (`clamscan` в PATH)
- Linux GUI: WebKitGTK

## Как протестировать

```bash
export PATH="$HOME/.local/go/bin:$PATH"
export GOROOT="$HOME/.local/go"

# 1) Юнит-тесты Go (нужен frontend/dist — собери фронт)
cd frontend && npm ci && npm run build && cd ..
go test ./...

# 2) Движок ClamAV
clamscan --version
# Linux: sudo apt install clamav && sudo freshclam

# 3) GUI (нужен WebKitGTK на Linux)
wails doctor
wails dev
# В окне: статус движка → «Выбрать файл и проверить»

# 4) Сборка бинарников
wails build                        # Linux
wails build -platform windows/amd64
```

В CI то же: сначала `npm run build`, потом `go test` и `wails build`.

## Лицензия

MIT
