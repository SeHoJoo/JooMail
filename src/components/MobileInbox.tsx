import { useEffect, useState } from "react";
import type { Account, Message, MockMode } from "../types";
import { Icon } from "./Icon";
import { highlight } from "./MessageRow";
import { EmptyState, ErrorState, LoadingState } from "./StateViews";

type MobileInboxProps = {
  account: Account;
  messages: Message[];
  selectedMessage?: Message;
  selectedId?: string;
  checkedIds: Set<string>;
  search: string;
  mode: MockMode;
  onRetry: () => void;
  onCompose: () => void;
  onSelectMessage: (id: string) => void;
  onToggleChecked: (id: string) => void;
};

export function MobileInbox({ account, messages, selectedMessage, selectedId, checkedIds, search, mode, onRetry, onCompose, onSelectMessage, onToggleChecked }: MobileInboxProps) {
  const [readingId, setReadingId] = useState("");
  const title = search ? "검색 결과" : "받은편지함";
  const checkedCount = checkedIds.size;
  const selecting = checkedCount > 0;
  const count = search ? messages.length : messages.length === 0 && mode === "normal" ? 0 : account.unread;
  const emptyTitle = search ? "검색 결과가 없습니다" : "받은편지함이 비어 있습니다";
  const emptyDescription = search ? "검색어를 확인해주세요" : "새 메일이 도착하면 여기에 표시됩니다";
  const readingMessage = readingId && selectedMessage?.id === readingId ? selectedMessage : messages.find((message) => message.id === readingId);

  useEffect(() => {
    if (readingId && !messages.some((message) => message.id === readingId)) {
      setReadingId("");
    }
  }, [messages, readingId]);

  if (readingMessage) {
    return <MobileReadingPane message={readingMessage} onBack={() => setReadingId("")} onCompose={onCompose} />;
  }

  return (
    <main className="min-h-screen bg-white pb-28 md:hidden">
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
        <h1 className="text-2xl font-bold text-ink">{title}</h1>
        {count > 0 || search ? <span className="ml-2 text-[15px] font-medium text-accent">{count}</span> : null}
        <Icon name="search" className="ml-auto h-[18px] w-[18px] text-ink" />
      </div>
      {search ? <div className="px-6 pt-2 text-[12.5px] text-muted">"{search}"에 대한 결과 {messages.length}건 · 현재 계정 전체</div> : null}
      {checkedCount > 0 ? (
        <div className="mt-5 flex h-11 items-center border-y border-line bg-selected px-6 text-accent">
          <input className="h-[15px] w-[15px] accent-accent" checked readOnly type="checkbox" />
          <span className="ml-3 text-[13px] font-medium">{checkedCount}개 선택됨</span>
        </div>
      ) : null}
      <div className="mt-7">
        {mode === "loading" ? (
          <div className="h-[calc(100vh-166px)]">
            <LoadingState />
          </div>
        ) : null}
        {mode === "error" ? (
          <div className="h-[calc(100vh-166px)]">
            <ErrorState onRetry={onRetry} />
          </div>
        ) : null}
        {mode === "normal" && messages.length === 0 ? (
          <div className="h-[calc(100vh-166px)] px-6">
            <EmptyState title={emptyTitle} description={emptyDescription} />
          </div>
        ) : null}
        {mode === "normal" ? messages.slice(0, 8).map((message) => {
          const checked = checkedIds.has(message.id);
          return (
          <button
            key={message.id}
            className={[
              "relative flex h-[94px] w-full border-b border-line text-left focus-visible:z-10 focus-visible:outline-offset-[-2px]",
              selectedId === message.id ? "bg-selected" : "bg-white",
              checked ? "bg-selected/70" : "",
            ].join(" ")}
            data-message-id={message.id}
            onClick={() => {
              onSelectMessage(message.id);
              setReadingId(message.id);
            }}
          >
            {selectedId === message.id ? <span className="absolute left-0 top-0 h-full w-0.5 bg-accent" /> : null}
            {message.unread ? <span className="absolute left-[50px] top-[30px] h-[7px] w-[7px] rounded-full bg-accent" /> : null}
            <span className={selecting ? "block" : "hidden"}>
              <input
                aria-label={`${message.sender} 선택`}
                className="absolute left-[18px] top-[39px] h-[15px] w-[15px] accent-accent"
                checked={checked}
                onClick={(event) => event.stopPropagation()}
                onChange={() => onToggleChecked(message.id)}
                type="checkbox"
              />
            </span>
            <span className="absolute left-[60px] top-2 flex h-11 w-11 items-center justify-center rounded-full bg-selected text-[13px] font-bold text-accent data-[muted=true]:bg-[#e6e8eb] data-[muted=true]:text-[#828891]" data-muted={!message.unread}>
              {message.initials}
            </span>
            <span className="absolute left-[108px] right-[84px] top-2 truncate text-[15px] text-ink data-[unread=true]:font-bold data-[unread=false]:font-semibold" data-unread={message.unread}>
              {message.sender}
            </span>
            <span className="absolute right-6 top-2 text-[12.5px] data-[unread=true]:font-medium data-[unread=true]:text-accent data-[unread=false]:text-muted" data-unread={message.unread}>
              {message.time}
            </span>
            <span className="absolute left-[108px] right-12 top-8 truncate text-[13.5px] text-ink">{highlight(message.subject, search)}</span>
            {message.hasAttachment ? <Icon name="paperclip" className="absolute right-8 top-[33px] h-3.5 w-3.5 text-muted" /> : null}
            <span className="absolute left-[108px] right-6 top-[55px] truncate text-[12.5px] text-[#a2a8b0]">{highlight(message.snippet, search)}</span>
          </button>
          );
        }) : null}
      </div>
      <button className="fixed bottom-10 right-6 flex h-14 w-14 items-center justify-center rounded-[18px] bg-accent text-white shadow-[0_8px_20px_rgba(45,100,216,0.42)]" aria-label="새 메일 쓰기" onClick={onCompose}>
        <Icon name="compose" className="h-[22px] w-[22px]" />
      </button>
    </main>
  );
}

function MobileReadingPane({ message, onBack, onCompose }: { message: Message; onBack: () => void; onCompose: () => void }) {
  return (
    <main className="min-h-screen bg-white pb-24 md:hidden">
      <div className="flex h-12 items-center px-6 pt-2 text-sm font-bold text-ink">
        <span>9:14</span>
        <span className="ml-auto text-xs font-medium">▮▮  Wi-Fi  ▭</span>
      </div>
      <div className="flex h-14 items-center border-b border-line px-4">
        <button className="flex h-9 w-9 items-center justify-center rounded-lg text-ink hover:bg-[#f6f7f8]" aria-label="메일 목록으로 돌아가기" onClick={onBack} type="button">
          <Icon name="chevron" className="h-4 w-4 rotate-90" />
        </button>
        <div className="ml-2 min-w-0 flex-1 truncate text-[13px] font-semibold text-ink">{message.subject}</div>
        <button className="ml-2 flex h-9 w-9 items-center justify-center rounded-lg text-ink hover:bg-[#f6f7f8]" aria-label="답장 작성" onClick={onCompose} type="button">
          <Icon name="compose" className="h-4 w-4" />
        </button>
      </div>
      <article className="px-6 py-5">
        <h1 className="text-[20px] font-bold leading-7 text-ink">{message.subject}</h1>
        <div className="mt-5 flex items-start gap-3">
          <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-selected text-[13px] font-bold text-accent">{message.initials}</div>
          <div className="min-w-0 flex-1">
            <div className="truncate text-[14px] font-bold text-ink">{message.sender}</div>
            <div className="mt-0.5 truncate text-[12px] text-muted">{message.senderEmail}</div>
          </div>
          <div className="shrink-0 text-right text-[12px] text-muted">{message.time}</div>
        </div>
        {message.htmlBody ? (
          <div className="mt-6 text-[14px] leading-6 text-text [&_a]:text-accent [&_a]:underline [&_li]:mb-1 [&_ol]:mb-4 [&_ol]:list-decimal [&_ol]:pl-5 [&_p]:mb-4 [&_ul]:mb-4 [&_ul]:list-disc [&_ul]:pl-5" dangerouslySetInnerHTML={{ __html: message.htmlBody }} />
        ) : (
          <div className="mt-6 space-y-4 text-[14px] leading-6 text-text">
            {message.body.length ? message.body.map((paragraph, index) => <p key={`${index}-${paragraph}`}>{paragraph}</p>) : <p className="text-muted">본문을 불러오는 중입니다.</p>}
          </div>
        )}
        {message.attachments?.length ? (
          <div className="mt-6 space-y-2">
            {message.attachments.map((attachment) => (
              <div key={`${attachment.name}-${attachment.size}`} className="flex items-center gap-2 rounded-lg border border-line px-3 py-2 text-[12.5px] text-text">
                <Icon name="paperclip" className="h-4 w-4 shrink-0 text-muted" />
                <span className="min-w-0 flex-1 truncate">{attachment.name}</span>
                <span className="shrink-0 text-muted">{attachment.size}</span>
              </div>
            ))}
          </div>
        ) : null}
      </article>
    </main>
  );
}
