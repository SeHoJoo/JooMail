import { useEffect, useState } from "react";
import type { Mailbox } from "../types";
import { Icon } from "./Icon";

type MailboxItemProps = {
  mailbox: Mailbox;
  selectedId: string;
  level?: number;
  onSelect: (id: string) => void;
};

const iconByKind: Record<Mailbox["kind"], Parameters<typeof Icon>[0]["name"]> = {
  inbox: "inbox",
  starred: "star",
  sent: "send",
  drafts: "draft",
  archive: "archive",
  spam: "spam",
  trash: "trash",
  folder: "folder",
};

export function MailboxItem({ mailbox, selectedId, level = 0, onSelect }: MailboxItemProps) {
  const selected = selectedId === mailbox.id;
  const selectable = mailbox.selectable !== false;
  const hasChildren = Boolean(mailbox.children?.length);
  const selectedInChildren = containsSelectedMailbox(mailbox.children ?? [], selectedId);
  const [open, setOpen] = useState(true);

  useEffect(() => {
    if (selectedInChildren) setOpen(true);
  }, [selectedInChildren]);

  return (
    <div>
      <div
        className={[
          "mx-[11px] flex h-8 w-[calc(100%-22px)] items-center gap-2 rounded-md px-2 text-left text-[13px]",
          selected ? "bg-selected font-medium text-[#1b47a0]" : "text-[#3a3f45]",
          selectable || hasChildren ? "hover:bg-white" : "text-muted",
        ].join(" ")}
        style={{ paddingLeft: `${10 + level * 22}px` }}
      >
        {hasChildren ? (
          <button className="flex h-5 w-5 shrink-0 items-center justify-center rounded hover:bg-[#eef1f5]" aria-label={open ? `${mailbox.label} 접기` : `${mailbox.label} 펼치기`} onClick={() => setOpen((value) => !value)} type="button">
            <Icon name="chevron" className={["h-3.5 w-3.5 text-muted", open ? "" : "-rotate-90"].join(" ")} />
          </button>
        ) : (
          <span className="h-5 w-5 shrink-0" />
        )}
        <button
          className={["flex min-w-0 flex-1 items-center gap-2 text-left", selectable ? "" : hasChildren ? "text-muted" : "cursor-default text-muted"].join(" ")}
          onClick={() => {
            if (selectable) {
              onSelect(mailbox.id);
              return;
            }
            if (hasChildren) setOpen((value) => !value);
          }}
          disabled={!selectable && !hasChildren}
          type="button"
        >
          <Icon name={iconByKind[mailbox.kind]} className="h-4 w-4 shrink-0" />
          <span className="min-w-0 flex-1 truncate">{mailbox.label}</span>
          {mailbox.unread ? <span className={selected ? "font-bold text-[#1b47a0]" : "text-muted"}>{mailbox.unread}</span> : null}
        </button>
      </div>
      {open
        ? mailbox.children?.map((child) => (
            <MailboxItem key={child.id} mailbox={child} selectedId={selectedId} level={level + 1} onSelect={onSelect} />
          ))
        : null}
    </div>
  );
}

function containsSelectedMailbox(mailboxes: Mailbox[], selectedId: string): boolean {
  return mailboxes.some((mailbox) => mailbox.id === selectedId || containsSelectedMailbox(mailbox.children ?? [], selectedId));
}
