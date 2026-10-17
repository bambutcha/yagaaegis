import {useEffect, useState} from 'react';
import {AppInfo, EngineStatus, PickFile, ScanPath} from '../wailsjs/go/main/App';

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
};

const statusLabel: Record<string, string> = {
    clean: 'чисто',
    infected: 'угроза',
    error: 'ошибка',
    unavailable: 'движок недоступен',
};

function App() {
    const [info, setInfo] = useState('LocalGuard');
    const [engine, setEngine] = useState<EngineInfo | null>(null);
    const [busy, setBusy] = useState(false);
    const [result, setResult] = useState<ScanResult | null>(null);
    const [error, setError] = useState('');

    useEffect(() => {
        AppInfo().then(setInfo).catch(() => undefined);
        EngineStatus()
            .then((v) => setEngine(v as EngineInfo))
            .catch((e) => setError(String(e)));
    }, []);

    async function pickAndScan() {
        setError('');
        setResult(null);
        try {
            const path = await PickFile();
            if (!path) {
                return;
            }
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

    const resultBorder =
        result?.status === 'clean'
            ? 'border-emerald-600'
            : result?.status === 'infected'
              ? 'border-red-600'
              : 'border-amber-600';

    return (
        <div className="mx-auto max-w-2xl px-5 py-8 text-slate-100">
            <header className="mb-6">
                <h1 className="mb-2 text-3xl font-semibold tracking-tight">LocalGuard</h1>
                <p className="mb-3 text-slate-400">{info}</p>
                <p className="rounded-lg border border-slate-700 bg-slate-900/80 px-3 py-2 text-sm leading-relaxed text-sky-100">
                    Приватность: байты файла не покидают этот компьютер. Сеть нужна только если ты сам
                    обновишь базы сигнатур.
                </p>
            </header>

            <section className="mb-4 rounded-xl border border-slate-700 bg-slate-900 p-4">
                <h2 className="mb-3 text-lg font-medium">Движок сканирования</h2>
                {engine?.available ? (
                    <p className="text-emerald-400">Готов: {engine.version || engine.binary}</p>
                ) : (
                    <p className="text-amber-300">
                        {engine?.message || 'ClamAV не найден. Установи clamav и перезапусти приложение.'}
                    </p>
                )}
                <button
                    type="button"
                    className="mt-3 rounded-lg bg-slate-700 px-4 py-2 text-sm font-semibold hover:bg-slate-600 disabled:opacity-60"
                    onClick={refreshEngine}
                    disabled={busy}
                >
                    Обновить статус движка
                </button>
            </section>

            <section className="mb-4 rounded-xl border border-slate-700 bg-slate-900 p-4">
                <h2 className="mb-3 text-lg font-medium">Проверка файла</h2>
                <button
                    type="button"
                    className="rounded-lg bg-sky-600 px-4 py-2 text-sm font-semibold hover:bg-sky-500 disabled:opacity-60"
                    onClick={pickAndScan}
                    disabled={busy}
                >
                    {busy ? 'Сканирование…' : 'Выбрать файл и проверить'}
                </button>
                {error && <p className="mt-2 text-red-300">{error}</p>}
            </section>

            {result && (
                <section className={`rounded-xl border bg-slate-900 p-4 ${resultBorder}`}>
                    <h2 className="mb-3 text-lg font-medium">
                        Результат: {statusLabel[result.status] || result.status}
                    </h2>
                    <ul className="list-disc space-y-1 break-all pl-5 text-sm leading-relaxed">
                        <li>
                            <strong>Путь:</strong> {result.path}
                        </li>
                        {result.sha256 && (
                            <li>
                                <strong>SHA-256:</strong> {result.sha256}
                            </li>
                        )}
                        {result.threat && (
                            <li>
                                <strong>Угроза:</strong> {result.threat}
                            </li>
                        )}
                        {result.engine && (
                            <li>
                                <strong>Движок:</strong> {result.engine}
                            </li>
                        )}
                        {typeof result.durationMs === 'number' && (
                            <li>
                                <strong>Время:</strong> {result.durationMs} мс
                            </li>
                        )}
                        {result.message && (
                            <li>
                                <strong>Подробности:</strong> {result.message}
                            </li>
                        )}
                        <li>
                            <strong>Локальный скан:</strong> {result.offline ? 'да' : 'нет'}
                        </li>
                    </ul>
                </section>
            )}
        </div>
    );
}

export default App;
