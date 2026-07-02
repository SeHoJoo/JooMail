import { useState, type FormEvent } from "react";
import type { Account } from "../types";
import { Icon } from "./Icon";

type AddAccountModalProps = {
  onClose: () => void;
  onAdded: (account: Account) => void;
};

export function AddAccountModal({ onClose, onAdded }: AddAccountModalProps) {
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [authError, setAuthError] = useState(false);
  const [submitting, setSubmitting] = useState(false);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (submitting) return;

    setSubmitting(true);
    try {
      const response = await fetch("/api/login", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        credentials: "include",
        body: JSON.stringify({ email, password, remember: true }),
      });

      if (!response.ok) {
        setAuthError(true);
        return;
      }

      const body = (await response.json()) as { account: Account };
      setAuthError(false);
      onAdded(body.account);
    } catch {
      setAuthError(true);
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/20 px-4" role="presentation" onMouseDown={onClose}>
      <form className="w-full max-w-[380px] rounded-[12px] border border-line bg-white px-5 pb-5 pt-4 shadow-compose" onSubmit={submit} onMouseDown={(event) => event.stopPropagation()}>
        <div className="flex items-center gap-3">
          <div className="flex h-9 w-9 items-center justify-center rounded-[10px] bg-accent text-white">
            <Icon name="mail" className="h-[18px] w-[18px]" />
          </div>
          <div className="min-w-0 flex-1">
            <h2 className="text-[15px] font-bold text-ink">계정 추가</h2>
            <p className="mt-0.5 text-[12px] text-muted">메일 계정 정보를 입력하세요</p>
          </div>
          <button className="flex h-8 w-8 items-center justify-center rounded-md text-muted hover:bg-[#f6f7f8] hover:text-text" aria-label="닫기" onClick={onClose} type="button">
            <Icon name="close" className="h-4 w-4" />
          </button>
        </div>

        {authError ? (
          <div className="mt-4 flex items-center gap-2 rounded-[9px] border border-[#f4d3d0] bg-[#fdf1f0] px-3 py-2 text-[12px] font-medium text-[#b23a30]">
            <Icon name="alert" className="h-[14px] w-[14px] shrink-0" />
            <span>이메일 또는 비밀번호가 올바르지 않습니다.</span>
          </div>
        ) : null}

        <label className="mt-5 block text-[12px] font-bold text-[#5b6169]" htmlFor="add-account-email">
          이메일 주소
        </label>
        <div className="mt-[6px] flex h-[40px] items-center gap-2 rounded-[10px] border border-[#dfe2e6] px-3 text-[#b6bbc2]">
          <Icon name="mail" className="h-4 w-4 shrink-0" />
          <input
            id="add-account-email"
            className="min-w-0 flex-1 border-0 bg-transparent text-[13.5px] text-ink outline-none placeholder:text-muted focus:outline-none"
            type="email"
            autoComplete="email"
            placeholder="이메일 주소"
            value={email}
            onChange={(event) => setEmail(event.target.value)}
          />
        </div>

        <label className="mt-4 block text-[12px] font-bold text-[#5b6169]" htmlFor="add-account-password">
          비밀번호
        </label>
        <div className="mt-[6px] flex h-[40px] items-center gap-2 rounded-[10px] border border-[#dfe2e6] px-3">
          <Icon name="lock" className="h-4 w-4 shrink-0 text-[#b6bbc2]" />
          <input
            id="add-account-password"
            className="min-w-0 flex-1 border-0 bg-transparent text-[13.5px] text-ink outline-none placeholder:text-muted focus:outline-none"
            type={showPassword ? "text" : "password"}
            autoComplete="current-password"
            placeholder="메일 비밀번호"
            value={password}
            onChange={(event) => setPassword(event.target.value)}
          />
          <button className="text-muted" type="button" aria-label="비밀번호 표시 전환" onClick={() => setShowPassword((current) => !current)}>
            <Icon name="eye" className="h-4 w-4" />
          </button>
        </div>

        <div className="mt-5 flex justify-end gap-2">
          <button className="h-9 rounded-lg px-3 text-[13px] font-medium text-text hover:bg-[#f6f7f8]" onClick={onClose} type="button">
            취소
          </button>
          <button className="h-9 rounded-lg bg-accent px-4 text-[13px] font-bold text-white disabled:cursor-default disabled:opacity-70" type="submit" disabled={submitting}>
            추가
          </button>
        </div>
      </form>
    </div>
  );
}
