import type { Account } from "../types";

type AccountSwitcherProps = {
  accounts: Account[];
  selectedAccount: Account;
  onSelectAccount: (id: string) => void;
  onLogout?: () => void;
};

export function AccountSwitcher({ accounts, selectedAccount, onSelectAccount, onLogout }: AccountSwitcherProps) {
  return (
    <label className="flex h-[69px] items-center gap-3 border-b border-line px-4">
      <span className="flex h-[30px] w-[30px] shrink-0 items-center justify-center rounded-full bg-accent text-[12px] font-bold text-white">
        {selectedAccount.initials}
      </span>
      <span className="min-w-0 flex-1">
        <span className="block truncate text-[13.5px] font-medium text-ink">{selectedAccount.email}</span>
        <span className="block truncate text-[11.5px] text-muted">{selectedAccount.label}</span>
      </span>
      <select
        className="h-7 w-6 appearance-none bg-transparent text-transparent"
        aria-label="계정 선택"
        value={selectedAccount.id}
        onChange={(event) => {
          if (event.target.value === "__logout") {
            onLogout?.();
            return;
          }
          onSelectAccount(event.target.value);
        }}
      >
        {accounts.map((account) => (
          <option key={account.id} value={account.id}>
            {account.email}
          </option>
        ))}
        {onLogout ? <option value="__logout">로그아웃</option> : null}
      </select>
    </label>
  );
}
