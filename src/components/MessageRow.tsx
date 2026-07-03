import type { Message } from "../types";
import { Icon } from "./Icon";

type MessageRowProps = {
  message: Message;
  selected: boolean;
  checked: boolean;
  search?: string;
  metaLabel?: string;
  onSelect: (options?: { shift: boolean; toggle: boolean }) => void;
  onToggleChecked: () => void;
  onToggleFlagged: () => void;
  onArchive: () => Promise<void> | void;
  onTrash: () => Promise<void> | void;
  onMarkUnread: () => Promise<void> | void;
};

export function highlight(text: string, search = "") {
  const query = search.trim();
  if (!query) return text;
  const lowerText = text.toLowerCase();
  const lowerQuery = query.toLowerCase();
  const parts: React.ReactNode[] = [];
  let cursor = 0;
  let matchIndex = lowerText.indexOf(lowerQuery);
  while (matchIndex !== -1) {
    if (matchIndex > cursor) parts.push(text.slice(cursor, matchIndex));
    const end = matchIndex + query.length;
    parts.push(
      <mark key={`${matchIndex}-${end}`} className="bg-[#ffeba0] px-0.5 text-text">
        {text.slice(matchIndex, end)}
      </mark>,
    );
    cursor = end;
    matchIndex = lowerText.indexOf(lowerQuery, cursor);
  }
  if (cursor < text.length) parts.push(text.slice(cursor));
  return parts.length ? <>{parts}</> : text;
}

export function MessageRow({ message, selected, checked, search, metaLabel, onSelect, onToggleChecked, onToggleFlagged, onArchive, onTrash, onMarkUnread }: MessageRowProps) {
  function runRowAction(event: React.MouseEvent<HTMLButtonElement>, action: () => Promise<void> | void) {
    event.stopPropagation();
    Promise.resolve(action()).catch(() => undefined);
  }

  return (
    <div
      className={[
        "group relative flex h-[64px] w-full items-start border-b border-line text-left",
        "focus-visible:z-10 focus-visible:bg-selected focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-[-2px] focus-visible:outline-accent",
        selected ? "bg-selected" : "bg-white hover:bg-[#fafbfc]",
        checked ? "bg-selected/70" : "",
      ].join(" ")}
      data-message-id={message.id}
      data-message-row
      data-selected={selected}
      onClick={(event) => onSelect({ shift: event.shiftKey, toggle: event.metaKey || event.ctrlKey })}
      onFocus={(event) => {
        if (event.target === event.currentTarget) onSelect();
      }}
      onKeyDown={(event) => {
        if (event.key === "Enter" || event.key === " ") {
          event.preventDefault();
          onSelect();
        }
      }}
      role="button"
      tabIndex={0}
    >
      {selected ? <span className="absolute left-0 top-0 h-full w-0.5 bg-accent" /> : null}
      <span className="absolute left-[29px] top-[29px] h-1.5 w-1.5 rounded-full bg-accent opacity-0 data-[show=true]:opacity-100" data-show={message.unread} />
      <input
        aria-label={`${message.sender} 선택`}
        className={[
          "absolute left-2 top-[24px] h-[15px] w-[15px] accent-accent group-hover:block",
          checked ? "block" : "hidden",
        ].join(" ")}
        checked={checked}
        onClick={(event) => event.stopPropagation()}
        onChange={onToggleChecked}
        type="checkbox"
      />
      <span className="absolute left-[43px] top-[11px] flex h-[30px] w-[30px] items-center justify-center rounded-full bg-[#eaf1fd] text-[10px] font-bold text-accent data-[muted=true]:bg-[#e6e8eb] data-[muted=true]:text-[#828891]" data-muted={!message.unread}>
        {message.initials}
      </span>
      <span className="absolute left-[82px] top-[9px] right-[84px] truncate text-[13px] leading-4 text-ink data-[unread=true]:font-bold data-[unread=false]:font-medium" data-unread={message.unread}>
        {message.sender}
      </span>
      <span className="absolute right-3 top-[11px] w-[64px] text-right text-[11px] text-[#9aa0a8]">{message.time}</span>
      <span className="absolute left-[82px] right-[50px] top-[29px] truncate text-[12.5px] leading-[15px] text-[#17191c] data-[unread=false]:text-[#5b6169]" data-unread={message.unread}>
        {highlight(message.subject, search)}
      </span>
      <span className={["absolute left-[82px] top-[46px] truncate text-[11.5px] leading-[15px] text-[#a2a8b0]", metaLabel ? "right-[112px]" : "right-[50px]"].join(" ")}>{highlight(message.snippet, search)}</span>
      {metaLabel ? <span className="absolute right-3 top-[46px] max-w-[86px] truncate rounded bg-[#f2f3f5] px-1.5 py-0.5 text-[10.5px] leading-3 text-muted">{metaLabel}</span> : null}
      {message.hasAttachment ? <Icon name="paperclip" className="absolute right-[52px] top-[30px] h-[13px] w-[13px] text-muted" /> : null}
      <div className="absolute right-2 top-[23px] z-10 hidden h-[24px] items-center rounded-md border border-line bg-white shadow-sm group-hover:flex group-focus-within:flex">
        <button className="flex h-[22px] w-[24px] items-center justify-center rounded-l-md hover:bg-[#eef1f5]" aria-label="보관" onClick={(event) => runRowAction(event, onArchive)} type="button">
          <Icon name="archive" className="h-[13px] w-[13px] text-muted" />
        </button>
        <button className="flex h-[22px] w-[24px] items-center justify-center hover:bg-[#eef1f5]" aria-label="삭제" onClick={(event) => runRowAction(event, onTrash)} type="button">
          <Icon name="trash" className="h-[13px] w-[13px] text-muted" />
        </button>
        <button className="flex h-[22px] w-[24px] items-center justify-center rounded-r-md hover:bg-[#eef1f5]" aria-label="읽지 않음으로 표시" onClick={(event) => runRowAction(event, onMarkUnread)} type="button">
          <Icon name="mail" className="h-[13px] w-[13px] text-muted" />
        </button>
      </div>
      <button
        className={[
          "absolute right-[27px] top-[26px] flex h-[21px] w-[21px] items-center justify-center rounded-md hover:bg-[#eef1f5]",
          message.flagged ? "opacity-100" : "opacity-0 group-hover:opacity-100 focus:opacity-100",
        ].join(" ")}
        aria-label={message.flagged ? "중요 표시 해제" : "중요 표시"}
        onClick={(event) => {
          event.stopPropagation();
          onToggleFlagged();
        }}
        type="button"
      >
        <Icon name="star" className={message.flagged ? "h-[13px] w-[13px] fill-[#f5b514] text-[#f5b514]" : "h-[13px] w-[13px] text-muted"} />
      </button>
    </div>
  );
}
