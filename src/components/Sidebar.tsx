import type { Account } from "../types";
import packageJSON from "../../package.json";
import { useState } from "react";
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
  onAddAccount?: () => void;
  onLogout?: () => void;
};

export function Sidebar({ accounts, selectedAccount, selectedMailboxId, onSelectAccount, onSelectMailbox, onCompose, onAddAccount, onLogout }: SidebarProps) {
  const [collapsed, setCollapsed] = useState(false);
  const system = selectedAccount.mailboxes.filter((mailbox) => mailbox.kind !== "folder");
  const folders = selectedAccount.mailboxes.filter((mailbox) => mailbox.kind === "folder");
  const collapsedMailboxes = flattenSidebarMailboxes(selectedAccount.mailboxes).filter((mailbox) => mailbox.selectable !== false);

  if (collapsed) {
    return <SidebarRail mailboxes={collapsedMailboxes} selectedAccount={selectedAccount} selectedMailboxId={selectedMailboxId} onCompose={onCompose} onSelectMailbox={onSelectMailbox} onExpand={() => setCollapsed(false)} className="md:flex" />;
  }

  return (
    <>
      <SidebarRail mailboxes={collapsedMailboxes} selectedAccount={selectedAccount} selectedMailboxId={selectedMailboxId} onCompose={onCompose} onSelectMailbox={onSelectMailbox} className="md:flex xl:hidden" />
      <aside className="hidden w-[248px] shrink-0 flex-col border-r border-line bg-panel xl:flex">
        <AccountSwitcher accounts={accounts} selectedAccount={selectedAccount} onSelectAccount={onSelectAccount} onAddAccount={onAddAccount} onLogout={onLogout} />
        <div className="flex items-center gap-2 px-[11px] pb-2 pt-[13px]">
          <button className="flex h-[38px] w-full items-center justify-center gap-2 rounded-lg bg-accent px-3 text-[13.5px] font-medium text-white" onClick={onCompose}>
            <Icon name="compose" className="h-4 w-4" />
            새 메일 쓰기
          </button>
          <button className="flex h-[38px] w-9 shrink-0 items-center justify-center rounded-lg border border-line bg-white text-muted hover:text-text" aria-label="사이드바 접기" onClick={() => setCollapsed(true)} type="button">
            <Icon name="chevron" className="h-4 w-4 rotate-90" />
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
            <span>버전</span>
            <span>v{packageJSON.version}</span>
          </div>
        </div>
      </aside>
    </>
  );
}

function SidebarRail({ mailboxes, selectedAccount, selectedMailboxId, onCompose, onSelectMailbox, onExpand, className }: { mailboxes: Account["mailboxes"]; selectedAccount: Account; selectedMailboxId: string; onCompose: () => void; onSelectMailbox: (id: string) => void; onExpand?: () => void; className: string }) {
  return (
    <aside className={["hidden w-[64px] shrink-0 flex-col items-center border-r border-line bg-panel py-2", className].join(" ")} aria-label="접힌 사이드바">
      <button className="flex h-11 w-11 items-center justify-center rounded-lg hover:bg-white" aria-label={onExpand ? "사이드바 펼치기" : selectedAccount.email} onClick={onExpand} type="button">
        <span className="flex h-[30px] w-[30px] items-center justify-center rounded-full bg-accent text-[12px] font-bold text-white">{selectedAccount.initials}</span>
      </button>
      <button className="mt-2 flex h-10 w-10 items-center justify-center rounded-lg bg-accent text-white" aria-label="새 메일 쓰기" onClick={onCompose} type="button">
        <Icon name="compose" className="h-4 w-4" />
      </button>
      <nav className="mt-3 flex w-full flex-1 flex-col items-center gap-1 overflow-y-auto px-2" aria-label="메일함">
        {mailboxes.map((mailbox) => {
          const selected = selectedMailboxId === mailbox.id;
          return (
            <button
              key={mailbox.id}
              className={["relative flex h-9 w-10 items-center justify-center rounded-md hover:bg-white", selected ? "bg-selected text-[#1b47a0]" : "text-[#3a3f45]"].join(" ")}
              aria-label={mailbox.label}
              title={mailbox.label}
              onClick={() => onSelectMailbox(mailbox.id)}
              type="button"
            >
              <Icon name={iconByKind[mailbox.kind]} className="h-4 w-4" />
              {mailbox.unread ? <span className="absolute right-1 top-1 min-w-[14px] rounded-full bg-white px-1 text-[10px] leading-[14px] text-accent">{mailbox.unread}</span> : null}
            </button>
          );
        })}
      </nav>
      {onExpand ? (
        <button className="mb-2 mt-2 flex h-9 w-10 items-center justify-center rounded-md text-muted hover:bg-white hover:text-text" aria-label="사이드바 펼치기" onClick={onExpand} type="button">
          <Icon name="chevron" className="h-4 w-4 -rotate-90" />
        </button>
      ) : null}
    </aside>
  );
}

const iconByKind: Record<Account["mailboxes"][number]["kind"], Parameters<typeof Icon>[0]["name"]> = {
  inbox: "inbox",
  starred: "star",
  sent: "send",
  drafts: "draft",
  archive: "archive",
  spam: "spam",
  trash: "trash",
  folder: "folder",
};

function flattenSidebarMailboxes(mailboxes: Account["mailboxes"]): Account["mailboxes"] {
  return mailboxes.flatMap((mailbox) => [mailbox, ...flattenSidebarMailboxes(mailbox.children ?? [])]);
}
