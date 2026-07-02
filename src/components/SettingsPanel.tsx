import type { Account } from "../types";
import packageJSON from "../../package.json";
import { Icon } from "./Icon";

type SettingsPanelProps = {
  account: Account;
  displayName: string;
  onDisplayNameChange: (value: string) => void;
  remoteImagesEnabled: boolean;
  onRemoteImagesChange: (enabled: boolean) => void;
  onLogout?: () => void;
  onClose: () => void;
};

export function SettingsPanel({ account, displayName, onDisplayNameChange, remoteImagesEnabled, onRemoteImagesChange, onLogout, onClose }: SettingsPanelProps) {
  return (
    <div className="fixed inset-0 z-30 hidden bg-black/10 md:block" role="presentation" onMouseDown={onClose}>
      <section
        className="absolute right-4 top-[60px] w-[360px] rounded-lg border border-line bg-white shadow-compose"
        aria-label="설정"
        onMouseDown={(event) => event.stopPropagation()}
      >
        <header className="flex h-12 items-center border-b border-line px-4">
          <h2 className="text-[14px] font-bold text-ink">설정</h2>
          <button className="ml-auto flex h-8 w-8 items-center justify-center rounded-md text-muted hover:bg-[#f7f8f9] hover:text-text" aria-label="설정 닫기" onClick={onClose} type="button">
            <Icon name="close" className="h-4 w-4" />
          </button>
        </header>
        <div className="px-4 py-3">
          <div className="border-b border-line pb-3">
            <div className="text-[11px] font-bold uppercase text-[#9aa0a8]">계정</div>
            <div className="mt-2 truncate text-[13px] font-medium text-ink">{account.email}</div>
            <div className="mt-1 text-[12px] text-muted">{account.label}</div>
          </div>

          <div className="border-b border-line py-3">
            <label className="block text-[12px] font-bold text-[#5b6169]" htmlFor="settings-display-name">
              이름
            </label>
            <input
              id="settings-display-name"
              className="mt-2 h-9 w-full rounded-md border border-[#dfe2e6] px-3 text-[13px] text-ink outline-none placeholder:text-muted focus:border-accent"
              value={displayName}
              onChange={(event) => onDisplayNameChange(event.target.value)}
              placeholder="발신자 이름"
            />
            <div className="mt-1 text-[11.5px] leading-4 text-muted">메일을 보낼 때 발신자 이름으로 사용합니다.</div>
          </div>

          <div className="border-b border-line py-3">
            <label className="flex items-center gap-3">
              <span className="min-w-0 flex-1">
                <span className="block text-[13px] font-medium text-ink">원격 이미지 자동 표시</span>
                <span className="mt-0.5 block text-[11.5px] leading-4 text-muted">메일을 열 때 차단된 원격 이미지를 바로 표시합니다.</span>
              </span>
              <button
                className={remoteImagesEnabled ? "h-6 w-11 rounded-full bg-accent p-0.5 text-left" : "h-6 w-11 rounded-full bg-[#d7dbe0] p-0.5 text-left"}
                role="switch"
                aria-checked={remoteImagesEnabled}
                onClick={() => onRemoteImagesChange(!remoteImagesEnabled)}
                type="button"
              >
                <span className={remoteImagesEnabled ? "block h-5 w-5 translate-x-5 rounded-full bg-white transition" : "block h-5 w-5 rounded-full bg-white transition"} />
              </button>
            </label>
          </div>

          <div className="flex items-center justify-between border-b border-line py-3 text-[12.5px]">
            <span className="text-muted">버전</span>
            <span className="font-medium text-text">v{packageJSON.version}</span>
          </div>

          {onLogout ? (
            <div className="pt-3">
              <button className="h-9 w-full rounded-md border border-line text-[13px] font-medium text-text hover:bg-[#f7f8f9]" onClick={onLogout} type="button">
                로그아웃
              </button>
            </div>
          ) : null}
        </div>
      </section>
    </div>
  );
}
