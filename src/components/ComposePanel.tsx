import type { Account, Message } from "../types";
import { Icon } from "./Icon";

type ComposePanelProps = {
  account: Account;
  message?: Message;
  onClose: () => void;
};

export function ComposePanel({ account, message, onClose }: ComposePanelProps) {
  return (
    <section className="fixed inset-0 z-30 flex flex-col bg-white md:inset-auto md:bottom-[15px] md:right-5 md:h-[599px] md:w-[580px] md:rounded-[10px] md:shadow-compose">
      <div className="flex h-[38px] shrink-0 items-center rounded-t-[10px] bg-[#1e2126] px-4 text-white">
        <div className="text-[13px] font-medium">새 메일</div>
        <div className="ml-auto flex gap-5">
          <Icon name="minimize" className="hidden h-3.5 w-3.5 md:block" />
          <Icon name="expand" className="hidden h-3.5 w-3.5 md:block" />
          <button aria-label="작성 닫기" onClick={onClose}>
            <Icon name="close" className="h-3.5 w-3.5" />
          </button>
        </div>
      </div>
      <div className="flex h-[55px] shrink-0 items-center border-b border-line px-4">
        <label className="w-[90px] text-xs text-muted">보내는 사람</label>
        <button className="flex h-[30px] w-[220px] items-center gap-2 rounded-[7px] border border-line px-1.5 text-[12.5px] text-ink">
          <span className="flex h-[18px] w-[18px] items-center justify-center rounded-full bg-accent text-[8px] font-bold text-white">{account.initials}</span>
          <span className="truncate">{account.email}</span>
          <Icon name="chevron" className="ml-auto h-3 w-3 text-muted" />
        </button>
      </div>
      <div className="flex h-[50px] shrink-0 items-center border-b border-line px-4">
        <label className="w-[90px] text-xs text-muted">받는사람</label>
        {message ? (
          <span className="flex items-center gap-1.5 rounded-md bg-[#eceef1] px-2.5 py-1 text-[12.5px] text-text">
            {message.sender}
            <span className="text-xs text-muted">×</span>
          </span>
        ) : null}
        <span className="ml-3 text-[12.5px] text-muted">이름 또는 이메일 입력...</span>
        <button className="ml-auto text-xs text-accent">참조/숨은참조</button>
      </div>
      <div className="flex h-[43px] shrink-0 items-center border-b border-line px-4">
        <label className="w-[90px] text-xs text-muted">제목</label>
        <div className="truncate text-[13.5px] font-bold text-ink">{message ? `Re: ${message.subject}` : ""}</div>
      </div>
      <textarea
        className="min-h-0 flex-1 resize-none border-0 px-4 py-4 text-[13.5px] leading-[1.55] text-text outline-none"
        defaultValue={
          message
            ? `${message.sender}님, 자료 잘 받았습니다. 첨부해 주신 로드맵 확인했고 몇 가지 코멘트 남겼습니다.\n\nMIME charset 변환 작업을 앞당긴 부분 특히 좋아요 — EUC-KR 메일 테스트 케이스를 제가 몇 개 더 준비해 두겠습니다.`
            : ""
        }
      />
      <div className="flex h-[46px] shrink-0 items-center border-t border-line px-4">
        <button className="flex items-center gap-1.5 rounded-[7px] bg-accent py-2 pl-4 pr-3 text-[13px] font-medium text-white">
          보내기
          <Icon name="chevron" className="h-3 w-3" />
        </button>
        <div className="ml-6 flex gap-5 text-muted">
          <Icon name="paperclip" className="h-[15px] w-[15px]" />
          <Icon name="bold" className="h-[15px] w-[15px]" />
          <Icon name="italic" className="h-[15px] w-[15px]" />
        </div>
        <div className="ml-auto text-[11px] text-muted">임시저장됨 · 오전 9:47</div>
        <Icon name="trash" className="ml-5 h-[15px] w-[15px] text-muted" />
      </div>
    </section>
  );
}
