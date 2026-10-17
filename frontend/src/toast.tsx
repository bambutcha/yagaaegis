import {useCallback, useEffect, useState} from 'react';
import {createPortal} from 'react-dom';

export type ToastKind = 'ok' | 'warn' | 'err' | 'info';

export type ToastItem = {
    id: number;
    kind: ToastKind;
    text: string;
};

let nextId = 1;
type Listener = (items: ToastItem[]) => void;
let items: ToastItem[] = [];
const listeners = new Set<Listener>();

function emit() {
    for (const l of listeners) l([...items]);
}

/** Не больше 2 toast сразу — новые вытесняют старые info. */
export function pushToast(text: string, kind: ToastKind = 'info', ms = 3800) {
    const id = nextId++;
    let next = [...items, {id, kind, text}];
    while (next.length > 2) {
        next = next.slice(1);
    }
    items = next;
    emit();
    window.setTimeout(() => {
        items = items.filter((t) => t.id !== id);
        emit();
    }, ms);
}

export function useToasts() {
    const [list, setList] = useState<ToastItem[]>(items);
    useEffect(() => {
        listeners.add(setList);
        return () => {
            listeners.delete(setList);
        };
    }, []);
    const dismiss = useCallback((id: number) => {
        items = items.filter((t) => t.id !== id);
        emit();
    }, []);
    return {list, dismiss};
}

export function ToastHost() {
    const {list, dismiss} = useToasts();
    const [mounted, setMounted] = useState(false);
    useEffect(() => setMounted(true), []);
    if (!mounted || list.length === 0) return null;

    // portal в body — fixed не привязан к .app-shell (overflow), контент не двигается
    return createPortal(
        <div
            className="pointer-events-none fixed inset-x-0 bottom-0 z-[100] flex flex-col items-stretch gap-2 p-[clamp(0.6rem,2.5vw,1rem)] sm:items-end"
            style={{position: 'fixed'}}
            aria-live="polite"
        >
            {list.map((t) => (
                <button
                    key={t.id}
                    type="button"
                    onClick={() => dismiss(t.id)}
                    className={`toast-enter pointer-events-auto w-full border bg-paper-mist/95 px-[clamp(0.75rem,2vw,1rem)] py-[clamp(0.55rem,1.5vh,0.75rem)] text-left font-mono text-[clamp(0.68rem,1.8vw,0.78rem)] leading-snug backdrop-blur-sm sm:w-auto sm:max-w-[min(22rem,92vw)] ${
                        t.kind === 'ok'
                            ? 'border-signal text-signal'
                            : t.kind === 'err'
                              ? 'border-signal-danger text-signal-danger'
                              : t.kind === 'warn'
                                ? 'border-signal-warn text-signal-warn'
                                : 'border-ink/25 text-ink-soft'
                    }`}
                >
                    {t.text}
                </button>
            ))}
        </div>,
        document.body,
    );
}
