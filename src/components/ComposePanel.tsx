import { useEffect, useRef } from "react";
import type { Account, Message } from "../types";
import { Icon } from "./Icon";

type ComposePanelProps = {
  account: Account;
  message?: Message;
  onClose: () => void;
};

export function ComposePanel({ account, message, onClose }: ComposePanelProps) {
  const bodyRef = useRef<HTMLTextAreaElement>(null);

  useEffect(() => {
    bodyRef.current?.focus();
  }, []);

  return (
    <section className="fixed inset-0 z-40 flex flex-col bg-white md:inset-auto md:bottom-[15px] md:right-5 md:h-[599px] md:w-[580px] md:rounded-[10px] md:shadow-compose" data-compose-panel>
      <div className="flex h-[38px] shrink-0 items-center bg-[#1e2126] px-4 text-white md:rounded-t-[10px]">
        <div className="text-[13px] font-medium">{message ? "답장" : "새 메일"}</div>
        <div className="ml-auto flex items-center gap-1.5">
          <span className="hidden h-7 w-7 items-center justify-center md:flex" aria-hidden="true">
            <Icon name="minimize" className="h-3.5 w-3.5" />
          </span>
          <span className="hidden h-7 w-7 items-center justify-center md:flex" aria-hidden="true">
            <Icon name="expand" className="h-3.5 w-3.5" />
          </span>
          <button className="flex h-7 w-7 items-center justify-center rounded-md bg-white/10 hover:bg-white/15 md:bg-transparent md:hover:bg-white/10" aria-label="작성 닫기" onClick={onClose}>
            <span aria-hidden="true" className="text-[18px] leading-none">
              ×
            </span>
          </button>
        </div>
      </div>
      <div className="flex h-[55px] shrink-0 items-center border-b border-line px-4">
        <label className="w-[90px] text-xs text-muted">보내는 사람</label>
        <button className="flex h-[30px] min-w-0 flex-1 items-center gap-2 rounded-[7px] border border-line px-1.5 text-[12.5px] text-ink md:flex-none md:w-[220px]">
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
        <span className="ml-3 min-w-0 flex-1 truncate text-[12.5px] text-muted">이름 또는 이메일 입력...</span>
        <button className="ml-auto text-xs text-accent">참조/숨은참조</button>
      </div>
      <div className="flex h-[43px] shrink-0 items-center border-b border-line px-4">
        <label className="w-[90px] text-xs text-muted">제목</label>
        <div className="truncate text-[13.5px] font-bold text-ink">{message ? `Re: ${message.subject}` : ""}</div>
      </div>
      <textarea
        ref={bodyRef}
        className="min-h-0 flex-1 resize-none border-0 px-4 py-4 text-[13.5px] leading-[1.55] text-text outline-none focus-visible:outline-none"
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
        <div className="ml-auto hidden text-[11px] text-muted sm:block">임시저장됨 · 오전 9:47</div>
        <Icon name="trash" className="ml-auto h-[15px] w-[15px] text-muted sm:ml-5" />
      </div>
    </section>
  );
}
