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
                <h1>LocalGuard</h1>
                <p className="tagline">{info}</p>
                <p className="privacy">
                    Privacy: file bytes never leave this computer. Optional network use is only for
                    signature updates you trigger yourself.
                </p>
            </header>

            <section className="card">
                <h2>Scanner engine</h2>
                {engine?.available ? (
                    <p className="ok-text">Ready: {engine.version || engine.binary}</p>
                ) : (
                    <p className="warn-text">
                        {engine?.message || 'ClamAV not detected. Install clamav and restart.'}
                    </p>
                )}
                <button className="btn secondary" onClick={refreshEngine} disabled={busy}>
                    Refresh engine status
                </button>
            </section>

            <section className="card">
                <h2>Scan a file</h2>
                <button className="btn" onClick={pickAndScan} disabled={busy}>
                    {busy ? 'Scanning…' : 'Choose file & scan'}
                </button>
                {error && <p className="bad-text">{error}</p>}
            </section>

            {result && (
                <section className={`card result ${statusClass}`}>
                    <h2>Result: {result.status}</h2>
                    <ul className="meta">
                        <li>
                            <strong>Path:</strong> {result.path}
                        </li>
                        {result.sha256 && (
                            <li>
                                <strong>SHA-256:</strong> {result.sha256}
                            </li>
                        )}
                        {result.threat && (
                            <li>
                                <strong>Threat:</strong> {result.threat}
                            </li>
                        )}
                        {result.engine && (
                            <li>
                                <strong>Engine:</strong> {result.engine}
                            </li>
                        )}
                        {typeof result.durationMs === 'number' && (
                            <li>
                                <strong>Duration:</strong> {result.durationMs} ms
                            </li>
                        )}
                        {result.message && (
                            <li>
                                <strong>Details:</strong> {result.message}
                            </li>
                        )}
                        <li>
                            <strong>Offline scan:</strong> {result.offline ? 'yes' : 'no'}
                        </li>
                    </ul>
                </section>
            )}
        </div>
    );
}

export default App;
