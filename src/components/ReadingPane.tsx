import { useEffect, useState, type ReactNode } from "react";
import type { Mailbox, Message, MockMode } from "../types";
import { EmptyState, ErrorState, LoadingState } from "./StateViews";
import { Icon } from "./Icon";

type ReadingPaneProps = {
  message?: Message;
  mode: MockMode;
  mailboxes: Mailbox[];
  showRemoteImagesByDefault: boolean;
  onRetry: () => void;
  onReply: () => void;
  onReplyAll: () => void;
  onForward: () => void;
  onToggleFlagged: (message: Message) => void;
  onArchive: (message: Message) => Promise<void> | void;
  onTrash: (message: Message) => Promise<void> | void;
  onMove: (message: Message, mailboxId: string) => Promise<void> | void;
  onMarkUnread: (message: Message) => Promise<void> | void;
};

export function ReadingPane({ message, mode, mailboxes, showRemoteImagesByDefault, onRetry, onReply, onReplyAll, onForward, onToggleFlagged, onArchive, onTrash, onMove, onMarkUnread }: ReadingPaneProps) {
  const [recipientsOpen, setRecipientsOpen] = useState(false);
  const [quotedOpen, setQuotedOpen] = useState(false);
  const [showRemoteImages, setShowRemoteImages] = useState(showRemoteImagesByDefault);
  const [folderMenuOpen, setFolderMenuOpen] = useState(false);
  const [moreMenuOpen, setMoreMenuOpen] = useState(false);
  const [actionError, setActionError] = useState("");
  const htmlBody = message?.htmlBody ? revealRemoteImages(message.htmlBody, showRemoteImages) : "";
  const moveTargets = message ? mailboxes.filter((mailbox) => mailbox.id !== message.mailboxId && mailbox.kind !== "starred" && mailbox.selectable !== false) : [];

  useEffect(() => {
    setRecipientsOpen(false);
    setQuotedOpen(false);
    setShowRemoteImages(showRemoteImagesByDefault);
    setFolderMenuOpen(false);
    setMoreMenuOpen(false);
    setActionError("");
  }, [message?.id, showRemoteImagesByDefault]);

  if (mode === "loading") return <section className="hidden min-w-0 flex-1 bg-white md:block"><LoadingState /></section>;
  if (mode === "error") return <section className="hidden min-w-0 flex-1 bg-white md:block"><ErrorState onRetry={onRetry} /></section>;
  if (!message) {
    return (
      <section className="hidden min-w-0 flex-1 bg-white md:block">
        <EmptyState title="메일을 선택하세요" description="왼쪽 목록에서 메일을 열면 여기에 표시됩니다" />
      </section>
    );
  }

  return (
    <section className="scrollbar-thin hidden min-w-0 flex-1 overflow-y-auto bg-white md:block">
      <div className="px-[27px] pb-[15px] pt-6">
        <div className="flex items-start gap-4">
          <h1 className="min-w-0 flex-1 text-[18px] font-bold text-ink" data-reading-title>{message.subject}</h1>
          <button
            className="flex h-8 w-8 shrink-0 items-center justify-center rounded-md hover:bg-[#f7f8f9]"
            aria-label={message.flagged ? "중요 표시 해제" : "중요 표시"}
            onClick={() => onToggleFlagged(message)}
            type="button"
          >
            <Icon name="star" className={message.flagged ? "h-[18px] w-[18px] fill-[#f5b514] text-[#f5b514]" : "h-[18px] w-[18px] text-muted"} />
          </button>
        </div>
        <div className="mt-6 flex items-start gap-3">
          <div className="flex h-[38px] w-[38px] shrink-0 items-center justify-center rounded-full bg-selected text-sm font-bold text-accent">{message.initials}</div>
          <div className="min-w-0 flex-1">
            <div className="flex min-w-0 items-baseline gap-1">
              <span className="text-sm font-bold text-ink">{message.sender}</span>
              <span className="truncate text-[13px] text-muted">&lt;{message.senderEmail}&gt;</span>
            </div>
            <div className="mt-1 text-xs text-muted">
              받는사람: {formatHeaderRecipients(message.headers?.to) || "나"} ·{" "}
              <button className="font-medium text-accent" onClick={() => setRecipientsOpen((open) => !open)} type="button">
                받는사람 {recipientsOpen ? "접기" : "보기"} {recipientsOpen ? "▴" : "▾"}
              </button>
            </div>
            {recipientsOpen ? (
              <div className="mt-2 rounded-lg border border-line bg-[#f7f8f9] px-3 py-2 text-xs leading-5 text-text">
                <div>받는사람: {formatHeaderRecipients(message.headers?.to) || "나"}</div>
                {message.headers?.cc?.length ? <div>참조: {formatHeaderRecipients(message.headers.cc)}</div> : null}
                <div>보낸사람: {message.sender} &lt;{message.senderEmail}&gt;</div>
                <div>날짜: {message.fullDate}</div>
              </div>
            ) : null}
          </div>
          <div className="hidden w-[200px] text-right text-xs text-muted lg:block">
            <div>{message.fullDate}</div>
            <div className="mt-1 text-[11px]">{message.time === "오전 9:14" ? "3분 전" : message.time}</div>
          </div>
        </div>
        <div className="mt-[18px] flex items-center gap-2">
          <ActionButton icon="reply" label="답장" onClick={onReply} />
          <ActionButton icon="replyAll" label="전체답장" onClick={onReplyAll} />
          <ActionButton icon="forward" label="전달" onClick={onForward} />
          <div className="ml-auto flex gap-[5px] text-[#3a3f45]">
            <IconButton icon="archive" label="보관" onClick={() => runMessageAction(message, onArchive, setActionError, closeActionMenus)} />
            <IconButton icon="trash" label="삭제" onClick={() => runMessageAction(message, onTrash, setActionError, closeActionMenus)} />
            <div className="relative">
              <IconButton
                icon="folder"
                label="이동"
                onClick={() => {
                  setActionError("");
                  setMoreMenuOpen(false);
                  setFolderMenuOpen((open) => !open);
                }}
              />
              {folderMenuOpen ? (
                <ActionMenu className="right-0 top-8 w-[180px]">
                  {moveTargets.length ? (
                    moveTargets.map((mailbox) => (
                      <button key={mailbox.id} className="w-full truncate px-3 py-2 text-left hover:bg-[#f6f7f8]" onClick={() => runMoveAction(message, mailbox.id)} type="button">
                        {mailbox.label}
                      </button>
                    ))
                  ) : (
                    <div className="px-3 py-2 text-muted">이동할 폴더 없음</div>
                  )}
                </ActionMenu>
              ) : null}
            </div>
            <div className="relative">
              <IconButton
                icon="more"
                label="더보기"
                onClick={() => {
                  setActionError("");
                  setFolderMenuOpen(false);
                  setMoreMenuOpen((open) => !open);
                }}
              />
              {moreMenuOpen ? (
                <ActionMenu className="right-0 top-8 w-[150px]">
                  <button className="w-full px-3 py-2 text-left hover:bg-[#f6f7f8]" onClick={() => runMessageAction(message, onMarkUnread, setActionError, closeActionMenus)} type="button">
                    읽지 않음으로 표시
                  </button>
                </ActionMenu>
              ) : null}
            </div>
          </div>
        </div>
        {actionError ? <div className="mt-2 text-right text-[12px] font-medium text-[#b23a30]">{actionError}</div> : null}
      </div>
      <div className="border-t border-line" />
      <div className="px-[27px] py-[18px]">
        {message.remoteImagesBlocked && !showRemoteImages ? (
          <div className="mb-5 flex h-9 items-center rounded-lg bg-[#f7f8f9] px-3 text-[12.5px] text-text">
            <Icon name="image" className="mr-3 h-4 w-4 text-muted" />
            이 메일의 원격 이미지가 차단되었습니다.
            <button className="ml-auto font-medium text-accent" onClick={() => setShowRemoteImages(true)} type="button">
              이미지 표시
            </button>
          </div>
        ) : null}
        {message.remoteImagesBlocked && showRemoteImages ? (
          <div className="mb-5 flex h-9 items-center rounded-lg bg-selected px-3 text-[12.5px] text-accent">
            <Icon name="image" className="mr-3 h-4 w-4" />
            원격 이미지 표시됨
          </div>
        ) : null}
        {message.htmlBody ? (
          <article className="max-w-[750px] text-sm leading-[1.5] text-text [&_a]:text-accent [&_a]:underline [&_img:not([src])]:hidden [&_img]:max-w-full [&_li]:mb-1 [&_ol]:mb-5 [&_ol]:list-decimal [&_ol]:pl-5 [&_p]:mb-5 [&_ul]:mb-5 [&_ul]:list-disc [&_ul]:pl-5" dangerouslySetInnerHTML={{ __html: htmlBody }} />
        ) : (
          <article className="max-w-[750px] whitespace-pre-line text-sm leading-[1.5] text-text">
            {message.body.slice(0, 3).map((paragraph) => (
              <p key={paragraph} className="mb-5">
                {paragraph}
              </p>
            ))}
            {message.bullets ? (
              <ul className="mb-5 list-disc space-y-2 pl-5">
                {message.bullets.map((bullet) => (
                  <li key={bullet}>{bullet}</li>
                ))}
              </ul>
            ) : null}
            {message.body.slice(3).map((paragraph) => (
              <p key={paragraph} className="mb-5">
                {paragraph}
              </p>
            ))}
            {message.link ? (
              <a className="text-[13.5px] text-accent underline" href={message.link}>
                {message.link}
              </a>
            ) : null}
          </article>
        )}
        {message.attachments?.length ? (
          <div className="mt-8">
            <div className="mb-3 text-xs text-muted">첨부파일 {message.attachments.length}개 · 3.1 MB</div>
            <div className="flex flex-wrap gap-3">
              {message.attachments.map((attachment) => (
                <a key={attachment.id ?? attachment.name} className="flex h-[52px] w-[220px] items-center rounded-lg border border-line bg-white px-2 hover:bg-[#f7f8f9]" href={attachment.id ? `/api/messages/${encodeURIComponent(message.id)}/attachments/${encodeURIComponent(attachment.id)}` : undefined} download={attachment.name}>
                  <div className={attachment.type === "pdf" ? "flex h-[34px] w-[34px] items-center justify-center rounded-md bg-[#fdecec] text-[#e9564f]" : "flex h-[34px] w-[34px] items-center justify-center rounded-md bg-[#eaf0f6] text-accent"}>
                    <Icon name={attachment.type === "image" ? "image" : "mail"} className="h-4 w-4" />
                  </div>
                  <div className="ml-2 min-w-0 flex-1">
                    <div className="truncate text-[12.5px] font-medium text-ink">{attachment.name}</div>
                    <div className="text-[11px] text-muted">{attachment.size}</div>
                  </div>
                  <Icon name="download" className="h-3.5 w-3.5 text-muted" />
                </a>
              ))}
            </div>
          </div>
        ) : null}
        <button className="mt-5 flex items-center gap-2 rounded-md bg-[#f7f8f9] px-3 py-1.5 text-xs text-muted hover:text-text" onClick={() => setQuotedOpen((open) => !open)} type="button">
          <Icon name="more" className="h-3.5 w-3.5" />
          인용된 이전 대화 {quotedOpen ? "접기" : "보기"}
        </button>
        {quotedOpen ? (
          <div className="mt-3 max-w-[750px] rounded-lg border border-line bg-[#f7f8f9] px-4 py-3 text-[12.5px] leading-5 text-muted">
            <p className="mb-2">On 2026년 7월 1일, {message.sender} wrote:</p>
            <p>이전 대화 내용은 기본 접힘 상태로 유지됩니다. 이 영역은 목업 UI에서만 펼쳐집니다.</p>
          </div>
        ) : null}
      </div>
    </section>
  );

  function closeActionMenus() {
    setFolderMenuOpen(false);
    setMoreMenuOpen(false);
  }

  function runMoveAction(target: Message, mailboxId: string) {
    return runMessageAction(target, (nextMessage) => onMove(nextMessage, mailboxId), setActionError, closeActionMenus);
  }
}

function ActionButton({ icon, label, onClick }: { icon: "reply" | "replyAll" | "forward"; label: string; onClick: () => void }) {
  return (
    <button className="flex h-8 shrink-0 items-center gap-[7px] whitespace-nowrap rounded-[7px] border border-line bg-white px-[13px] text-[13px] font-medium text-text hover:border-[#d0d4d9] hover:bg-[#f6f7f8]" onClick={onClick}>
      <Icon name={icon} className="h-[15px] w-[15px] shrink-0" />
      {label}
    </button>
  );
}

function IconButton({ icon, label, onClick }: { icon: "archive" | "trash" | "folder" | "more"; label: string; onClick: () => void }) {
  return (
    <button className="flex h-[30px] w-[30px] items-center justify-center rounded-md hover:bg-[#f7f8f9]" aria-label={label} onClick={onClick} type="button">
      <Icon name={icon} className="h-[15px] w-[15px]" />
    </button>
  );
}

function ActionMenu({ children, className }: { children: ReactNode; className: string }) {
  return (
    <div className={`absolute z-20 rounded-lg border border-line bg-white py-1 text-[12.5px] text-text shadow-compose ${className}`}>
      {children}
    </div>
  );
}

async function runMessageAction(message: Message, action: (message: Message) => Promise<void> | void, setActionError: (value: string) => void, onDone: () => void) {
  setActionError("");
  try {
    await action(message);
    onDone();
  } catch {
    setActionError("작업을 완료하지 못했습니다.");
  }
}

function revealRemoteImages(html: string, show: boolean) {
  if (!show) return html;
  return html.replace(/\sdata-joomail-remote-src=/gi, " src=");
}

function formatHeaderRecipients(values: string[] | undefined) {
  return (values ?? []).map((value) => value.trim()).filter(Boolean).join(", ");
}
