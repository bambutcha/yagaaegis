# Changelog

Формат: [Keep a Changelog](https://keepachangelog.com/ru/1.1.0/) + SemVer.  
Каждый GitHub Release обязан дублировать блок: **о продукте (1–2 предложения)** + **изменения относительно предыдущего тега**.

## [1.0.3] — 2026-10-18

**YagaAegis** — локальный сканер файлов/папок на Windows и Linux на базе ClamAV; файлы не уходят в сеть.

### Изменения относительно 1.0.2

- `MaxThreads` / `MaxQueue` в clamd по числу CPU (до 12)
- Скан папки: горутины шлют файлы в clamd параллельно (сам разбор файла всё ещё внутри ClamAV)

## [1.0.2] — 2026-10-18

**YagaAegis** — локальный сканер файлов/папок на Windows и Linux на базе ClamAV; файлы не уходят в сеть.

### Изменения относительно 1.0.1

- Лимиты ClamAV подняты (~4 GiB файл / ~8 GiB скан) — большие установщики больше не «тихо» пропускаются
- Если лимит всё же сработал — статус **«Внимание»** и toast, а не ложное «Чисто»
- Перезапуск clamd при смене конфига лимитов

## [1.0.1] — 2026-10-18

**YagaAegis** — локальный сканер на ClamAV (UI + вшитый runtime).

### Изменения относительно 1.0.0

- clamd не убивается при закрытии окна — следующий запуск уже с тёплыми базами
- Uninstall останавливает фоновый clamd

## [1.0.0] — 2026-10-18

**YagaAegis** — первый major: самодостаточный офлайн-сканер для Linux/Windows.

### Изменения относительно 0.4.1

- Ренейм LocalGuard → YagaAegis, репозиторий `bambutcha/yagaaegis`
- Предупреждения о скриптах/макросах в документах (в т.ч. внутри заражённых папок)
- Счётчики: проверено / угроз / без вируса
- Ускорение через clamd + clamdscan
- История-оверлей, тосты, папки, обновления баз и приложения

## [0.4.1] — 2026-10-18

### Изменения относительно 0.4.0

- История через portal (не сдвигает вёрстку)
- Модуль/API обновлений на `bambutcha/yagaaegis`

## [0.4.0] — 2026-10-18

### Изменения относительно 0.3.x

- Бренд и артефакты YagaAegis
- Миграция данных с `~/.config/localguard`

---

[1.0.3]: https://github.com/bambutcha/yagaaegis/releases/tag/v1.0.3
[1.0.2]: https://github.com/bambutcha/yagaaegis/releases/tag/v1.0.2
[1.0.1]: https://github.com/bambutcha/yagaaegis/releases/tag/v1.0.1
[1.0.0]: https://github.com/bambutcha/yagaaegis/releases/tag/v1.0.0
[0.4.1]: https://github.com/bambutcha/yagaaegis/releases/tag/v0.4.1
[0.4.0]: https://github.com/bambutcha/yagaaegis/releases/tag/v0.4.0
