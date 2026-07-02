import { Icon } from "./Icon";

export function EmptyState({ title, description }: { title: string; description: string }) {
  return (
    <div className="flex h-full min-h-[220px] flex-col items-center justify-center px-6 text-center">
      <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-[#f3f4f5] text-[#b6bbc2]">
        <Icon name="mail" className="h-5 w-5" />
      </div>
      <div className="mt-3 text-[13.5px] font-semibold text-[#5b6169]">{title}</div>
      <div className="mt-1 text-[12.5px] text-muted">{description}</div>
    </div>
  );
}

export function LoadingState() {
  return (
    <div className="space-y-0 p-3">
      {Array.from({ length: 6 }).map((_, index) => (
        <div key={index} className="flex gap-3 border-b border-[#f2f3f5] py-3 last:border-b-0">
          <div className="mt-1 h-2 w-2 rounded-full bg-[#ecedef]" />
          <div className="min-w-0 flex-1 space-y-2">
            <div className="h-2.5 w-2/5 rounded-full bg-[#ecedef]" />
            <div className="h-2.5 w-4/5 rounded-full bg-[#ecedef]" />
          </div>
        </div>
      ))}
    </div>
  );
}

export function ErrorState({ onRetry }: { onRetry: () => void }) {
  return (
    <div className="flex h-full min-h-[220px] flex-col items-center justify-center px-6 text-center">
      <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-[#fdecec] text-[#d1453b]">
        <Icon name="alert" className="h-5 w-5" />
      </div>
      <div className="mt-3 text-[13.5px] font-semibold text-ink">메일을 불러오지 못했습니다</div>
      <div className="mt-1 text-[12.5px] text-muted">서버에 연결할 수 없습니다 (IMAP timeout)</div>
      <button className="mt-5 flex items-center gap-2 rounded-lg border border-line bg-white px-4 py-2 text-[12.5px] font-medium text-text hover:bg-[#f6f7f8]" onClick={onRetry}>
        <Icon name="refresh" className="h-3.5 w-3.5" />
        다시 시도
      </button>
    </div>
  );
}
