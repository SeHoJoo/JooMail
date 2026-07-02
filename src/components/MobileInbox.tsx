import type { Account, Message } from "../types";
import { Icon } from "./Icon";

type MobileInboxProps = {
  account: Account;
  messages: Message[];
  onCompose: () => void;
  onSelectMessage: (id: string) => void;
};

export function MobileInbox({ account, messages, onCompose, onSelectMessage }: MobileInboxProps) {
  return (
    <main className="min-h-screen bg-white md:hidden">
      <div className="flex h-12 items-center px-6 pt-2 text-sm font-bold text-ink">
        <span>9:14</span>
        <span className="ml-auto text-xs font-medium">▮▮  Wi-Fi  ▭</span>
      </div>
      <div className="flex items-center gap-5 px-6 pt-2">
        <Icon name="menu" className="h-[18px] w-[18px] text-ink" />
        <button className="flex min-w-0 items-center gap-2 rounded-full border border-line px-2 py-1.5 text-[12.5px] font-medium text-ink">
          <span className="flex h-[22px] w-[22px] shrink-0 items-center justify-center rounded-full bg-accent text-[9px] font-bold text-white">{account.initials}</span>
          <span className="truncate">{account.email}</span>
          <Icon name="chevron" className="h-[11px] w-[11px] shrink-0 text-muted" />
        </button>
      </div>
      <div className="flex items-center px-6 pt-6">
        <h1 className="text-2xl font-bold text-ink">받은편지함</h1>
        <span className="ml-2 text-[15px] font-medium text-accent">{account.unread}</span>
        <Icon name="search" className="ml-auto h-[18px] w-[18px] text-ink" />
      </div>
      <div className="mt-8">
        {messages.slice(0, 6).map((message) => (
          <button key={message.id} className="relative flex h-[100px] w-full border-b border-line text-left" onClick={() => onSelectMessage(message.id)}>
            {message.unread ? <span className="absolute left-[22px] top-[30px] h-[7px] w-[7px] rounded-full bg-accent" /> : null}
            <span className="absolute left-8 top-2 flex h-11 w-11 items-center justify-center rounded-full bg-selected text-[13px] font-bold text-accent data-[muted=true]:bg-[#e6e8eb] data-[muted=true]:text-[#828891]" data-muted={!message.unread}>
              {message.initials}
            </span>
            <span className="absolute left-20 top-2 w-[200px] truncate text-[15px] text-ink data-[unread=true]:font-bold data-[unread=false]:font-semibold" data-unread={message.unread}>
              {message.sender}
            </span>
            <span className="absolute right-6 top-2 text-[12.5px] data-[unread=true]:font-medium data-[unread=true]:text-accent data-[unread=false]:text-muted" data-unread={message.unread}>
              {message.time}
            </span>
            <span className="absolute left-20 top-8 w-[260px] truncate text-[13.5px] text-ink">{message.subject}</span>
            {message.hasAttachment ? <Icon name="paperclip" className="absolute right-8 top-[33px] h-3.5 w-3.5 text-muted" /> : null}
            <span className="absolute left-20 top-[55px] w-[292px] truncate text-[12.5px] text-[#a2a8b0]">{message.snippet}</span>
          </button>
        ))}
      </div>
      <button className="fixed bottom-12 right-8 flex h-14 w-14 items-center justify-center rounded-full bg-accent text-white shadow-[0_6px_16px_rgba(0,0,0,0.25)]" aria-label="새 메일 쓰기" onClick={onCompose}>
        <Icon name="compose" className="h-[22px] w-[22px]" />
      </button>
    </main>
  );
}
