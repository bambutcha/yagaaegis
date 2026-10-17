import {useCallback, useEffect, useState} from 'react';

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
    for (const l of listeners) l(items);
}

export function pushToast(text: string, kind: ToastKind = 'info', ms = 4200) {
    const id = nextId++;
    items = [...items, {id, kind, text}];
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
    if (list.length === 0) return null;
    return (
        <div
            className="pointer-events-none fixed inset-x-0 bottom-0 z-50 flex flex-col items-stretch gap-2 p-[clamp(0.6rem,2.5vw,1rem)] sm:items-end"
            aria-live="polite"
        >
            {list.map((t) => (
                <button
                    key={t.id}
                    type="button"
                    onClick={() => dismiss(t.id)}
                    className={`toast-enter pointer-events-auto max-w-full border px-[clamp(0.75rem,2vw,1rem)] py-[clamp(0.55rem,1.5vh,0.75rem)] text-left font-mono text-[clamp(0.68rem,1.8vw,0.78rem)] leading-snug shadow-stamp sm:max-w-[min(24rem,92vw)] ${
                        t.kind === 'ok'
                            ? 'border-signal bg-paper-mist text-signal'
                            : t.kind === 'err'
                              ? 'border-signal-danger bg-paper-mist text-signal-danger'
                              : t.kind === 'warn'
                                ? 'border-signal-warn bg-paper-mist text-signal-warn'
                                : 'border-ink/25 bg-paper-mist text-ink-soft'
                    }`}
                >
                    {t.text}
                </button>
            ))}
        </div>
    );
}
