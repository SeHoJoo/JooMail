import { useState, type FormEvent } from "react";
import type { Account } from "../types";
import { Icon } from "./Icon";

type LoginPageProps = {
  onLoginSuccess?: (account: Account) => void;
};

export function LoginPage({ onLoginSuccess }: LoginPageProps) {
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [remember, setRemember] = useState(true);
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
        body: JSON.stringify({ email, password, remember }),
      });

      if (!response.ok) {
        setAuthError(true);
        return;
      }

      const body = (await response.json()) as { account: Account };
      setAuthError(false);
      onLoginSuccess?.(body.account);
    } catch {
      setAuthError(true);
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <main className="flex min-h-screen items-center justify-center bg-panel px-4 py-10 text-ink">
      <form className="w-full max-w-[404px] rounded-[14px] border border-[#e6e8ea] bg-white px-[34px] pb-7 pt-[34px]" onSubmit={submit}>
        <header className="flex flex-col items-center gap-3 text-center">
          <div className="flex h-[46px] w-[46px] items-center justify-center rounded-[13px] bg-accent text-white">
            <Icon name="mail" className="h-[22px] w-[22px]" />
          </div>
          <div>
            <h1 className="text-[18px] font-bold leading-6 text-ink">로그인</h1>
            <p className="mt-1 text-[12.5px] leading-5 text-muted">메일 서버에 연결할 계정 정보를 입력하세요</p>
          </div>
        </header>

        {authError ? (
          <div className="mt-5 flex items-center gap-2 rounded-[10px] border border-[#f4d3d0] bg-[#fdf1f0] px-3 py-[10px] text-[12.5px] font-medium text-[#b23a30]">
            <Icon name="alert" className="h-[15px] w-[15px] shrink-0" />
            <span>이메일 또는 비밀번호가 올바르지 않습니다.</span>
          </div>
        ) : null}

        <div className="mt-7">
          <label className="text-[12px] font-bold text-[#5b6169]" htmlFor="login-email">
            이메일 주소
          </label>
          <div className="mt-[6px] flex h-[42px] items-center gap-2 rounded-[10px] border border-[#dfe2e6] px-3 text-[#b6bbc2]">
            <Icon name="mail" className="h-4 w-4 shrink-0" />
            <input
              id="login-email"
              className="min-w-0 flex-1 border-0 bg-transparent text-[13.5px] text-ink outline-none placeholder:text-muted focus:outline-none"
              type="email"
              autoComplete="email"
              placeholder="you@good-night.co.kr"
              value={email}
              onChange={(event) => setEmail(event.target.value)}
            />
          </div>
        </div>

        <div className="mt-[18px]">
          <label className="text-[12px] font-bold text-[#5b6169]" htmlFor="login-password">
            비밀번호
          </label>
          <div
            className={[
              "mt-[6px] flex h-[42px] items-center gap-2 rounded-[10px] border px-3",
              authError ? "border-[#e0857c] shadow-[0_0_0_3px_rgba(224,133,124,0.14)]" : "border-accent shadow-[0_0_0_3px_rgba(45,100,216,0.10)]",
            ].join(" ")}
          >
            <Icon name="lock" className="h-4 w-4 shrink-0 text-[#b6bbc2]" />
            <input
              id="login-password"
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
          {authError ? <p className="mt-[7px] text-[11.5px] font-medium text-[#c0655c]">비밀번호를 다시 확인해 주세요.</p> : null}
        </div>

        {!authError ? (
          <div className="mt-[14px] flex items-center justify-between gap-3">
            <label className="flex items-center gap-2 text-[12.5px] font-medium text-[#3a3f45]">
              <button
                className={[
                  "flex h-[16px] w-[16px] shrink-0 items-center justify-center rounded-[4px] border",
                  remember ? "border-accent bg-accent text-white" : "border-[#dfe2e6] bg-white text-transparent",
                ].join(" ")}
                type="button"
                role="checkbox"
                aria-checked={remember}
                onClick={() => setRemember((current) => !current)}
              >
                <Icon name="check" className="h-[12px] w-[12px]" />
              </button>
              로그인 상태 유지
            </label>
            <a className="text-[12.5px] font-medium text-accent" href="#forgot-password" onClick={(event) => event.preventDefault()}>
              비밀번호 찾기
            </a>
          </div>
        ) : null}

        <div className="mt-7">
          <button
            className="flex h-11 w-full items-center justify-center rounded-[10px] bg-accent px-4 text-[14px] font-bold text-white disabled:cursor-default disabled:opacity-70"
            type="submit"
            disabled={submitting}
          >
            로그인
          </button>
          <a
            className="mt-[18px] flex items-center justify-center gap-1 border-t border-[#eef0f2] pt-[17px] text-[12.5px] font-medium text-[#6b727a]"
            href="#manual-server"
            onClick={(event) => event.preventDefault()}
          >
            <Icon name="chevron" className="h-4 w-4 -rotate-90" />
            서버 직접 설정 (IMAP / SMTP)
          </a>
        </div>
      </form>
    </main>
  );
}
