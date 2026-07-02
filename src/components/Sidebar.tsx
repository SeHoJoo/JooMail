import type { Account } from "../types";
import { AccountSwitcher } from "./AccountSwitcher";
import { Icon } from "./Icon";
import { MailboxItem } from "./MailboxItem";

type SidebarProps = {
  accounts: Account[];
  selectedAccount: Account;
  selectedMailboxId: string;
  onSelectAccount: (id: string) => void;
  onSelectMailbox: (id: string) => void;
  onCompose: () => void;
};

export function Sidebar({ accounts, selectedAccount, selectedMailboxId, onSelectAccount, onSelectMailbox, onCompose }: SidebarProps) {
  const system = selectedAccount.mailboxes.filter((mailbox) => mailbox.kind !== "folder");
  const folders = selectedAccount.mailboxes.filter((mailbox) => mailbox.kind === "folder");

  return (
    <aside className="hidden w-[248px] shrink-0 flex-col border-r border-line bg-panel md:flex">
      <AccountSwitcher accounts={accounts} selectedAccount={selectedAccount} onSelectAccount={onSelectAccount} />
      <div className="px-[11px] pb-2 pt-[13px]">
        <button className="flex h-[38px] w-full items-center justify-center gap-2 rounded-lg bg-accent px-3 text-[13.5px] font-medium text-white" onClick={onCompose}>
          <Icon name="compose" className="h-4 w-4" />
          새 메일 쓰기
        </button>
      </div>
      <nav className="space-y-0">
        {system.map((mailbox) => (
          <MailboxItem key={mailbox.id} mailbox={mailbox} selectedId={selectedMailboxId} onSelect={onSelectMailbox} />
        ))}
      </nav>
      <div className="mt-4 px-4 text-[11px] font-bold text-[#9aa0a8]">폴더</div>
      <nav className="mt-2 space-y-0 bg-white py-1">
        {folders.map((mailbox) => (
          <MailboxItem key={mailbox.id} mailbox={mailbox} selectedId={selectedMailboxId} onSelect={onSelectMailbox} />
        ))}
      </nav>
      <div className="mt-auto px-[11px] pb-5 text-[11px] text-muted">
        <div className="flex justify-between">
          <span>저장용량</span>
          <span>{selectedAccount.storage}</span>
        </div>
        <div className="mt-2 h-1 rounded-full bg-[#e4e7ea]">
          <div className="h-1 w-[41%] rounded-full bg-[#8b93a0]" />
        </div>
      </div>
    </aside>
  );
}
