import type { Message, MockMode } from "../types";
import { EmptyState, ErrorState, LoadingState } from "./StateViews";
import { Icon } from "./Icon";

type ReadingPaneProps = {
  message?: Message;
  mode: MockMode;
  onRetry: () => void;
  onReply: () => void;
};

export function ReadingPane({ message, mode, onRetry, onReply }: ReadingPaneProps) {
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
      <div className="px-[27px] pb-10 pt-6">
        <div className="flex items-start gap-4">
          <h1 className="min-w-0 flex-1 text-[18px] font-bold text-ink">{message.subject}</h1>
          <Icon name="star" className="h-[18px] w-[18px] shrink-0 fill-[#f5b514] text-[#f5b514]" />
        </div>
        <div className="mt-6 flex items-start gap-3">
          <div className="flex h-[38px] w-[38px] shrink-0 items-center justify-center rounded-full bg-selected text-sm font-bold text-accent">{message.initials}</div>
          <div className="min-w-0 flex-1">
            <div className="flex min-w-0 items-baseline gap-1">
              <span className="text-sm font-bold text-ink">{message.sender}</span>
              <span className="truncate text-[13px] text-muted">&lt;{message.senderEmail}&gt;</span>
            </div>
            <div className="mt-1 text-xs text-muted">받는사람: 나 (jooseho@gmail.com) · 받는사람 보기 ▾</div>
          </div>
          <div className="hidden w-[200px] text-right text-xs text-muted lg:block">
            <div>{message.fullDate}</div>
            <div className="mt-1 text-[11px]">{message.time === "오전 9:14" ? "3분 전" : message.time}</div>
          </div>
        </div>
        <div className="mt-6 flex items-center gap-2">
          <ActionButton icon="reply" label="답장" onClick={onReply} />
          <ActionButton icon="replyAll" label="전체답장" onClick={onReply} />
          <ActionButton icon="forward" label="전달" onClick={onReply} />
          <div className="ml-auto flex gap-2 text-[#3a3f45]">
            <Icon name="archive" className="h-[15px] w-[15px]" />
            <Icon name="trash" className="h-[15px] w-[15px]" />
            <Icon name="folder" className="h-[15px] w-[15px]" />
            <Icon name="more" className="h-[15px] w-[15px]" />
          </div>
        </div>
      </div>
      <div className="border-t border-line" />
      <div className="px-[27px] py-[18px]">
        {message.remoteImagesBlocked ? (
          <div className="mb-5 flex h-9 items-center rounded-lg bg-[#f7f8f9] px-3 text-[12.5px] text-text">
            <Icon name="image" className="mr-3 h-4 w-4 text-muted" />
            이 메일의 원격 이미지가 차단되었습니다.
            <button className="ml-auto font-medium text-accent">이미지 표시</button>
          </div>
        ) : null}
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
        {message.attachments?.length ? (
          <div className="mt-8">
            <div className="mb-3 text-xs text-muted">첨부파일 {message.attachments.length}개 · 3.1 MB</div>
            <div className="flex flex-wrap gap-3">
              {message.attachments.map((attachment) => (
                <div key={attachment.name} className="flex h-[52px] w-[220px] items-center rounded-lg border border-line bg-white px-2">
                  <div className={attachment.type === "pdf" ? "flex h-[34px] w-[34px] items-center justify-center rounded-md bg-[#fdecec] text-[#e9564f]" : "flex h-[34px] w-[34px] items-center justify-center rounded-md bg-[#eaf0f6] text-accent"}>
                    <Icon name={attachment.type === "image" ? "image" : "mail"} className="h-4 w-4" />
                  </div>
                  <div className="ml-2 min-w-0 flex-1">
                    <div className="truncate text-[12.5px] font-medium text-ink">{attachment.name}</div>
                    <div className="text-[11px] text-muted">{attachment.size}</div>
                  </div>
                  <Icon name="download" className="h-3.5 w-3.5 text-muted" />
                </div>
              ))}
            </div>
          </div>
        ) : null}
        <button className="mt-5 flex items-center gap-2 rounded-md bg-[#f7f8f9] px-3 py-1.5 text-xs text-muted">
          <Icon name="more" className="h-3.5 w-3.5" />
          인용된 이전 대화 보기
        </button>
      </div>
    </section>
  );
}

function ActionButton({ icon, label, onClick }: { icon: "reply" | "replyAll" | "forward"; label: string; onClick: () => void }) {
  return (
    <button className="flex items-center gap-1.5 rounded-[7px] border border-line bg-white px-3 py-[7px] text-[12.5px] font-medium text-text" onClick={onClick}>
      <Icon name={icon} className="h-3.5 w-3.5" />
      {label}
    </button>
  );
}
