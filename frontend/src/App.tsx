import {useEffect, useState} from 'react';
import './App.css';
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
    const [info, setInfo] = useState('Stillroom');
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
            const res = await ScanPath(path);
            setResult(res as ScanResult);
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

    const statusClass =
        result?.status === 'clean'
            ? 'ok'
            : result?.status === 'infected'
              ? 'bad'
              : result
                ? 'warn'
                : '';

    return (
        <div id="App">
            <header className="header">
                <h1>Stillroom</h1>
                <p className="tagline">{info}</p>
                <p className="privacy">
                    Приватность: байты файла не покидают этот компьютер. Сеть нужна только если
                    ты сам обновишь базы сигнатур.
                </p>
            </header>

            <section className="card">
                <h2>Движок сканирования</h2>
                {engine?.available ? (
                    <p className="ok-text">Готов: {engine.version || engine.binary}</p>
                ) : (
                    <p className="warn-text">
                        {engine?.message || 'ClamAV не найден. Установи clamav и перезапусти приложение.'}
                    </p>
                )}
                <button className="btn secondary" onClick={refreshEngine} disabled={busy}>
                    Обновить статус движка
                </button>
            </section>

            <section className="card">
                <h2>Проверка файла</h2>
                <button className="btn" onClick={pickAndScan} disabled={busy}>
                    {busy ? 'Сканирование…' : 'Выбрать файл и проверить'}
                </button>
                {error && <p className="bad-text">{error}</p>}
            </section>

            {result && (
                <section className={`card result ${statusClass}`}>
                    <h2>Результат: {statusLabel[result.status] || result.status}</h2>
                    <ul className="meta">
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
