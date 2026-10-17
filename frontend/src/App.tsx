import {useEffect, useMemo, useState} from 'react';
import {
    AppInfo,
    AppVersion,
    CheckAppUpdate,
    EngineStatus,
    OpenURL,
    PickFile,
    ScanPath,
    UpdateDatabases,
} from '../wailsjs/go/main/App';

type ScanResult = {
    path: string;
    sha256?: string;
    status: string;
    threat?: string;
    engine?: string;
    scannedAt?: string;
    durationMs?: number;
    message?: string;
    offline?: boolean;
};

type EngineInfo = {
    available: boolean;
    offline?: boolean;
    privacy?: string;
    binary?: string;
    version?: string;
    message?: string;
    dbReady?: boolean;
    dbDir?: string;
};

type UpdateInfo = {
    ok?: boolean;
    hasUpdate?: boolean;
    latest?: string;
    url?: string;
    message?: string;
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

function App() {
    const [info, setInfo] = useState('');
    const [appLabel, setAppLabel] = useState('v…');
    const [engine, setEngine] = useState<EngineInfo | null>(null);
    const [busy, setBusy] = useState(false);
    const [updating, setUpdating] = useState(false);
    const [checkingUpd, setCheckingUpd] = useState(false);
    const [result, setResult] = useState<ScanResult | null>(null);
    const [error, setError] = useState('');
    const [updateMsg, setUpdateMsg] = useState('');
    const [appUpdate, setAppUpdate] = useState<UpdateInfo | null>(null);

    useEffect(() => {
        AppInfo().then(setInfo).catch(() => undefined);
        AppVersion()
            .then((v) => setAppLabel(String((v as {label?: string}).label || 'v?')))
            .catch(() => undefined);
        EngineStatus()
            .then((v) => setEngine(v as EngineInfo))
            .catch((e) => setError(String(e)));
    }, []);

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

    async function pickAndScan() {
        setError('');
        setResult(null);
        try {
            const path = await PickFile();
            if (!path) return;
            setBusy(true);
            setResult((await ScanPath(path)) as ScanResult);
        } catch (e) {
            setError(String(e));
        } finally {
            setBusy(false);
        }
    }

    async function refreshEngine() {
        setError('');
        try {
            setEngine((await EngineStatus()) as EngineInfo);
        } catch (e) {
            setError(String(e));
        }
    }

    async function updateDatabases() {
        setError('');
        setUpdateMsg('');
        setUpdating(true);
        try {
            const res = (await UpdateDatabases()) as {ok?: boolean; message?: string};
            setUpdateMsg(res.message || (res.ok ? 'Базы обновлены' : 'Ошибка обновления'));
            setEngine((await EngineStatus()) as EngineInfo);
        } catch (e) {
            setError(String(e));
        } finally {
            setUpdating(false);
        }
    }

    async function checkAppUpdate() {
        setError('');
        setCheckingUpd(true);
        try {
            setAppUpdate((await CheckAppUpdate()) as UpdateInfo);
        } catch (e) {
            setError(String(e));
        } finally {
            setCheckingUpd(false);
        }
    }

    return (
        <div className="app-shell px-[clamp(0.85rem,3vw,2rem)] py-[clamp(0.75rem,2.5vh,1.5rem)]">
            <header className="shrink-0 animate-rise">
                <div className="flex flex-wrap items-end justify-between gap-3 stack-xs">
                    <div className="min-w-0">
                        <p className="mb-1 font-mono text-[0.65rem] uppercase tracking-[0.22em] text-ink-mute">
                            offline · файлы не уходят в сеть
                        </p>
                        <h1 className="brand-mark font-display font-extrabold text-ink">LocalGuard</h1>
                    </div>
                    <div className="text-right">
                        <p className="font-mono text-[0.8rem] font-medium text-ink-soft">{appLabel}</p>
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

            <main className="mt-[clamp(1rem,3vh,1.75rem)] flex min-h-0 flex-1 flex-col gap-[clamp(0.75rem,2vh,1.25rem)]">
                <div
                    className={`relative flex shrink-0 flex-col items-stretch gap-3 sm:flex-row sm:items-center ${
                        busy ? 'scanning-veil' : ''
                    }`}
                >
                    <button
                        type="button"
                        className="btn-primary min-h-[3rem] flex-1 px-5 py-3 text-[clamp(0.95rem,2.2vw,1.15rem)]"
                        onClick={pickAndScan}
                        disabled={busy || updating}
                    >
                        {busy ? 'Сканирование…' : 'Выбрать файл и проверить'}
                    </button>
                    <div className="flex shrink-0 flex-wrap gap-2 stack-xs">
                        <button
                            type="button"
                            className="btn-ghost min-h-[2.75rem] px-4 py-2 text-sm"
                            onClick={updateDatabases}
                            disabled={busy || updating}
                        >
                            {updating ? 'Обновление…' : 'Базы'}
                        </button>
                        <button
                            type="button"
                            className="btn-ghost min-h-[2.75rem] px-4 py-2 text-sm"
                            onClick={refreshEngine}
                            disabled={busy || updating}
                        >
                            Статус
                        </button>
                        <button
                            type="button"
                            className="btn-ghost min-h-[2.75rem] px-4 py-2 text-sm"
                            onClick={checkAppUpdate}
                            disabled={checkingUpd}
                        >
                            {checkingUpd ? '…' : 'Обновления'}
                        </button>
                    </div>
                </div>

                {(error || updateMsg || appUpdate?.message) && (
                    <div className="shrink-0 space-y-1 font-mono text-sm">
                        {error && <p className="text-signal-danger">{error}</p>}
                        {updateMsg && <p className="text-ink-mute">{updateMsg}</p>}
                        {appUpdate?.message && (
                            <p className={appUpdate.hasUpdate ? 'text-signal' : 'text-ink-mute'}>
                                {appUpdate.message}
                                {appUpdate.hasUpdate && appUpdate.url && (
                                    <>
                                        {' · '}
                                        <button
                                            type="button"
                                            className="underline decoration-signal underline-offset-2"
                                            onClick={() => OpenURL(appUpdate.url!)}
                                        >
                                            скачать
                                        </button>
                                    </>
                                )}
                            </p>
                        )}
                    </div>
                )}

                <div className="scroll-pane flex-1">
                    {result ? (
                        <article
                            className={`result-block border-l-[3px] pl-[clamp(0.75rem,2vw,1.1rem)] ${resultTone}`}
                        >
                            <p className="font-display text-[clamp(1.4rem,3.5vw,2rem)] font-bold leading-none">
                                {statusRu[result.status] || result.status}
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
                                        <dd className="mt-0.5">
                                            {(result.durationMs / 1000).toLocaleString('ru-RU', {
                                                minimumFractionDigits: 1,
                                                maximumFractionDigits: 2,
                                            })}{' '}
                                            с
                                        </dd>
                                    </div>
                                )}
                                {result.engine && (
                                    <div>
                                        <dt className="uppercase tracking-wider">Движок</dt>
                                        <dd className="mt-0.5">{result.engine}</dd>
                                    </div>
                                )}
                            </dl>
                        </article>
                    ) : (
                        <p className="max-w-md font-mono text-[0.75rem] leading-relaxed text-ink-mute hide-short">
                            Выберите документ или архив. Проверка идёт только на этом компьютере — байты файла
                            никуда не отправляются. Сеть нужна лишь для обновления сигнатур и проверки версии
                            приложения.
                        </p>
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
