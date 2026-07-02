import type { RefObject } from "react";
import { Icon } from "./Icon";

type ToolbarProps = {
  search: string;
  searchInputRef?: RefObject<HTMLInputElement>;
  onSearch: (value: string) => void;
  onCompose: () => void;
  onRefresh: () => void;
  onSettings: () => void;
};

export function Toolbar({ search, searchInputRef, onSearch, onCompose, onRefresh, onSettings }: ToolbarProps) {
  return (
    <header className="hidden h-[52px] shrink-0 items-center border-b border-line bg-white px-4 md:flex">
      <div className="flex items-center gap-4">
        <Icon name="menu" className="h-[18px] w-[18px] text-[#3a3f45]" />
        <div className="text-[15px] font-bold text-accent">JooMail</div>
      </div>
      <label className="ml-8 flex h-[34px] w-[560px] items-center rounded-lg bg-[#f2f3f5] px-3 text-[13px] text-muted">
        <Icon name="search" className="h-4 w-4 shrink-0" />
        <input
          ref={searchInputRef}
          className="ml-3 min-w-0 flex-1 bg-transparent outline-none placeholder:text-muted"
          value={search}
          onChange={(event) => onSearch(event.target.value)}
          aria-label="메일 검색"
          placeholder="메일 검색 — 발신자, 제목, 본문"
        />
        <span className="text-[11px] text-[#b6bbc2]">/</span>
      </label>
      <button className="ml-auto hidden rounded-lg bg-accent px-4 py-2 text-[13px] font-medium text-white focus-visible:outline-offset-2 lg:block" onClick={onCompose}>
        새 메일 쓰기
      </button>
      <div className="ml-5 flex gap-6 text-[#3a3f45]">
        <button aria-label="새로고침" onClick={onRefresh} type="button">
          <Icon name="refresh" className="h-[17px] w-[17px]" />
        </button>
        <button aria-label="설정" onClick={onSettings} type="button">
          <Icon name="settings" className="h-[17px] w-[17px]" />
        </button>
      </div>
    </header>
  );
}
