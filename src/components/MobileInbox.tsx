import { useEffect, useRef, useState } from "react";
import { useVirtualizer } from "@tanstack/react-virtual";
import type { Account, Mailbox, Message, MockMode, SearchScope } from "../types";
import { Icon } from "./Icon";
import { highlight } from "./MessageRow";
import { EmptyState, ErrorState, LoadingState } from "./StateViews";
import { attachmentURL, MailHTMLBody, renderTextWithLinks } from "./mailRendering";

const ACCOUNT_SEARCH_RESULT_CAP = 50;
const MOBILE_ROW_HEIGHT = 94;
const SCROLL_KEY_PREFIX = "joomail:mobile-list-scroll:";

type MobileInboxProps = {
  account: Account;
  messages: Message[];
  selectedMessage?: Message;
  selectedId?: string;
  selectedMailboxId: string;
  checkedIds: Set<string>;
  search: string;
  searchInput: string;
  searchScope: SearchScope;
  mode: MockMode;
  scrollKey: string;
  showRemoteImagesByDefault: boolean;
  forceReadingId?: string;
  folderMenuOpenByDefault?: boolean;
  onRetry: () => void;
  onCompose: () => void;
  onReply: (messageId: string) => void;
  onSearch: (value: string) => void;
  onSearchScopeChange: (scope: SearchScope) => void;
  onSelectMessage: (id: string) => void;
  onSelectMailbox: (id: string) => void;
  onToggleChecked: (id: string) => void;
  onToggleFlagged: (message: Message) => void;
  onClearChecked: () => void;
  onBulkArchive: () => Promise<void> | void;
  onBulkTrash: () => Promise<void> | void;
  onLogout?: () => void;
};

export function MobileInbox({ account, messages, selectedMessage, selectedId, selectedMailboxId, checkedIds, search, searchInput, searchScope, mode, scrollKey, showRemoteImagesByDefault, forceReadingId = "", folderMenuOpenByDefault = false, onRetry, onCompose, onReply, onSearch, onSearchScopeChange, onSelectMessage, onSelectMailbox, onToggleChecked, onToggleFlagged, onClearChecked, onBulkArchive, onBulkTrash, onLogout }: MobileInboxProps) {
  const [readingId, setReadingId] = useState(forceReadingId);
  const [folderMenuOpen, setFolderMenuOpen] = useState(folderMenuOpenByDefault);
  const [searchOpen, setSearchOpen] = useState(Boolean(search || searchInput));
  const scrollRef = useRef<HTMLDivElement>(null);
  const mailboxTitle = findMailbox(account.mailboxes, selectedMailboxId)?.label ?? "받은편지함";
  const title = search ? "검색 결과" : mailboxTitle;
  const checkedCount = checkedIds.size;
  const selecting = checkedCount > 0;
  const count = search ? messages.length : messages.length === 0 && mode === "normal" ? 0 : account.unread;
  const emptyTitle = search ? "검색 결과가 없습니다" : "받은편지함이 비어 있습니다";
  const emptyDescription = search ? "검색어를 확인해주세요" : "새 메일이 도착하면 여기에 표시됩니다";
  const readingMessage = readingId && selectedMessage?.id === readingId ? selectedMessage : messages.find((message) => message.id === readingId);
  const accountSearchCapped = Boolean(search) && searchScope === "account" && messages.length >= ACCOUNT_SEARCH_RESULT_CAP;
  const virtualizer = useVirtualizer({
    count: messages.length,
    getScrollElement: () => scrollRef.current,
    estimateSize: () => MOBILE_ROW_HEIGHT,
    overscan: 6,
  });
  const virtualRows = virtualizer.getVirtualItems();
  const renderedRows = virtualRows.length
    ? virtualRows
    : messages.map((_, index) => ({
        index,
        size: MOBILE_ROW_HEIGHT,
        start: index * MOBILE_ROW_HEIGHT,
      }));

  useEffect(() => {
    if (readingId && !messages.some((message) => message.id === readingId)) {
      setReadingId("");
    }
  }, [messages, readingId]);

  useEffect(() => {
    setReadingId(forceReadingId);
  }, [forceReadingId]);

  useEffect(() => {
    setFolderMenuOpen(folderMenuOpenByDefault);
  }, [folderMenuOpenByDefault]);

  useEffect(() => {
    if (search || searchInput) setSearchOpen(true);
  }, [search, searchInput]);

  useEffect(() => {
    const top = Number(localStorage.getItem(`${SCROLL_KEY_PREFIX}${scrollKey}`));
    if (Number.isFinite(top) && scrollRef.current) {
      scrollRef.current.scrollTop = top;
    }
  }, [scrollKey]);

  function saveScrollPosition() {
    if (!scrollRef.current) return;
    localStorage.setItem(`${SCROLL_KEY_PREFIX}${scrollKey}`, String(scrollRef.current.scrollTop));
  }

  if (readingMessage) {
    return <MobileReadingPane message={readingMessage} showRemoteImagesByDefault={showRemoteImagesByDefault} onBack={() => setReadingId("")} onReply={onReply} onToggleFlagged={onToggleFlagged} />;
  }

  return (
    <main className="min-h-screen bg-white pb-28 md:hidden">
      <div className="flex items-center gap-5 px-6 pt-5">
        <button className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg text-ink hover:bg-[#f6f7f8]" aria-label="폴더 열기" onClick={() => setFolderMenuOpen(true)} type="button">
          <Icon name="menu" className="h-[18px] w-[18px]" />
        </button>
        <div className="flex min-w-0 items-center gap-2 rounded-full border border-line px-2 py-1.5 text-[12.5px] font-medium text-ink">
          <span className="flex h-[22px] w-[22px] shrink-0 items-center justify-center rounded-full bg-accent text-[9px] font-bold text-white">{account.initials}</span>
          <span className="truncate">{account.email}</span>
        </div>
      </div>
      {folderMenuOpen ? (
        <MobileFolderDrawer
          account={account}
          selectedMailboxId={selectedMailboxId}
          onClose={() => setFolderMenuOpen(false)}
          onSelectMailbox={(id) => {
            onSelectMailbox(id);
            setFolderMenuOpen(false);
          }}
          onLogout={onLogout}
        />
      ) : null}
      <div className="flex items-center px-6 pt-6">
        <h1 className="text-2xl font-bold text-ink">{title}</h1>
        {count > 0 || search ? <span className="ml-2 text-[15px] font-medium text-accent">{count}</span> : null}
        <button className="ml-auto flex h-9 w-9 items-center justify-center rounded-lg text-ink hover:bg-[#f6f7f8]" aria-label="메일 검색" onClick={() => setSearchOpen((open) => !open)} type="button">
          <Icon name="search" className="h-[18px] w-[18px]" />
        </button>
      </div>
      {searchOpen ? (
        <div className="px-6 pt-3">
          <input
            className="h-10 w-full rounded-lg bg-[#f2f3f5] px-3 text-[13px] text-ink outline-none placeholder:text-muted"
            value={searchInput}
            onChange={(event) => onSearch(event.target.value)}
            placeholder="메일 검색"
            aria-label="메일 검색"
          />
        </div>
      ) : null}
      {search ? (
        <div className="px-6 pt-2 text-[12.5px] text-muted">
          <div>"{search}"에 대한 결과 {messages.length}건 · {searchScope === "account" ? (accountSearchCapped ? "현재 계정 · 최신 50건" : "현재 계정") : "현재 메일함"}</div>
          <div className="mt-2 inline-flex rounded-md border border-line bg-white p-0.5">
            <MobileScopeButton scope="mailbox" active={searchScope === "mailbox"} onSelect={onSearchScopeChange}>
              현재 메일함
            </MobileScopeButton>
            <MobileScopeButton scope="account" active={searchScope === "account"} onSelect={onSearchScopeChange}>
              현재 계정
            </MobileScopeButton>
          </div>
        </div>
      ) : null}
      {checkedCount > 0 ? (
        <div className="mt-5 flex h-11 items-center border-y border-line bg-selected px-6 text-accent">
          <input className="h-[15px] w-[15px] accent-accent" aria-label="선택 해제" checked onChange={onClearChecked} type="checkbox" />
          <span className="ml-3 text-[13px] font-medium">{checkedCount}개 선택됨</span>
          <button className="ml-auto" aria-label="선택 메일 보관" onClick={onBulkArchive} type="button">
            <Icon name="archive" className="h-4 w-4" />
          </button>
          <button className="ml-4" aria-label="선택 메일 삭제" onClick={onBulkTrash} type="button">
            <Icon name="trash" className="h-4 w-4" />
          </button>
        </div>
      ) : null}
      <div ref={scrollRef} className="mt-7 max-h-[calc(100vh-190px)] overflow-y-auto" onScroll={saveScrollPosition}>
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
        {mode === "normal" && messages.length ? (
          <div className="relative w-full" style={{ height: `${virtualizer.getTotalSize()}px` }}>
            {renderedRows.map((virtualRow) => {
              const message = messages[virtualRow.index];
              const checked = checkedIds.has(message.id);
              return (
                <button
                  key={message.id}
                  className={[
                    "absolute left-0 top-0 flex h-[94px] w-full border-b border-line text-left focus-visible:z-10 focus-visible:outline-offset-[-2px]",
                    selectedId === message.id ? "bg-selected" : "bg-white",
                    checked ? "bg-selected/70" : "",
                  ].join(" ")}
                  style={{ transform: `translateY(${virtualRow.start}px)` }}
                  data-message-id={message.id}
                  onClick={() => {
                    onSelectMessage(message.id);
                    setReadingId(message.id);
                  }}
                >
                  {selectedId === message.id ? <span className="absolute left-0 top-0 h-full w-0.5 bg-accent" /> : null}
                  {message.unread ? <span className="absolute left-[50px] top-[30px] h-[7px] w-[7px] rounded-full bg-accent" /> : null}
                  <span className="block">
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
                  <span className={["absolute left-[108px] top-[55px] truncate text-[12.5px] text-[#a2a8b0]", search && searchScope === "account" ? "right-[104px]" : "right-6"].join(" ")}>{highlight(message.snippet, search)}</span>
                  {search && searchScope === "account" ? <span className="absolute right-6 top-[55px] max-w-[72px] truncate rounded bg-[#f2f3f5] px-1.5 py-0.5 text-[10.5px] leading-3 text-muted">{findMailbox(account.mailboxes, message.mailboxId)?.label ?? message.mailboxId}</span> : null}
                </button>
              );
            })}
          </div>
        ) : null}
      </div>
      <button className="fixed bottom-10 right-6 flex h-14 w-14 items-center justify-center rounded-[18px] bg-accent text-white shadow-[0_8px_20px_rgba(45,100,216,0.42)]" aria-label="새 메일 쓰기" onClick={onCompose}>
        <Icon name="compose" className="h-[22px] w-[22px]" />
      </button>
    </main>
  );
}

function MobileScopeButton({ scope, active, onSelect, children }: { scope: SearchScope; active: boolean; onSelect: (scope: SearchScope) => void; children: string }) {
  return (
    <button className={active ? "rounded px-2.5 py-1 font-medium text-accent" : "rounded px-2.5 py-1 text-muted"} onClick={() => onSelect(scope)} type="button">
      {children}
    </button>
  );
}

function MobileFolderDrawer({ account, selectedMailboxId, onSelectMailbox, onClose, onLogout }: { account: Account; selectedMailboxId: string; onSelectMailbox: (id: string) => void; onClose: () => void; onLogout?: () => void }) {
  const system = account.mailboxes.filter((mailbox) => mailbox.kind !== "folder");
  const folders = account.mailboxes.filter((mailbox) => mailbox.kind === "folder");

  return (
    <div className="fixed inset-0 z-30 bg-black/20" role="presentation" onMouseDown={onClose}>
      <section className="flex h-full w-[286px] flex-col bg-panel shadow-compose" aria-label="폴더" onMouseDown={(event) => event.stopPropagation()}>
        <header className="flex h-[69px] items-center gap-3 border-b border-line px-4">
          <span className="flex h-[30px] w-[30px] shrink-0 items-center justify-center rounded-full bg-accent text-[12px] font-bold text-white">{account.initials}</span>
          <span className="min-w-0 flex-1">
            <span className="block truncate text-[13.5px] font-medium text-ink">{account.email}</span>
            <span className="block truncate text-[11.5px] text-muted">{account.label}</span>
          </span>
          <button className="flex h-8 w-8 items-center justify-center rounded-md text-muted hover:bg-white hover:text-text" aria-label="폴더 닫기" onClick={onClose} type="button">
            <Icon name="close" className="h-4 w-4" />
          </button>
        </header>
        <nav className="py-2">
          {system.map((mailbox) => (
            <MobileMailboxButton key={mailbox.id} mailbox={mailbox} selectedId={selectedMailboxId} onSelect={onSelectMailbox} />
          ))}
        </nav>
        {folders.length ? (
          <>
            <div className="mt-3 px-4 text-[11px] font-bold text-[#9aa0a8]">폴더</div>
            <nav className="mt-2 bg-white py-1">
              {folders.map((mailbox) => (
                <MobileMailboxButton key={mailbox.id} mailbox={mailbox} selectedId={selectedMailboxId} onSelect={onSelectMailbox} />
              ))}
            </nav>
          </>
        ) : null}
        {onLogout ? (
          <div className="mt-auto border-t border-line px-[11px] py-4">
            <button className="flex h-9 w-full items-center justify-center rounded-md border border-line bg-white text-[13px] font-medium text-text hover:bg-[#f7f8f9]" onClick={onLogout} type="button">
              로그아웃
            </button>
          </div>
        ) : null}
      </section>
    </div>
  );
}

function MobileMailboxButton({ mailbox, selectedId, level = 0, onSelect }: { mailbox: Mailbox; selectedId: string; level?: number; onSelect: (id: string) => void }) {
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
          "mx-[11px] flex h-9 w-[calc(100%-22px)] items-center gap-2 rounded-md px-2 text-left text-[13px]",
          selected ? "bg-selected font-medium text-[#1b47a0]" : "text-[#3a3f45]",
          selectable || hasChildren ? "hover:bg-white" : "text-muted",
        ].join(" ")}
        style={{ paddingLeft: `${10 + level * 22}px` }}
      >
        {hasChildren ? (
          <button className="flex h-6 w-6 shrink-0 items-center justify-center rounded hover:bg-[#eef1f5]" aria-label={open ? `${mailbox.label} 접기` : `${mailbox.label} 펼치기`} onClick={() => setOpen((value) => !value)} type="button">
            <Icon name="chevron" className={["h-3.5 w-3.5 text-muted", open ? "" : "-rotate-90"].join(" ")} />
          </button>
        ) : (
          <span className="h-6 w-6 shrink-0" />
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
            <MobileMailboxButton key={child.id} mailbox={child} selectedId={selectedId} level={level + 1} onSelect={onSelect} />
          ))
        : null}
    </div>
  );
}

function containsSelectedMailbox(mailboxes: Mailbox[], selectedId: string): boolean {
  return mailboxes.some((mailbox) => mailbox.id === selectedId || containsSelectedMailbox(mailbox.children ?? [], selectedId));
}

function findMailbox(mailboxes: Mailbox[], id: string): Mailbox | undefined {
  for (const mailbox of mailboxes) {
    if (mailbox.id === id) return mailbox;
    const child = findMailbox(mailbox.children ?? [], id);
    if (child) return child;
  }
  return undefined;
}

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

function MobileReadingPane({ message, showRemoteImagesByDefault, onBack, onReply, onToggleFlagged }: { message: Message; showRemoteImagesByDefault: boolean; onBack: () => void; onReply: (messageId: string) => void; onToggleFlagged: (message: Message) => void }) {
  const [showRemoteImages, setShowRemoteImages] = useState(showRemoteImagesByDefault);
  const htmlBody = message.htmlBody ? revealRemoteImages(message.htmlBody, showRemoteImages) : "";

  useEffect(() => {
    setShowRemoteImages(showRemoteImagesByDefault);
  }, [message.id, showRemoteImagesByDefault]);

  return (
    <main className="min-h-screen bg-white pb-24 md:hidden">
      <div className="flex h-14 items-center border-b border-line px-4 pt-2">
        <button className="flex h-9 w-9 items-center justify-center rounded-lg text-ink hover:bg-[#f6f7f8]" aria-label="메일 목록으로 돌아가기" onClick={onBack} type="button">
          <Icon name="chevron" className="h-4 w-4 rotate-90" />
        </button>
        <div className="ml-2 min-w-0 flex-1 truncate text-[13px] font-semibold text-ink">{message.subject}</div>
        <button className="ml-2 flex h-9 w-9 items-center justify-center rounded-lg text-ink hover:bg-[#f6f7f8]" aria-label={message.flagged ? "중요 표시 해제" : "중요 표시"} onClick={() => onToggleFlagged(message)} type="button">
          <Icon name="star" className={message.flagged ? "h-4 w-4 fill-[#f5b514] text-[#f5b514]" : "h-4 w-4 text-muted"} />
        </button>
        <button className="ml-2 flex h-9 w-9 items-center justify-center rounded-lg text-ink hover:bg-[#f6f7f8]" aria-label="답장 작성" onClick={() => onReply(message.id)} type="button">
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
        {message.remoteImagesBlocked && !showRemoteImages ? (
          <div className="mt-5 flex min-h-10 items-center rounded-lg bg-[#f7f8f9] px-3 py-2 text-[12.5px] text-text">
            <Icon name="image" className="mr-3 h-4 w-4 shrink-0 text-muted" />
            <span className="min-w-0 flex-1">원격 이미지가 차단되었습니다.</span>
            <button className="ml-3 shrink-0 font-medium text-accent" onClick={() => setShowRemoteImages(true)} type="button">
              이미지 표시
            </button>
          </div>
        ) : null}
        {message.remoteImagesBlocked && showRemoteImages ? (
          <div className="mt-5 flex min-h-10 items-center rounded-lg bg-selected px-3 py-2 text-[12.5px] text-accent">
            <Icon name="image" className="mr-3 h-4 w-4 shrink-0" />
            원격 이미지 표시됨
          </div>
        ) : null}
        {message.htmlBody ? (
          <MailHTMLBody html={htmlBody} className="mt-6" />
        ) : (
          <div className="mt-6 space-y-4 text-[14px] leading-6 text-text">
            {message.body.length ? message.body.map((paragraph, index) => <p key={`${index}-${paragraph}`}>{renderTextWithLinks(paragraph)}</p>) : <p className="text-muted">본문을 불러오는 중입니다.</p>}
          </div>
        )}
        {message.attachments?.length ? (
          <div className="mt-6 space-y-2">
            {message.attachments.map((attachment) => {
              const href = attachmentURL(message, attachment);
              return (
              <a key={`${attachment.id ?? attachment.name}-${attachment.size}`} className="flex items-center gap-2 rounded-lg border border-line px-3 py-2 text-[12.5px] text-text" href={href} download={attachment.name}>
                <span className="flex h-9 w-9 shrink-0 items-center justify-center overflow-hidden rounded-md bg-[#eaf0f6] text-accent">
                  {attachment.type === "image" && href ? <img className="h-full w-full object-cover" src={href} alt="" loading="lazy" /> : <Icon name="paperclip" className="h-4 w-4" />}
                </span>
                <span className="min-w-0 flex-1 truncate">{attachment.name}</span>
                <span className="shrink-0 text-muted">{attachment.size}</span>
              </a>
              );
            })}
          </div>
        ) : null}
      </article>
    </main>
  );
}

function revealRemoteImages(html: string, show: boolean) {
  if (!show) return html;
  return html.replace(/\sdata-joomail-remote-src=/gi, " src=");
}
