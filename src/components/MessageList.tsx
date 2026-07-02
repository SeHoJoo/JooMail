import { useState } from "react";
import type { Mailbox, Message, MockMode } from "../types";
import { Icon } from "./Icon";
import { EmptyState, ErrorState, LoadingState } from "./StateViews";
import { MessageRow } from "./MessageRow";

type MessageListProps = {
  title: string;
  unreadCount: number;
  messages: Message[];
  mailboxes: Mailbox[];
  selectedId?: string;
  checkedIds: Set<string>;
  search: string;
  mode: MockMode;
  onRetry: () => void;
  onSelectMessage: (id: string) => void;
  onToggleAllChecked: () => void;
  onToggleChecked: (id: string) => void;
  onToggleFlagged: (message: Message) => void;
  onBulkArchive: () => Promise<void> | void;
  onBulkTrash: () => Promise<void> | void;
  onBulkMove: (mailboxId: string) => Promise<void> | void;
  onClearChecked: () => void;
};

export function MessageList({ title, unreadCount, messages, mailboxes, selectedId, checkedIds, search, mode, onRetry, onSelectMessage, onToggleAllChecked, onToggleChecked, onToggleFlagged, onBulkArchive, onBulkTrash, onBulkMove, onClearChecked }: MessageListProps) {
  const checkedCount = checkedIds.size;
  const [folderMenuOpen, setFolderMenuOpen] = useState(false);
  const [actionError, setActionError] = useState("");
  const allVisibleChecked = messages.length > 0 && messages.every((message) => checkedIds.has(message.id));
  const moveTargets = mailboxes.filter((mailbox) => mailbox.kind !== "starred");

  return (
    <section className="flex min-w-[320px] shrink-0 flex-col border-r border-line bg-white" style={{ width: "var(--list-width)" }}>
      {checkedCount > 0 ? (
        <div className="flex h-11 items-center border-b border-line bg-selected px-4 text-accent">
          <input className="h-[15px] w-[15px] accent-accent" aria-label="선택 해제" checked onChange={onClearChecked} type="checkbox" />
          <span className="ml-3 text-[13px] font-medium">{checkedCount}개 선택됨</span>
          <div className="ml-auto flex gap-4 text-[#3a3f45]">
            <button aria-label="선택 메일 보관" onClick={() => runBulkAction(onBulkArchive, setActionError, onClearChecked)} type="button">
              <Icon name="archive" className="h-4 w-4" />
            </button>
            <button aria-label="선택 메일 삭제" onClick={() => runBulkAction(onBulkTrash, setActionError, onClearChecked)} type="button">
              <Icon name="trash" className="h-4 w-4" />
            </button>
            <div className="relative">
              <button
                aria-label="선택 메일 이동"
                onClick={() => {
                  setActionError("");
                  setFolderMenuOpen((open) => !open);
                }}
                type="button"
              >
                <Icon name="folder" className="h-4 w-4" />
              </button>
              {folderMenuOpen ? (
                <div className="absolute right-0 top-7 z-20 w-[180px] rounded-lg border border-line bg-white py-1 text-[12.5px] text-text shadow-compose">
                  {moveTargets.length ? (
                    moveTargets.map((mailbox) => (
                      <button key={mailbox.id} className="w-full truncate px-3 py-2 text-left hover:bg-[#f6f7f8]" onClick={() => runBulkAction(() => onBulkMove(mailbox.id), setActionError, onClearChecked)} type="button">
                        {mailbox.label}
                      </button>
                    ))
                  ) : (
                    <div className="px-3 py-2 text-muted">이동할 폴더 없음</div>
                  )}
                </div>
              ) : null}
            </div>
            <button aria-label="선택 해제" onClick={onClearChecked} type="button">
              <Icon name="close" className="h-4 w-4" />
            </button>
          </div>
          {actionError ? <div className="absolute mt-12 text-[12px] font-medium text-[#b23a30]">{actionError}</div> : null}
        </div>
      ) : (
        <div className="flex h-11 items-center border-b border-line px-4">
          <input className="h-[15px] w-[15px] accent-accent" checked={allVisibleChecked} disabled={!messages.length} onChange={onToggleAllChecked} type="checkbox" aria-label={allVisibleChecked ? "전체 선택 해제" : "전체 선택"} />
          <div className="ml-3 truncate text-[13.5px] font-bold text-ink">{search ? `'${search}' 결과 ${messages.length}건` : title}</div>
          <div className="ml-auto text-[11.5px] text-muted">{search ? "현재 계정 전체" : `안읽음 ${unreadCount}`}</div>
          <button className="ml-4 text-muted" aria-label="메일 목록 새로고침" onClick={onRetry} type="button">
            <Icon name="refresh" className="h-[15px] w-[15px]" />
          </button>
        </div>
      )}
      {search ? (
        <div className="border-b border-line px-4 py-2 text-[11.5px] text-muted">
          검색 범위: 현재 계정 전체
        </div>
      ) : null}
      <div className="scrollbar-thin min-h-0 flex-1 overflow-y-auto">
        {mode === "loading" ? <LoadingState /> : null}
        {mode === "error" ? <ErrorState onRetry={onRetry} /> : null}
        {mode === "normal" && messages.length === 0 ? (
          <EmptyState title={search ? "검색 결과가 없습니다" : "받은편지함이 비어 있습니다"} description={search ? "검색어를 확인해주세요" : "새 메일이 도착하면 여기에 표시됩니다"} />
        ) : null}
        {mode === "normal"
          ? messages.map((message) => (
              <MessageRow
                key={message.id}
                message={message}
                selected={selectedId === message.id}
                checked={checkedIds.has(message.id)}
                search={search}
                onSelect={() => onSelectMessage(message.id)}
                onToggleChecked={() => onToggleChecked(message.id)}
                onToggleFlagged={() => onToggleFlagged(message)}
              />
            ))
          : null}
      </div>
    </section>
  );
}

async function runBulkAction(action: () => Promise<void> | void, setActionError: (value: string) => void, onDone: () => void) {
  setActionError("");
  try {
    await action();
    onDone();
  } catch {
    setActionError("작업을 완료하지 못했습니다.");
  }
}
