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

  return (
    <div>
      <button
        className={[
          "mx-[11px] flex h-8 w-[calc(100%-22px)] items-center gap-2 rounded-md px-2 text-left text-[13px]",
          selected ? "bg-selected font-medium text-[#1b47a0]" : "text-[#3a3f45]",
          selectable ? "hover:bg-white" : "cursor-default text-muted",
        ].join(" ")}
        style={{ paddingLeft: `${10 + level * 14}px` }}
        onClick={() => {
          if (selectable) onSelect(mailbox.id);
        }}
        disabled={!selectable}
        type="button"
      >
        {mailbox.children?.length ? <Icon name="chevron" className="h-3.5 w-3.5 -rotate-90 text-muted" /> : null}
        <Icon name={iconByKind[mailbox.kind]} className="h-4 w-4 shrink-0" />
        <span className="min-w-0 flex-1 truncate">{mailbox.label}</span>
        {mailbox.unread ? <span className={selected ? "font-bold text-[#1b47a0]" : "text-muted"}>{mailbox.unread}</span> : null}
      </button>
      {mailbox.children?.map((child) => (
        <MailboxItem key={child.id} mailbox={child} selectedId={selectedId} level={level + 1} onSelect={onSelect} />
      ))}
    </div>
  );
}
