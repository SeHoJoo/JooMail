import { useEffect, useRef, useState } from "react";
import type { Account } from "../types";
import { Icon } from "./Icon";

type AccountSwitcherProps = {
  accounts: Account[];
  selectedAccount: Account;
  onSelectAccount: (id: string) => void;
  onAddAccount?: () => void;
  onLogout?: () => void;
};

export function AccountSwitcher({ accounts, selectedAccount, onSelectAccount, onAddAccount, onLogout }: AccountSwitcherProps) {
  const [open, setOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;

    function handlePointerDown(event: PointerEvent) {
      if (!rootRef.current?.contains(event.target as Node)) {
        setOpen(false);
      }
    }

    function handleKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape") setOpen(false);
    }

    document.addEventListener("pointerdown", handlePointerDown);
    document.addEventListener("keydown", handleKeyDown);
    return () => {
      document.removeEventListener("pointerdown", handlePointerDown);
      document.removeEventListener("keydown", handleKeyDown);
    };
  }, [open]);

  return (
    <div className="relative border-b border-line" ref={rootRef}>
      <button
        className="flex min-h-[69px] w-full cursor-pointer items-center gap-2 px-3 py-2 text-left hover:bg-[#f7f8f9]"
        aria-label="계정 선택"
        aria-expanded={open}
        onClick={() => setOpen((value) => !value)}
        type="button"
      >
        <span className="flex h-[30px] w-[30px] shrink-0 items-center justify-center rounded-full bg-accent text-[12px] font-bold text-white">
          {selectedAccount.initials}
        </span>
        <span className="min-w-0 flex-1">
          <span className="block break-all text-[12.5px] font-medium leading-4 text-ink">{selectedAccount.email}</span>
          <span className="block truncate text-[11.5px] text-muted">{selectedAccount.label}</span>
        </span>
        <Icon name="chevron" className={["h-3.5 w-3.5 shrink-0 text-muted", open ? "rotate-180" : ""].join(" ")} />
      </button>

      {open ? (
        <div className="absolute left-full top-2 z-40 ml-2 w-[272px] rounded-lg border border-line bg-white py-1 text-[12.5px] text-text shadow-compose">
          {accounts.map((account) => (
            <button
              key={account.id}
              className={["flex w-full items-center gap-2 px-3 py-2 text-left hover:bg-[#f6f7f8]", account.id === selectedAccount.id ? "bg-selected text-accent" : ""].join(" ")}
              onClick={() => {
                onSelectAccount(account.id);
                setOpen(false);
              }}
              type="button"
            >
              <span className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-accent text-[10px] font-bold text-white">{account.initials}</span>
              <span className="min-w-0 flex-1">
                <span className="block break-all leading-4">{account.email}</span>
                <span className="block truncate text-[11px] text-muted">{account.label}</span>
              </span>
            </button>
          ))}
          {onAddAccount || onLogout ? (
            <>
              <div className="my-1 border-t border-line" />
              {onAddAccount ? (
                <button
                  className="w-full px-3 py-2 text-left hover:bg-[#f6f7f8]"
                  onClick={() => {
                    setOpen(false);
                    onAddAccount();
                  }}
                  type="button"
                >
                  계정 추가
                </button>
              ) : null}
              {onLogout ? (
                <button
                  className="w-full px-3 py-2 text-left text-[#b23a30] hover:bg-[#fdf1f0]"
                  onClick={() => {
                    setOpen(false);
                    onLogout();
                  }}
                  type="button"
                >
                  로그아웃
                </button>
              ) : null}
            </>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}
