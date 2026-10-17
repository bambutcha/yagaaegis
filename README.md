# LocalGuard

Локальный сканер файлов и документов.  
**Содержимое файлов не уходит в сеть** — проверка только на устройстве.

Стек: Go 1.27, Wails v2, React, TypeScript, Tailwind, ClamAV.

## Приватность

| Действие | Сеть |
|----------|------|
| Сканирование | Нет |
| `freshclam` (базы) | Только скачивание сигнатур |

## Требования

- Go 1.27.2+
- Node.js 20+
- Wails CLI v2
- ClamAV (`clamscan` в PATH)
- Linux GUI: WebKitGTK

## Сборка и тест

```bash
cd frontend && npm ci && npm run build && cd ..
go test ./...
wails doctor
wails dev
wails build
wails build -platform windows/amd64
```

## Лицензия

MIT
