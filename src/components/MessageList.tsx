import type { Message, MockMode } from "../types";
import { Icon } from "./Icon";
import { EmptyState, ErrorState, LoadingState } from "./StateViews";
import { MessageRow } from "./MessageRow";

type MessageListProps = {
  title: string;
  unreadCount: number;
  messages: Message[];
  selectedId?: string;
  checkedIds: Set<string>;
  search: string;
  mode: MockMode;
  onRetry: () => void;
  onSelectMessage: (id: string) => void;
  onToggleChecked: (id: string) => void;
  onClearChecked: () => void;
};

export function MessageList({ title, unreadCount, messages, selectedId, checkedIds, search, mode, onRetry, onSelectMessage, onToggleChecked, onClearChecked }: MessageListProps) {
  const checkedCount = checkedIds.size;

  return (
    <section className="flex min-w-[320px] shrink-0 flex-col border-r border-line bg-white" style={{ width: "var(--list-width)" }}>
      {checkedCount > 0 ? (
        <div className="m-3 mb-0 flex h-10 items-center rounded-lg bg-selected px-3 text-accent">
          <input className="h-[15px] w-[15px] accent-accent" checked readOnly type="checkbox" />
          <span className="ml-3 text-[13px] font-medium">{checkedCount}개 선택됨</span>
          <div className="ml-auto flex gap-5 text-[#3a3f45]">
            <button aria-label="선택 메일 보관">
              <Icon name="archive" className="h-4 w-4" />
            </button>
            <button aria-label="선택 메일 삭제">
              <Icon name="trash" className="h-4 w-4" />
            </button>
            <button aria-label="선택 메일 이동">
              <Icon name="folder" className="h-4 w-4" />
            </button>
            <button aria-label="선택 해제" onClick={onClearChecked}>
              <Icon name="close" className="h-4 w-4" />
            </button>
          </div>
        </div>
      ) : (
        <div className="flex h-11 items-center border-b border-line px-4">
          <input className="h-[15px] w-[15px] accent-accent" type="checkbox" aria-label="전체 선택" />
          <div className="ml-3 truncate text-[13.5px] font-bold text-ink">{search ? `'${search}' 결과 ${messages.length}건` : title}</div>
          <div className="ml-auto text-[11.5px] text-muted">{search ? "현재 계정 전체" : `안읽음 ${unreadCount}`}</div>
          <button className="ml-4 text-muted" aria-label="메일 목록 새로고침">
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
              />
            ))
          : null}
      </div>
    </section>
  );
}
