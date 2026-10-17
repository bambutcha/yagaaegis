import {useCallback, useEffect, useMemo, useState} from 'react';
import {
    AppInfo,
    AppVersion,
    ApplyAppUpdate,
    CheckAppUpdate,
    ClearScanHistory,
    EngineStatus,
    OpenURL,
    PickFile,
    PickFolder,
    ScanHistory,
    ScanPath,
    UpdateDatabases,
} from '../wailsjs/go/main/App';
import {pushToast, ToastHost} from './toast';

type Hit = {path: string; threat: string};

type ScanResult = {
    path: string;
    sha256?: string;
    status: string;
    threat?: string;
    hits?: Hit[];
    isDir?: boolean;
    engine?: string;
    scannedAt?: string;
    durationMs?: number;
    message?: string;
    offline?: boolean;
};

type HistoryEntry = {
    path: string;
    status: string;
    threat?: string;
    message?: string;
    isDir?: boolean;
    hitCount?: number;
    durationMs?: number;
    scannedAt?: string;
};

type EngineInfo = {
    available: boolean;
    message?: string;
    version?: string;
    dbReady?: boolean;
    dbDir?: string;
};

type UpdateInfo = {
    ok?: boolean;
    hasUpdate?: boolean;
    latest?: string;
    url?: string;
    assetUrl?: string;
    message?: string;
    needRestart?: boolean;
};

const statusRu: Record<string, string> = {
    clean: 'Чисто',
    infected: 'Угроза',
    error: 'Ошибка',
    unavailable: 'Нет движка',
};

function shortVersion(v?: string) {
    if (!v) return '';
    const m = v.match(/ClamAV\s+[\d.]+/i);
    return m ? m[0] : v.split('\n')[0];
}

function formatSecs(ms?: number) {
    if (typeof ms !== 'number') return '';
    return (
        (ms / 1000).toLocaleString('ru-RU', {
            minimumFractionDigits: 1,
            maximumFractionDigits: 2,
        }) + ' с'
    );
}

function App() {
    const [info, setInfo] = useState('');
    const [appCredit, setAppCredit] = useState('v…');
    const [authorURL, setAuthorURL] = useState('https://github.com/bambutcha');
    const [engine, setEngine] = useState<EngineInfo | null>(null);
    const [busy, setBusy] = useState(false);
    const [updating, setUpdating] = useState(false);
    const [checkingUpd, setCheckingUpd] = useState(false);
    const [applyingUpd, setApplyingUpd] = useState(false);
    const [result, setResult] = useState<ScanResult | null>(null);
    const [history, setHistory] = useState<HistoryEntry[]>([]);
    const [error, setError] = useState('');
    const [appUpdate, setAppUpdate] = useState<UpdateInfo | null>(null);

    const refreshHistory = useCallback(() => {
        ScanHistory()
            .then((v) => setHistory((v as HistoryEntry[]) || []))
            .catch(() => undefined);
    }, []);

    useEffect(() => {
        AppInfo().then(setInfo).catch(() => undefined);
        AppVersion()
            .then((v) => {
                const o = v as {credit?: string; label?: string; authorURL?: string};
                setAppCredit(String(o.credit || o.label || 'v?'));
                if (o.authorURL) setAuthorURL(o.authorURL);
            })
            .catch(() => undefined);
        EngineStatus()
            .then((v) => setEngine(v as EngineInfo))
            .catch((e) => setError(String(e)));
        refreshHistory();
    }, [refreshHistory]);

    const engineOk = Boolean(engine?.available);
    const dbOk = Boolean(engine?.dbReady);
    const ready = engineOk && dbOk;

    const resultTone = useMemo(() => {
        switch (result?.status) {
            case 'clean':
                return 'border-signal text-signal';
            case 'infected':
                return 'border-signal-danger text-signal-danger';
            default:
                return 'border-signal-warn text-signal-warn';
        }
    }, [result?.status]);

    async function runScan(path: string) {
        if (!path) return;
        setBusy(true);
        setResult(null);
        setError('');
        try {
            const res = (await ScanPath(path)) as ScanResult;
            setResult(res);
            refreshHistory();
            if (res.status === 'infected') {
                const n = res.hits?.length || 1;
                pushToast(n > 1 ? `Обнаружено угроз: ${n}` : `Угроза: ${res.threat || 'найдена'}`, 'err', 5000);
            } else if (res.status === 'clean') {
                pushToast(res.isDir ? 'Папка чистая' : 'Файл чистый', 'ok');
            } else if (res.message) {
                pushToast(res.message, 'warn');
            }
        } catch (e) {
            setError(String(e));
            pushToast(String(e), 'err');
        } finally {
            setBusy(false);
        }
    }

    async function pickFileAndScan() {
        setError('');
        try {
            const path = await PickFile();
            await runScan(path);
        } catch (e) {
            setError(String(e));
        }
    }

    async function pickFolderAndScan() {
        setError('');
        try {
            const path = await PickFolder();
            await runScan(path);
        } catch (e) {
            setError(String(e));
        }
    }

    async function refreshEngine() {
        setError('');
        try {
            const st = (await EngineStatus()) as EngineInfo;
            setEngine(st);
            if (st.available && st.dbReady) {
                pushToast(st.message || 'Движок и базы готовы', 'ok');
            } else if (st.available) {
                pushToast(st.message || 'Движок есть, баз нет', 'warn');
            } else {
                pushToast(st.message || 'Движок недоступен', 'err');
            }
        } catch (e) {
            pushToast(String(e), 'err');
        }
    }

    async function updateDatabases() {
        setError('');
        setUpdating(true);
        pushToast('Проверяю базы…', 'info', 2000);
        try {
            const res = (await UpdateDatabases()) as {
                ok?: boolean;
                message?: string;
                upToDate?: boolean;
                changed?: number;
            };
            if (!res.ok) {
                pushToast(res.message || 'Ошибка обновления баз', 'err');
            } else if (res.upToDate) {
                pushToast(res.message || 'Базы уже актуальны', 'ok');
            } else {
                pushToast(res.message || 'Базы обновлены', 'ok');
            }
            setEngine((await EngineStatus()) as EngineInfo);
        } catch (e) {
            pushToast(String(e), 'err');
        } finally {
            setUpdating(false);
        }
    }

    async function checkAppUpdate() {
        setError('');
        setCheckingUpd(true);
        pushToast('Проверяю GitHub Releases…', 'info', 2000);
        try {
            const res = (await CheckAppUpdate()) as UpdateInfo;
            setAppUpdate(res);
            if (!res.ok) {
                pushToast(res.message || 'Не удалось проверить обновления', 'err');
            } else if (res.hasUpdate) {
                pushToast(res.message || 'Есть новая версия', 'warn', 6000);
            } else {
                pushToast(res.message || 'У вас актуальная версия', 'ok');
            }
        } catch (e) {
            pushToast(String(e), 'err');
        } finally {
            setCheckingUpd(false);
        }
    }

    async function applyAppUpdate() {
        setError('');
        setApplyingUpd(true);
        pushToast('Скачиваю обновление…', 'info', 5000);
        try {
            const res = (await ApplyAppUpdate()) as UpdateInfo;
            setAppUpdate(res);
            pushToast(res.message || (res.ok ? 'Готово' : 'Ошибка'), res.ok ? 'ok' : 'err', 7000);
        } catch (e) {
            pushToast(String(e), 'err');
        } finally {
            setApplyingUpd(false);
        }
    }

    async function clearHistory() {
        await ClearScanHistory();
        refreshHistory();
    }

    return (
        <div className="app-shell px-[clamp(0.85rem,3vw,2rem)] py-[clamp(0.75rem,2.5vh,1.5rem)]">
            <ToastHost />
            <header className="shrink-0 animate-rise">
                <div className="flex flex-wrap items-end justify-between gap-3 stack-xs">
                    <div className="min-w-0">
                        <p className="mb-1 font-mono text-[0.65rem] uppercase tracking-[0.22em] text-ink-mute">
                            offline · файлы не уходят в сеть
                        </p>
                        <h1 className="brand-mark font-display font-extrabold text-ink">LocalGuard</h1>
                    </div>
                    <div className="text-right">
                        <button
                            type="button"
                            className="font-mono text-[0.8rem] font-medium text-ink-soft underline-offset-2 hover:underline"
                            title="Автор на GitHub"
                            onClick={() => OpenURL(authorURL)}
                        >
                            {appCredit}
                        </button>
                        {info && (
                            <p className="mt-1 max-w-[16rem] font-mono text-[0.7rem] leading-snug text-ink-mute hide-short sm:max-w-xs">
                                {info}
                            </p>
                        )}
                    </div>
                </div>
            </header>

            <div className="panel-rule mt-[clamp(0.75rem,2vh,1.25rem)] shrink-0" />

            <section className="mt-[clamp(0.65rem,1.8vh,1rem)] flex shrink-0 flex-wrap items-center gap-x-5 gap-y-2 font-mono text-[0.72rem] text-ink-soft">
                <div className="flex items-center gap-2">
                    <span className={`status-dot ${engineOk ? 'ok' : 'warn'}`} />
                    <span>{engineOk ? shortVersion(engine?.version) || 'движок' : 'движок недоступен'}</span>
                </div>
                <div className="flex items-center gap-2">
                    <span className={`status-dot ${dbOk ? 'ok' : 'warn'}`} />
                    <span>{dbOk ? 'базы готовы' : 'базы не загружены'}</span>
                </div>
                {!ready && engine?.message && (
                    <span className="w-full truncate text-signal-warn sm:w-auto">{engine.message}</span>
                )}
            </section>

            <main className="mt-[clamp(0.65rem,2vh,1.5rem)] flex min-h-0 flex-1 flex-col gap-[clamp(0.55rem,1.5vh,1rem)]">
                <div
                    className={`actions-bar relative flex shrink-0 flex-col items-stretch gap-2 sm:flex-row sm:items-center ${
                        busy ? 'scanning-veil' : ''
                    }`}
                >
                    <div className="flex min-w-0 flex-1 gap-2">
                        <button
                            type="button"
                            className="btn-primary min-h-[2.75rem] flex-1 px-4 py-2 text-[clamp(0.85rem,2vw,1.05rem)]"
                            onClick={pickFileAndScan}
                            disabled={busy || updating}
                        >
                            {busy ? '…' : 'Файл'}
                        </button>
                        <button
                            type="button"
                            className="btn-primary min-h-[2.75rem] flex-1 px-4 py-2 text-[clamp(0.85rem,2vw,1.05rem)]"
                            onClick={pickFolderAndScan}
                            disabled={busy || updating}
                        >
                            {busy ? '…' : 'Папка'}
                        </button>
                    </div>
                    <div className="actions-secondary flex shrink-0 gap-2">
                        <button
                            type="button"
                            className="btn-ghost min-h-[2.5rem] flex-1 px-3 py-2 text-sm sm:flex-none"
                            onClick={updateDatabases}
                            disabled={busy || updating}
                        >
                            {updating ? '…' : 'Базы'}
                        </button>
                        <button
                            type="button"
                            className="btn-ghost min-h-[2.5rem] flex-1 px-3 py-2 text-sm sm:flex-none"
                            onClick={refreshEngine}
                            disabled={busy || updating}
                        >
                            Статус
                        </button>
                        <button
                            type="button"
                            className="btn-ghost min-h-[2.5rem] flex-1 px-3 py-2 text-sm sm:flex-none"
                            onClick={checkAppUpdate}
                            disabled={checkingUpd}
                        >
                            {checkingUpd ? '…' : 'Обновл.'}
                        </button>
                    </div>
                </div>

                {(error || (appUpdate?.hasUpdate && appUpdate.assetUrl)) && (
                    <div className="shrink-0 space-y-1 font-mono text-sm">
                        {error && <p className="text-signal-danger">{error}</p>}
                        {appUpdate?.hasUpdate && appUpdate.assetUrl && (
                            <p className="text-signal">
                                Новая версия доступна
                                {' · '}
                                <button
                                    type="button"
                                    className="underline decoration-signal underline-offset-2 disabled:opacity-50"
                                    onClick={applyAppUpdate}
                                    disabled={applyingUpd}
                                >
                                    {applyingUpd ? 'устанавливаю…' : 'установить'}
                                </button>
                                {appUpdate.url && (
                                    <>
                                        {' · '}
                                        <button
                                            type="button"
                                            className="underline decoration-signal underline-offset-2"
                                            onClick={() => OpenURL(appUpdate.url!)}
                                        >
                                            страница
                                        </button>
                                    </>
                                )}
                            </p>
                        )}
                    </div>
                )}

                <div className="scroll-pane min-h-[7rem] flex-1 space-y-5">
                    {result ? (
                        <article
                            className={`result-block border-l-[3px] pl-[clamp(0.75rem,2vw,1.1rem)] ${resultTone}`}
                        >
                            <p className="font-display text-[clamp(1.4rem,3.5vw,2rem)] font-bold leading-none">
                                {statusRu[result.status] || result.status}
                                {result.isDir ? ' · папка' : ''}
                            </p>
                            {result.message && (
                                <p className="mt-2 max-w-prose text-sm text-ink-soft">{result.message}</p>
                            )}
                            <dl className="mt-4 grid gap-2 font-mono text-[0.72rem] text-ink-mute sm:grid-cols-2">
                                <div className="min-w-0 sm:col-span-2">
                                    <dt className="uppercase tracking-wider">Путь</dt>
                                    <dd className="mt-0.5 break-all text-ink-soft">{result.path}</dd>
                                </div>
                                {result.threat && (
                                    <div>
                                        <dt className="uppercase tracking-wider">Угроза</dt>
                                        <dd className="mt-0.5 text-signal-danger">{result.threat}</dd>
                                    </div>
                                )}
                                {result.sha256 && (
                                    <div className="min-w-0 sm:col-span-2">
                                        <dt className="uppercase tracking-wider">SHA-256</dt>
                                        <dd className="mt-0.5 break-all">{result.sha256}</dd>
                                    </div>
                                )}
                                {typeof result.durationMs === 'number' && (
                                    <div>
                                        <dt className="uppercase tracking-wider">Время</dt>
                                        <dd className="mt-0.5">{formatSecs(result.durationMs)}</dd>
                                    </div>
                                )}
                                {result.engine && (
                                    <div>
                                        <dt className="uppercase tracking-wider">Движок</dt>
                                        <dd className="mt-0.5">{result.engine}</dd>
                                    </div>
                                )}
                            </dl>
                            {result.hits && result.hits.length > 0 && (
                                <ul className="mt-4 max-h-40 space-y-2 overflow-auto font-mono text-[0.7rem] text-signal-danger">
                                    {result.hits.map((h) => (
                                        <li key={h.path + h.threat} className="break-all">
                                            <span className="text-ink-soft">{h.threat}</span>
                                            <br />
                                            {h.path}
                                        </li>
                                    ))}
                                </ul>
                            )}
                        </article>
                    ) : (
                        <p className="max-w-md font-mono text-[0.75rem] leading-relaxed text-ink-mute hide-short">
                            Выберите файл или папку. Проверка только на этом компьютере. Папка сканируется
                            рекурсивно.
                        </p>
                    )}

                    {history.length > 0 && (
                        <section>
                            <div className="mb-2 flex items-baseline justify-between gap-2">
                                <h2 className="font-display text-sm font-bold tracking-wide text-ink">История</h2>
                                <button
                                    type="button"
                                    className="font-mono text-[0.65rem] text-ink-mute underline-offset-2 hover:underline"
                                    onClick={clearHistory}
                                >
                                    очистить
                                </button>
                            </div>
                            <ul className="space-y-2 border-t border-ink/15 pt-2 font-mono text-[0.68rem] text-ink-mute">
                                {history.slice(0, 12).map((h, i) => (
                                    <li key={(h.scannedAt || '') + h.path + i} className="flex flex-wrap gap-x-3 gap-y-0.5">
                                        <span
                                            className={
                                                h.status === 'infected'
                                                    ? 'text-signal-danger'
                                                    : h.status === 'clean'
                                                      ? 'text-signal'
                                                      : ''
                                            }
                                        >
                                            {statusRu[h.status] || h.status}
                                            {h.hitCount && h.hitCount > 1 ? ` ×${h.hitCount}` : ''}
                                        </span>
                                        <span className="min-w-0 flex-1 truncate text-ink-soft" title={h.path}>
                                            {h.isDir ? '[dir] ' : ''}
                                            {h.path}
                                        </span>
                                        <span className="shrink-0">{formatSecs(h.durationMs)}</span>
                                    </li>
                                ))}
                            </ul>
                        </section>
                    )}
                </div>
            </main>

            {engine?.dbDir && (
                <footer className="panel-rule mt-auto shrink-0 pt-2 hide-short">
                    <p className="truncate font-mono text-[0.65rem] text-ink-mute">базы · {engine.dbDir}</p>
                </footer>
            )}
        </div>
    );
}

export default App;
