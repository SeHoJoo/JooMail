import { useState } from "react";

export type QaState =
  | "normal"
  | "loading"
  | "error"
  | "empty"
  | "empty-reading"
  | "search"
  | "search-account"
  | "search-empty"
  | "multiselect"
  | "compose"
  | "remote-images-shown"
  | "quoted-expanded"
  | "long-overflow"
  | "many-attachments"
  | "empty-custom-folder"
  | "nested-tree"
  | "mobile-reading-attachments"
  | "compose-cc-bcc";

type DevStateSwitcherProps = {
  states: QaState[];
  onApply: (state: QaState) => void;
};

const labels: Record<QaState, string> = {
  normal: "Normal",
  loading: "Loading",
  error: "Error",
  empty: "Empty",
  "empty-reading": "Empty reading",
  search: "Search",
  "search-account": "Search account",
  "search-empty": "Search empty",
  multiselect: "Multi-select",
  compose: "Compose",
  "remote-images-shown": "Remote images",
  "quoted-expanded": "Quoted expanded",
  "long-overflow": "Long overflow",
  "many-attachments": "Many attachments",
  "empty-custom-folder": "Empty folder",
  "nested-tree": "Nested tree",
  "mobile-reading-attachments": "Mobile reading",
  "compose-cc-bcc": "Compose Cc/Bcc",
};

export function DevStateSwitcher({ states, onApply }: DevStateSwitcherProps) {
  const [open, setOpen] = useState(false);

  if (!import.meta.env.DEV) return null;

  return (
    <div className="fixed bottom-4 right-4 z-50 flex flex-col items-end gap-2 text-[12px]">
      {open ? (
        <div className="w-[170px] rounded-lg border border-line bg-white p-2 shadow-compose">
          <div className="mb-1 px-2 py-1 text-[11px] font-bold uppercase tracking-[0.08em] text-muted">QA states</div>
          <div className="grid gap-1">
            {states.map((state) => (
              <button
                key={state}
                className="rounded-md px-2 py-1.5 text-left text-text hover:bg-[#f2f3f5] focus:bg-selected"
                onClick={() => {
                  onApply(state);
                  setOpen(false);
                }}
              >
                {labels[state]}
              </button>
            ))}
          </div>
        </div>
      ) : null}
      <button
        className="h-8 rounded-full border border-line bg-white px-3 text-[12px] font-bold text-accent shadow-compose"
        aria-expanded={open}
        aria-label="QA 상태 전환"
        onClick={() => setOpen((current) => !current)}
      >
        QA
      </button>
    </div>
  );
}
