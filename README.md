# LocalGuard `v0.2.1`

Локальный сканер файлов для **Windows и Linux**.  
Скачал пакет → распаковал/установил → запустил. **Содержимое файлов в сеть не уходит.**

Стек: Go 1.27, Wails v2, React, TypeScript, Tailwind, вшитый ClamAV runtime.

## Что получает пользователь

| Платформа | Артефакт |
|-----------|----------|
| Linux | `LocalGuard-linux-amd64.tar.gz` — бинарник + `runtime/` + `seed/` |
| Windows | `LocalGuard-windows-amd64.zip` (portable) и/или `*-installer.exe` (NSIS) |

Внутри:

```
LocalGuard[.exe]
Uninstall[.exe]       ← удаление приложения + данных (linux/windows)
runtime/<os>-amd64/   ← clamscan + libs + certs
seed/                 ← CVD на первый офлайн-запуск
```

### Удаление

```bash
# Linux (из папки дистрибутива)
./Uninstall          # спросит подтверждение
./Uninstall -yes     # сразу
./Uninstall -yes -keep-data   # оставить ~/.config/localguard

# Windows portable — Uninstall.exe рядом с LocalGuard.exe
# Windows NSIS — «Удаление» в меню Пуск или Uninstall.exe / штатный uninstaller
```

Обновление сигнатур — кнопка **«Базы»** (зеркало Microsoft, обычно доступно из РФ).  
Обновление приложения — **«Обновления»** → проверка GitHub Release → **«установить»** (portable zip/tar.gz поверх текущей папки, затем перезапуск).

| Действие | Сеть |
|----------|------|
| Сканирование | Нет |
| «Базы» | Только CVD |
| «Обновления» / установка | Метаданные + скачивание релиза с GitHub |

## Версии

Источник правды: `internal/version/version.go` (+ `wails.json` `productVersion`).  
Правило для агента: `.cursor/rules/localguard-versioning.mdc` (как в Orvum).

## Сборка

```bash
chmod +x scripts/*.sh
./scripts/package.sh linux amd64
# → dist/LocalGuard-linux-amd64.tar.gz

# на Windows (или CI):
./scripts/package.sh windows amd64 --nsis
```

Разработка:

```bash
cd frontend && npm ci && npm run build && cd ..
go test ./...
wails dev
```

## Лицензия

MIT (код LocalGuard). ClamAV в runtime — GPL-2.0; соблюдайте условия GPL при распространении бинарников с движком.
