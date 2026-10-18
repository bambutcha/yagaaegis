# YagaAegis `v1.0.1`

Автор: [bambutcha](https://github.com/bambutcha)

Локальный сканер файлов для **Windows и Linux**.  
Скачал пакет → распаковал/установил → запустил. **Содержимое файлов в сеть не уходит.**

Стек: Go 1.27, Wails v2, React, TypeScript, Tailwind, вшитый ClamAV runtime.

## Что получает пользователь

| Платформа | Артефакт |
|-----------|----------|
| Linux | `YagaAegis-linux-amd64.tar.gz` — бинарник + `runtime/` + `seed/` (~130 MB сжатый) |
| Windows | `YagaAegis-windows-amd64.zip` / `*-installer.exe` (~250 MB: seed CVD ≈108 MB + ClamAV runtime) |

Внутри:

```
YagaAegis[.exe]
Uninstall[.exe]       ← удаление приложения + данных (linux/windows)
runtime/<os>-amd64/   ← clamscan + libs + certs
seed/                 ← CVD на первый офлайн-запуск
```

### Удаление

```bash
# Linux (из папки дистрибутива)
./Uninstall          # спросит подтверждение
./Uninstall -yes     # сразу
./Uninstall -yes -keep-data   # оставить ~/.config/yagaaegis

# Windows portable — Uninstall.exe рядом с YagaAegis.exe
# Windows NSIS — «Удаление» в меню Пуск или Uninstall.exe / штатный uninstaller
```

Если ClamAV не нашёл вирус, но в PDF/Office/HTML есть скрипты или макросы — статус **«Внимание»** (не то же самое, что угроза). В папке с вирусами такие файлы тоже показываются отдельно.

Скорость: при наличии `clamd`/`clamdscan` в `runtime/` базы грузятся в демон (~секунды один раз). Демон остаётся после закрытия окна — следующий запуск уже тёплый. Содержимое проверенных файлов в RAM не копится (только сигнатуры CVD, фиксированный объём). Uninstall останавливает clamd.

Обновление сигнатур — кнопка **«Базы»** (зеркало Microsoft, обычно доступно из РФ).  
Обновление приложения — **«Обновления»** → проверка GitHub Release → **«установить»** (portable zip/tar.gz поверх текущей папки, затем перезапуск).

| Действие | Сеть |
|----------|------|
| Сканирование | Нет |
| «Базы» | Только CVD |
| «Обновления» / установка | Метаданные + скачивание релиза с GitHub |

## Версии

Источник правды: `internal/version/version.go` (+ `wails.json` `productVersion`).  
Правило для агента: `.cursor/rules/yagaaegis-versioning.mdc` (как в Orvum).

## Сборка

```bash
chmod +x scripts/*.sh
./scripts/package.sh linux amd64
# → dist/YagaAegis-linux-amd64.tar.gz

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

MIT (код YagaAegis). ClamAV в runtime — GPL-2.0; соблюдайте условия GPL при распространении бинарников с движком.
