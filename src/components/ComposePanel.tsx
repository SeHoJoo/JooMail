import { useEffect, useRef, useState } from "react";
import type { Account, ComposeDraft, ComposeMode, Message } from "../types";
import { Icon } from "./Icon";

type ComposePanelProps = {
  accounts: Account[];
  account: Account;
  mode: ComposeMode;
  message?: Message;
  onClose: () => void;
  onSend?: (draft: ComposeDraft) => Promise<void>;
};

type MockAttachment = {
  name: string;
  size: string;
};

export function ComposePanel({ accounts, account, mode, message, onClose, onSend }: ComposePanelProps) {
  const bodyRef = useRef<HTMLTextAreaElement>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [fromAccountId, setFromAccountId] = useState(account.id);
  const [fromMenuOpen, setFromMenuOpen] = useState(false);
  const [showCcBcc, setShowCcBcc] = useState(false);
  const [recipientText, setRecipientText] = useState(() => composeInitialState(mode, message, account.email).to);
  const [ccText, setCcText] = useState(() => composeInitialState(mode, message, account.email).cc);
  const [bccText, setBccText] = useState("");
  const [subject, setSubject] = useState(() => composeInitialState(mode, message, account.email).subject);
  const [body, setBody] = useState(() => composeInitialState(mode, message, account.email).body);
  const [attachments, setAttachments] = useState<MockAttachment[]>([]);
  const [attachmentFiles, setAttachmentFiles] = useState<File[]>([]);
  const [sendError, setSendError] = useState("");
  const [draftNotice, setDraftNotice] = useState("");
  const [sending, setSending] = useState(false);
  const [minimized, setMinimized] = useState(false);
  const [expanded, setExpanded] = useState(false);
  const fromAccount = accounts.find((item) => item.id === fromAccountId) ?? account;

  useEffect(() => {
    bodyRef.current?.focus();
  }, []);

  useEffect(() => {
    setFromAccountId(account.id);
  }, [account.id]);

  useEffect(() => {
    const initial = composeInitialState(mode, message, account.email);
    setRecipientText(initial.to);
    setCcText(initial.cc);
    setBccText("");
    setShowCcBcc(Boolean(initial.cc));
    setSubject(initial.subject);
    setBody(initial.body);
    setSendError("");
    setDraftNotice("");
    setAttachments([]);
    setAttachmentFiles([]);
    setMinimized(false);
  }, [account.email, message, mode]);

  function handleFiles(files: FileList | null) {
    if (!files?.length) return;
    const nextFiles = Array.from(files);
    setAttachmentFiles((current) => [...current, ...nextFiles]);
    setAttachments((current) => [...current, ...nextFiles.map((file) => ({ name: file.name, size: formatFileSize(file.size) }))]);
    if (fileInputRef.current) fileInputRef.current.value = "";
  }

  async function submitSend() {
    if (!onSend || sending) return;
    setSending(true);
    setSendError("");
    setDraftNotice("");
    try {
      await onSend({
        fromAccountId,
        fromName: fromAccount.label,
        to: parseRecipients(recipientText || message?.senderEmail || ""),
        cc: parseRecipients(ccText),
        bcc: parseRecipients(bccText),
        subject,
        textBody: body,
        attachments: attachmentFiles,
      });
      onClose();
    } catch {
      setSendError("메일을 보내지 못했습니다. 입력값과 서버 상태를 확인해 주세요.");
    } finally {
      setSending(false);
    }
  }

  function showDraftDeferredNotice() {
    setSendError("");
    setDraftNotice("임시저장은 서버 Drafts 저장 지원이 추가된 뒤 사용할 수 있습니다.");
  }

  if (minimized) {
    return (
      <section className="fixed bottom-[15px] right-5 z-40 hidden h-[38px] w-[320px] items-center rounded-t-[10px] bg-[#1e2126] px-4 text-white shadow-compose md:flex" data-compose-panel>
        <button className="min-w-0 flex-1 truncate text-left text-[13px] font-medium" onClick={() => setMinimized(false)} type="button">
          {subject.trim() || composeTitle(mode)}
        </button>
        <button className="ml-2 flex h-7 w-7 items-center justify-center rounded-md hover:bg-white/10" aria-label="작성 다시 열기" onClick={() => setMinimized(false)} type="button">
          <Icon name="expand" className="h-3.5 w-3.5" />
        </button>
        <button className="flex h-7 w-7 items-center justify-center rounded-md hover:bg-white/10" aria-label="작성 닫기" onClick={onClose} type="button">
          <Icon name="close" className="h-3.5 w-3.5" />
        </button>
      </section>
    );
  }

  return (
    <section className={expanded ? "fixed inset-0 z-40 flex flex-col bg-white md:inset-[48px] md:rounded-[10px] md:shadow-compose" : "fixed inset-0 z-40 flex flex-col bg-white md:inset-auto md:bottom-[15px] md:right-5 md:h-[599px] md:w-[580px] md:rounded-[10px] md:shadow-compose"} data-compose-panel>
      <div className="flex h-[38px] shrink-0 items-center bg-[#1e2126] px-4 text-white md:rounded-t-[10px]">
        <div className="text-[13px] font-medium">{composeTitle(mode)}</div>
        <div className="ml-auto flex items-center gap-1.5">
          <button className="hidden h-7 w-7 items-center justify-center rounded-md hover:bg-white/10 md:flex" aria-label="작성 최소화" onClick={() => setMinimized(true)} type="button">
            <Icon name="minimize" className="h-3.5 w-3.5" />
          </button>
          <button className="hidden h-7 w-7 items-center justify-center rounded-md hover:bg-white/10 md:flex" aria-label={expanded ? "작성 축소" : "작성 확대"} onClick={() => setExpanded((value) => !value)} type="button">
            <Icon name={expanded ? "minimize" : "expand"} className="h-3.5 w-3.5" />
          </button>
          <button className="flex h-7 w-7 items-center justify-center rounded-md bg-white/10 hover:bg-white/15 md:bg-transparent md:hover:bg-white/10" aria-label="작성 닫기" onClick={onClose} type="button">
            <Icon name="close" className="h-3.5 w-3.5" />
          </button>
        </div>
      </div>
      <div className="relative flex h-[55px] shrink-0 items-center border-b border-line px-4">
        <label className="w-[90px] shrink-0 text-xs text-muted">보내는 사람</label>
        <button
          className="flex h-[30px] min-w-0 flex-1 items-center gap-2 rounded-[7px] border border-line px-1.5 text-[12.5px] text-ink hover:bg-[#f7f8f9] md:flex-none md:w-[220px]"
          aria-expanded={fromMenuOpen}
          aria-haspopup="listbox"
          onClick={() => setFromMenuOpen((open) => !open)}
          type="button"
        >
          <span className="flex h-[18px] w-[18px] items-center justify-center rounded-full bg-accent text-[8px] font-bold text-white">{fromAccount.initials}</span>
          <span className="truncate">{fromAccount.email}</span>
          <Icon name="chevron" className="ml-auto h-3 w-3 text-muted" />
        </button>
        {fromMenuOpen ? (
          <div className="absolute left-[106px] top-[45px] z-10 w-[260px] rounded-lg border border-line bg-white py-1 shadow-compose" role="listbox">
            {accounts.map((item) => (
              <button
                key={item.id}
                className="flex w-full items-center gap-2 px-2.5 py-2 text-left text-[12.5px] text-ink hover:bg-[#f7f8f9]"
                onClick={() => {
                  setFromAccountId(item.id);
                  setFromMenuOpen(false);
                }}
                role="option"
                aria-selected={item.id === fromAccount.id}
                type="button"
              >
                <span className="flex h-[20px] w-[20px] shrink-0 items-center justify-center rounded-full bg-accent text-[8px] font-bold text-white">{item.initials}</span>
                <span className="min-w-0 flex-1">
                  <span className="block truncate font-medium">{item.email}</span>
                  <span className="block truncate text-[11px] text-muted">{item.label}</span>
                </span>
                {item.id === fromAccount.id ? <span className="text-accent">✓</span> : null}
              </button>
            ))}
          </div>
        ) : null}
      </div>
      <div className="flex h-[50px] shrink-0 items-center border-b border-line px-4">
        <label className="w-[90px] shrink-0 text-xs text-muted" htmlFor="compose-to">
          받는사람
        </label>
        <input
          id="compose-to"
          className="min-w-0 flex-1 border-0 bg-transparent text-[12.5px] text-ink outline-none placeholder:text-muted focus-visible:outline-none"
          value={recipientText}
          onChange={(event) => setRecipientText(event.target.value)}
          aria-label="받는사람"
          placeholder="이름 또는 이메일 입력..."
        />
        <button className="ml-2 shrink-0 text-xs text-accent" onClick={() => setShowCcBcc((show) => !show)} type="button">
          참조/숨은참조
        </button>
      </div>
      {showCcBcc ? (
        <>
          <RecipientRow id="compose-cc" label="참조" value={ccText} onChange={setCcText} />
          <RecipientRow id="compose-bcc" label="숨은참조" value={bccText} onChange={setBccText} />
        </>
      ) : null}
      <div className="flex h-[43px] shrink-0 items-center border-b border-line px-4">
        <label className="w-[90px] shrink-0 text-xs text-muted" htmlFor="compose-subject">
          제목
        </label>
        <input
          id="compose-subject"
          className="min-w-0 flex-1 border-0 bg-transparent text-[13.5px] font-bold text-ink outline-none placeholder:text-muted focus-visible:outline-none"
          value={subject}
          onChange={(event) => setSubject(event.target.value)}
          aria-label="제목"
        />
      </div>
      <textarea
        ref={bodyRef}
        className="min-h-0 flex-1 resize-none border-0 px-4 py-4 text-[13.5px] leading-[1.55] text-text outline-none focus-visible:outline-none"
        value={body}
        onChange={(event) => setBody(event.target.value)}
      />
      {sendError ? (
        <div className="shrink-0 border-t border-[#f4d3d0] bg-[#fdf1f0] px-4 py-2 text-[12.5px] font-medium text-[#b23a30]">{sendError}</div>
      ) : null}
      {!sendError && draftNotice ? (
        <div className="shrink-0 border-t border-line bg-[#f7f8f9] px-4 py-2 text-[12.5px] text-muted">{draftNotice}</div>
      ) : null}
      {attachments.length ? (
        <div className="shrink-0 border-t border-line px-4 py-2">
          <div className="mb-1 text-[11px] text-muted">첨부파일 {attachments.length}개</div>
          <div className="flex flex-wrap gap-2">
            {attachments.map((attachment) => (
              <div key={`${attachment.name}-${attachment.size}`} className="flex max-w-full items-center gap-2 rounded-md border border-line bg-[#f7f8f9] px-2 py-1.5 text-[12px] text-text">
                <Icon name="paperclip" className="h-3.5 w-3.5 shrink-0 text-muted" />
                <span className="max-w-[210px] truncate">{attachment.name}</span>
                <span className="shrink-0 text-[11px] text-muted">{attachment.size}</span>
              </div>
            ))}
          </div>
        </div>
      ) : null}
      <div className="flex h-[46px] shrink-0 items-center border-t border-line px-4">
        <button className="flex items-center gap-1.5 rounded-[7px] bg-accent py-2 pl-4 pr-3 text-[13px] font-medium text-white disabled:opacity-70" onClick={submitSend} disabled={sending} type="button">
          {sending ? "전송 중" : "보내기"}
          <Icon name="chevron" className="h-3 w-3" />
        </button>
        <div className="ml-6 flex gap-5 text-muted">
          <button className="flex h-[18px] w-[18px] items-center justify-center hover:text-text" aria-label="파일 첨부" onClick={() => fileInputRef.current?.click()} type="button">
            <Icon name="paperclip" className="h-[15px] w-[15px]" />
          </button>
        </div>
        <button className="ml-5 hidden text-[12px] font-medium text-muted hover:text-text sm:block" onClick={showDraftDeferredNotice} type="button">
          임시저장
        </button>
        <div className="ml-4 hidden text-[11px] text-muted sm:block">{draftNotice ? "임시저장 대기" : "작성 중"}</div>
        <input ref={fileInputRef} className="hidden" type="file" multiple onChange={(event) => handleFiles(event.target.files)} />
        <button className="ml-auto flex h-[18px] w-[18px] items-center justify-center text-muted hover:text-text sm:ml-5" aria-label="작성 삭제" onClick={onClose} type="button">
          <Icon name="trash" className="h-[15px] w-[15px]" />
        </button>
      </div>
    </section>
  );
}

function RecipientRow({ id, label, value, onChange }: { id: string; label: string; value: string; onChange: (value: string) => void }) {
  return (
    <div className="flex h-[42px] shrink-0 items-center border-b border-line px-4">
      <label className="w-[90px] shrink-0 text-xs text-muted" htmlFor={id}>
        {label}
      </label>
      <input
        id={id}
        className="min-w-0 flex-1 border-0 bg-transparent text-[12.5px] text-ink outline-none placeholder:text-muted focus-visible:outline-none"
        value={value}
        onChange={(event) => onChange(event.target.value)}
        aria-label={label}
        placeholder="이름 또는 이메일 입력..."
      />
    </div>
  );
}

function formatFileSize(bytes: number) {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${Math.round(bytes / 102.4) / 10} KB`;
  return `${Math.round(bytes / 1024 / 102.4) / 10} MB`;
}

function parseRecipients(value: string) {
  return value
    .split(/[,\n;]/)
    .map((item) => item.trim())
    .filter(Boolean);
}

function composeTitle(mode: ComposeMode) {
  if (mode === "reply") return "답장";
  if (mode === "replyAll") return "전체답장";
  if (mode === "forward") return "전달";
  return "새 메일";
}

function composeInitialState(mode: ComposeMode, message: Message | undefined, accountEmail: string) {
  if (!message || mode === "compose") {
    return { to: "", cc: "", subject: "", body: "" };
  }

  if (mode === "forward") {
    return {
      to: "",
      cc: "",
      subject: addSubjectPrefix("Fwd:", message.subject),
      body: forwardedBody(message),
    };
  }

  const self = accountEmail.toLowerCase();
  const replyTo = normalizeAddress(message.senderEmail);
  const to = mode === "replyAll"
    ? uniqueAddresses([replyTo, ...(message.headers?.to ?? [])], self).join(", ")
    : replyTo;
  const cc = mode === "replyAll" ? uniqueAddresses(message.headers?.cc ?? [], self).join(", ") : "";

  return {
    to,
    cc,
    subject: addSubjectPrefix("Re:", message.subject),
    body: replyBody(message),
  };
}

function addSubjectPrefix(prefix: "Re:" | "Fwd:", subject: string) {
  const normalized = subject.trim();
  if (normalized.toLowerCase().startsWith(prefix.toLowerCase())) return normalized;
  return `${prefix} ${normalized}`;
}

function replyBody(message: Message) {
  return `\n\nOn ${message.fullDate}, ${message.sender} wrote:\n${quoteBody(message)}`;
}

function forwardedBody(message: Message) {
  return [
    "",
    "",
    "---------- Forwarded message ---------",
    `From: ${message.sender} <${message.senderEmail}>`,
    `Date: ${message.fullDate}`,
    `Subject: ${message.subject}`,
    "",
    message.body.join("\n\n"),
  ].join("\n");
}

function quoteBody(message: Message) {
  const source = message.body.join("\n\n") || message.snippet;
  return source
    .split("\n")
    .map((line) => `> ${line}`)
    .join("\n");
}

function uniqueAddresses(values: string[], self: string) {
  const seen = new Set<string>();
  const result: string[] = [];
  values.flatMap(splitAddressList).forEach((value) => {
    const address = normalizeAddress(value);
    const key = address.toLowerCase();
    if (!address || key === self || seen.has(key)) return;
    seen.add(key);
    result.push(address);
  });
  return result;
}

function splitAddressList(value: string) {
  return value
    .split(",")
    .map((item) => item.trim())
    .filter(Boolean);
}

function normalizeAddress(value: string) {
  const match = value.match(/<([^>]+)>/);
  return (match?.[1] ?? value).trim();
}
