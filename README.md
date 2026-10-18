# YagaAegis `v1.0.3`

Автор: [bambutcha](https://github.com/bambutcha) · репозиторий: [bambutcha/yagaaegis](https://github.com/bambutcha/yagaaegis)

Локальный антивирусный сканер для **Windows и Linux**: скачал пакет → запустил → проверил файл или папку.  
**Содержимое ваших файлов в интернет не уходит** — в сеть идут только обновления баз сигнатур и (по желанию) обновления самого приложения с GitHub.

История изменений: [CHANGELOG.md](./CHANGELOG.md).

## Как устроена проверка (кратко)

```
Вы → YagaAegis (UI) → clamdscan → clamd → базы CVD
                              ↘ (если демона нет) clamscan
```

| Термин | Что это |
|--------|---------|
| **ClamAV** | Движок и экосистема (не отдельная «кнопка» в UI) |
| **Базы CVD** | Списки известных вредоносных сигнатур (`main` / `daily` / `bytecode`) |
| **clamd** | Фоновый процесс: один раз грузит базы в RAM и отвечает на запросы |
| **clamdscan** | Клиент: «проверь этот путь» у уже запущенного clamd |
| **clamscan** | Запасной режим: каждый запуск заново грузит базы (медленнее) |

Под капотом при скане:

1. Файл (или дерево папки) читается **локально**.
2. ClamAV сравнивает содержимое/структуру с **миллионами сигнатур** и эвристиками (PE, архивы, PDF и т.д.).
3. Совпадение → статус **«Угроза»** и имя сигнатуры.
4. Нет совпадения, но в документе есть скрипты/макросы → **«Внимание»** (это не вирус, а риск).
5. Сработал лимит размера/глубины → тоже **«Внимание»** («проверка неполная»), а не ложное «Чисто».

Проверенные файлы **не копятся в памяти**: после проверки буферы отпускаются. В RAM постоянно лежат в основном **базы сигнатур** (пока жив clamd).

## Возможности

- Скан файла и папки (рекурсивно)
- История сканов, обновление баз (зеркало Microsoft), обновление приложения с GitHub Releases
- Предупреждения о скриптах/макросах в PDF/Office/HTML
- Uninstall для Linux и Windows
- Тёплый clamd между запусками UI (быстрее повторные проверки)

## Дистрибутив

| Платформа | Артефакт |
|-----------|----------|
| Linux | `YagaAegis-linux-amd64.tar.gz` |
| Windows | `YagaAegis-windows-amd64.zip` / `*-installer.exe` |

```
YagaAegis[.exe]
Uninstall[.exe]
runtime/<os>-amd64/   ← clamscan, clamd, clamdscan, libs, certs
seed/                 ← CVD на первый офлайн-запуск
```

### Удаление

```bash
./Uninstall                # спросит подтверждение
./Uninstall -yes
./Uninstall -yes -keep-data   # оставить ~/.config/yagaaegis
```

Uninstall также останавливает фоновый clamd.

| Действие | Сеть |
|----------|------|
| Сканирование | Нет |
| «Базы» | Только CVD |
| «Обновления» | GitHub Releases |

## Версии и релизы

- Код версии: `internal/version/version.go` + `wails.json` → `productVersion`
- Журнал: [CHANGELOG.md](./CHANGELOG.md)
- Правило для агента: `.cursor/rules/yagaaegis-versioning.mdc`  
  При каждом git-теге / GitHub Release — **краткое описание продукта** и **что изменилось относительно предыдущего тега**.

## Сборка

```bash
chmod +x scripts/*.sh
./scripts/package.sh linux amd64
./scripts/package.sh windows amd64 --nsis   # Windows / CI
```

```bash
cd frontend && npm ci && npm run build && cd ..
go test ./...
wails dev
```

Стек: Go, Wails v2, React, TypeScript, Tailwind, вшитый runtime ClamAV.

## Лицензия

MIT (код YagaAegis). ClamAV в runtime — GPL-2.0; соблюдайте GPL при распространении сборок с движком.
