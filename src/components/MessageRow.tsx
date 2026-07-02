import type { Message } from "../types";
import { Icon } from "./Icon";

type MessageRowProps = {
  message: Message;
  selected: boolean;
  checked: boolean;
  search?: string;
  onSelect: () => void;
  onToggleChecked: () => void;
};

function highlight(text: string, search = "") {
  if (!search.trim()) return text;
  const index = text.toLowerCase().indexOf(search.toLowerCase());
  if (index === -1) return text;
  return (
    <>
      {text.slice(0, index)}
      <mark className="bg-[#ffeba0] px-0.5 text-text">{text.slice(index, index + search.length)}</mark>
      {text.slice(index + search.length)}
    </>
  );
}

export function MessageRow({ message, selected, checked, search, onSelect, onToggleChecked }: MessageRowProps) {
  return (
    <button
      className={[
        "group relative flex h-[58px] w-full items-start border-b border-line text-left",
        "focus-visible:z-10 focus-visible:bg-selected focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-[-2px] focus-visible:outline-accent",
        selected ? "bg-selected" : "bg-white hover:bg-[#fafbfc]",
        checked ? "bg-selected/70" : "",
      ].join(" ")}
      data-message-id={message.id}
      data-message-row
      data-selected={selected}
      onClick={onSelect}
      onFocus={onSelect}
    >
      {selected ? <span className="absolute left-0 top-0 h-full w-0.5 bg-accent" /> : null}
      <span className="absolute left-[10px] top-[26px] h-1.5 w-1.5 rounded-full bg-accent opacity-0 data-[show=true]:opacity-100" data-show={message.unread} />
      <span className="absolute left-6 top-2.5 flex h-7 w-7 items-center justify-center rounded-full bg-[#eaf1fd] text-[10px] font-bold text-accent data-[muted=true]:bg-[#e6e8eb] data-[muted=true]:text-[#828891]" data-muted={!message.unread}>
        {message.initials}
      </span>
      <input
        aria-label={`${message.sender} 선택`}
        className={[
          "absolute left-[15px] top-[12px] h-[15px] w-[15px] accent-accent group-hover:block",
          checked ? "block" : "hidden",
        ].join(" ")}
        checked={checked}
        onClick={(event) => event.stopPropagation()}
        onChange={onToggleChecked}
        type="checkbox"
      />
      <span className="absolute left-[62px] top-2.5 right-[84px] truncate text-[13px] text-ink data-[unread=true]:font-bold data-[unread=false]:font-medium" data-unread={message.unread}>
        {message.sender}
      </span>
      <span className="absolute right-3 top-[11px] w-[64px] text-right text-[11px] text-[#9aa0a8]">{message.time}</span>
      <span className="absolute left-[62px] right-[50px] top-[27px] truncate text-[12.5px] text-[#17191c] data-[unread=false]:text-[#5b6169]" data-unread={message.unread}>
        {highlight(message.subject, search)}
      </span>
      <span className="absolute left-[62px] right-[50px] top-[43px] truncate text-[11.5px] text-[#a2a8b0]">{highlight(message.snippet, search)}</span>
      {message.hasAttachment ? <Icon name="paperclip" className="absolute right-[52px] top-[28px] h-[13px] w-[13px] text-muted" /> : null}
      {message.flagged ? <Icon name="star" className="absolute right-[31px] top-[28px] h-[13px] w-[13px] fill-[#f5b514] text-[#f5b514]" /> : null}
    </button>
  );
}
