import { Icon } from "./Icon";

export function EmptyState({ title, description }: { title: string; description: string }) {
  return (
    <div className="flex h-full min-h-[220px] flex-col items-center justify-center text-center">
      <div className="flex h-12 w-12 items-center justify-center rounded-full bg-[#f3f4f5] text-muted">
        <Icon name="mail" className="h-[22px] w-[22px]" />
      </div>
      <div className="mt-3 text-sm font-bold text-ink">{title}</div>
      <div className="mt-1 text-xs text-muted">{description}</div>
    </div>
  );
}

export function LoadingState() {
  return (
    <div className="space-y-6 p-5">
      {Array.from({ length: 6 }).map((_, index) => (
        <div key={index} className="flex gap-3">
          <div className="h-[30px] w-[30px] rounded-full bg-[#ecedef]" />
          <div className="min-w-0 flex-1 space-y-2 pt-1">
            <div className="h-2.5 w-3/4 rounded-full bg-[#ecedef]" />
            <div className="h-2 w-1/2 rounded-full bg-[#ecedef]" />
          </div>
        </div>
      ))}
    </div>
  );
}

export function ErrorState({ onRetry }: { onRetry: () => void }) {
  return (
    <div className="flex h-full min-h-[220px] flex-col items-center justify-center text-center">
      <div className="flex h-12 w-12 items-center justify-center rounded-full bg-[#fdecec] text-[#f0524f]">
        <Icon name="alert" className="h-[22px] w-[22px]" />
      </div>
      <div className="mt-3 text-sm font-bold text-ink">메일을 불러오지 못했습니다</div>
      <div className="mt-1 text-xs text-muted">서버에 연결할 수 없습니다 (IMAP timeout)</div>
      <button className="mt-5 flex items-center gap-2 rounded-lg border border-line bg-white px-4 py-2 text-[12.5px] font-medium text-text" onClick={onRetry}>
        <Icon name="refresh" className="h-3.5 w-3.5" />
        다시 시도
      </button>
    </div>
  );
}
