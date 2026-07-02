import { useEffect, useMemo, useRef, useState } from "react";
import { accounts as mockAccounts, messages as mockMessages } from "./data";
import type { Account, ComposeDraft, Message, MockMode } from "./types";
import { ComposePanel } from "./components/ComposePanel";
import { MessageList } from "./components/MessageList";
import { MobileInbox } from "./components/MobileInbox";
import { ReadingPane } from "./components/ReadingPane";
import { Sidebar } from "./components/Sidebar";
import { Toolbar } from "./components/Toolbar";
import { DevStateSwitcher, type QaState } from "./components/DevStateSwitcher";
import { LoginPage } from "./components/LoginPage";

const LIST_WIDTH_KEY = "joomail:list-width";
const QA_STATES: QaState[] = ["normal", "loading", "error", "empty", "empty-reading", "search", "search-empty", "multiselect", "compose"];
const SEARCH_QA_QUERY = "MIME";
const SEARCH_EMPTY_QA_QUERY = "qa-no-results-000";

export default function App() {
  const qaParam = new URLSearchParams(window.location.search).get("qa");
  if (import.meta.env.DEV && qaParam && qaParam !== "login") {
    return <AppShell />;
  }

  return <LoginGate />;
}

function LoginGate() {
  const [accounts, setAccounts] = useState<Account[] | null>(null);
  const [checking, setChecking] = useState(true);

  useEffect(() => {
    checkSession();
  }, []);

  async function checkSession() {
    setChecking(true);
    try {
      const body = await apiJSON<{ accounts: Account[] }>("/api/accounts");
      setAccounts(body.accounts);
    } catch {
      setAccounts(null);
    } finally {
      setChecking(false);
    }
  }

  if (checking) {
    return <div className="min-h-screen bg-panel" />;
  }

  if (!accounts) {
    return <LoginPage onLoginSuccess={() => checkSession()} />;
  }

  return <AppShell initialAccounts={accounts} onSessionExpired={() => setAccounts(null)} />;
}

type AppShellProps = {
  initialAccounts?: Account[];
  onSessionExpired?: () => void;
};

export function AppShell({ initialAccounts, onSessionExpired }: AppShellProps) {
  const useApi = Boolean(initialAccounts);
  const [accounts] = useState<Account[]>(initialAccounts ?? mockAccounts);
  const [accountId, setAccountId] = useState((initialAccounts ?? mockAccounts)[0].id);
  const [mailboxId, setMailboxId] = useState("inbox");
  const [selectedMessageId, setSelectedMessageId] = useState(useApi ? "" : "m1");
  const [checkedIds, setCheckedIds] = useState<Set<string>>(new Set());
  const [search, setSearch] = useState("");
  const [composeOpen, setComposeOpen] = useState(false);
  const [replyMessageId, setReplyMessageId] = useState<string | null>(null);
  const [mode, setMode] = useState<MockMode>("normal");
  const [apiMessages, setApiMessages] = useState<Message[]>([]);
  const [selectedMessageDetail, setSelectedMessageDetail] = useState<Message | undefined>();
  const [reloadToken, setReloadToken] = useState(0);
  const [listWidth, setListWidth] = useState(() => Number(localStorage.getItem(LIST_WIDTH_KEY)) || 388);
  const [forceEmptyList, setForceEmptyList] = useState(false);
  const shellRef = useRef<HTMLDivElement>(null);
  const searchInputRef = useRef<HTMLInputElement>(null);
  const initialQaAppliedRef = useRef(false);

  const selectedAccount = accounts.find((account) => account.id === accountId) ?? accounts[0];

  const visibleMessages = useMemo(() => {
    if (useApi) return apiMessages;
    if (forceEmptyList) return [];
    const accountMessages = mockMessages.filter((message) => message.accountId === accountId);
    if (search.trim()) {
      const query = search.trim().toLowerCase();
      return accountMessages.filter((message) =>
        [message.sender, message.subject, message.snippet, ...message.body].some((field) => field.toLowerCase().includes(query)),
      );
    }
    return accountMessages.filter((message) => message.mailboxId === mailboxId);
  }, [accountId, apiMessages, forceEmptyList, mailboxId, search, useApi]);

  const selectedMessage = useApi ? selectedMessageDetail ?? visibleMessages.find((message) => message.id === selectedMessageId) : visibleMessages.find((message) => message.id === selectedMessageId);
  const composeMessage = replyMessageId
    ? useApi
      ? selectedMessageDetail?.id === replyMessageId
        ? selectedMessageDetail
        : visibleMessages.find((message) => message.id === replyMessageId)
      : mockMessages.find((message) => message.id === replyMessageId)
    : undefined;
  const selectedMailbox = selectedAccount.mailboxes
    .flatMap((mailbox) => [mailbox, ...(mailbox.children ?? [])])
    .find((mailbox) => mailbox.id === mailboxId);

  useEffect(() => {
    localStorage.setItem(LIST_WIDTH_KEY, String(listWidth));
  }, [listWidth]);

  useEffect(() => {
    if (!useApi) return;
    let cancelled = false;
    setMode("loading");
    setSelectedMessageDetail(undefined);
    apiJSON<{ messages: ApiMessage[] }>(`/api/accounts/${encodeURIComponent(accountId)}/mailboxes/${encodeURIComponent(mailboxId)}/messages${search ? `?q=${encodeURIComponent(search)}` : ""}`)
      .then((body) => {
        if (cancelled) return;
        const nextMessages = body.messages.map(summaryToMessage);
        setApiMessages(nextMessages);
        setSelectedMessageId((current) => (current && nextMessages.some((message) => message.id === current) ? current : nextMessages[0]?.id ?? ""));
        setMode("normal");
      })
      .catch((error) => {
        if (cancelled) return;
        if (isUnauthorized(error)) {
          onSessionExpired?.();
          return;
        }
        setApiMessages([]);
        setSelectedMessageId("");
        setMode("error");
      });
    return () => {
      cancelled = true;
    };
  }, [accountId, mailboxId, onSessionExpired, reloadToken, search, useApi]);

  useEffect(() => {
    if (!useApi || !selectedMessageId) {
      if (useApi) setSelectedMessageDetail(undefined);
      return;
    }
    let cancelled = false;
    apiJSON<{ message: ApiMessage }>(`/api/messages/${encodeURIComponent(selectedMessageId)}`)
      .then((body) => {
        if (!cancelled) setSelectedMessageDetail(normalizeMessage(body.message));
      })
      .catch((error) => {
        if (cancelled) return;
        if (isUnauthorized(error)) {
          onSessionExpired?.();
          return;
        }
        setMode("error");
      });
    return () => {
      cancelled = true;
    };
  }, [onSessionExpired, selectedMessageId, useApi]);

  useEffect(() => {
    if (!visibleMessages.length) {
      if (selectedMessageId) setSelectedMessageId("");
      return;
    }
    if (selectedMessageId && !visibleMessages.some((message) => message.id === selectedMessageId)) {
      setSelectedMessageId(visibleMessages[0]?.id ?? "");
    }
  }, [selectedMessageId, visibleMessages]);

  useEffect(() => {
    if (initialQaAppliedRef.current) return;
    initialQaAppliedRef.current = true;

    const qaParam = new URLSearchParams(window.location.search).get("qa");
    if (isQaState(qaParam)) {
      applyQaState(qaParam);
    }
  }, []);

  useEffect(() => {
    function handleKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape" && composeOpen) {
        event.preventDefault();
        closeCompose();
        return;
      }

      if (isTypingTarget(event.target)) return;
      if (event.metaKey || event.ctrlKey || event.altKey) return;

      if (event.key === "/") {
        event.preventDefault();
        searchInputRef.current?.focus();
        return;
      }

      if (event.key === "c") {
        event.preventDefault();
        openCompose();
        return;
      }

      if (event.key === "Escape") {
        if (checkedIds.size > 0) {
          event.preventDefault();
          setCheckedIds(new Set());
          return;
        }
        if (search) {
          event.preventDefault();
          setSearch("");
        }
        return;
      }

      if (event.key === "ArrowDown" || event.key === "j" || event.key === "ArrowUp" || event.key === "k") {
        if (!visibleMessages.length) return;
        event.preventDefault();
        const movingDown = event.key === "ArrowDown" || event.key === "j";
        const currentIndex = visibleMessages.findIndex((message) => message.id === selectedMessageId);
        const fallbackIndex = movingDown ? -1 : visibleMessages.length;
        const nextIndex = movingDown
          ? Math.min(visibleMessages.length - 1, (currentIndex === -1 ? fallbackIndex : currentIndex) + 1)
          : Math.max(0, (currentIndex === -1 ? fallbackIndex : currentIndex) - 1);
        setSelectedMessageId(visibleMessages[nextIndex].id);
        return;
      }

      if (event.key === "Enter") {
        const focusedMessageId = getFocusedMessageId(event.target);
        const nextSelectedId = focusedMessageId || selectedMessageId || visibleMessages[0]?.id;
        if (!nextSelectedId) return;
        event.preventDefault();
        setSelectedMessageId(nextSelectedId);
        return;
      }

      if (event.key === "x") {
        const targetId = selectedMessageId || visibleMessages[0]?.id;
        if (!targetId) return;
        event.preventDefault();
        toggleChecked(targetId);
        return;
      }

      if (event.key === "r") {
        const targetId = selectedMessageId || visibleMessages[0]?.id;
        if (!targetId) return;
        event.preventDefault();
        setSelectedMessageId(targetId);
        openReply(targetId);
      }
    }

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [checkedIds.size, composeOpen, search, selectedMessageId, visibleMessages]);

  function applyQaState(nextState: QaState) {
    const accountMessages = mockMessages.filter((message) => message.accountId === accountId);
    const inboxMessages = accountMessages.filter((message) => message.mailboxId === "inbox");
    const firstInboxId = inboxMessages[0]?.id ?? "";
    const searchMessages = accountMessages.filter((message) =>
      [message.sender, message.subject, message.snippet, ...message.body].some((field) => field.toLowerCase().includes(SEARCH_QA_QUERY.toLowerCase())),
    );

    setForceEmptyList(nextState === "empty");
    setMode(nextState === "loading" || nextState === "error" ? nextState : "normal");
    setMailboxId("inbox");
    setSearch("");
    setCheckedIds(new Set());
    setReplyMessageId(null);
    setComposeOpen(nextState === "compose");
    setSelectedMessageId(firstInboxId);

    if (nextState === "empty" || nextState === "empty-reading" || nextState === "search-empty") {
      setSelectedMessageId("");
    }

    if (nextState === "search") {
      setSearch(SEARCH_QA_QUERY);
      setSelectedMessageId(searchMessages[0]?.id ?? "");
    }

    if (nextState === "search-empty") {
      setSearch(SEARCH_EMPTY_QA_QUERY);
    }

    if (nextState === "multiselect") {
      setCheckedIds(new Set(inboxMessages.slice(0, 3).map((message) => message.id)));
    }
  }

  function selectAccount(nextAccountId: string) {
    setAccountId(nextAccountId);
    setMailboxId("inbox");
    setForceEmptyList(false);
    setCheckedIds(new Set());
    if (useApi) {
      setSearch("");
      setSelectedMessageId("");
      return;
    }
    triggerLoading();
  }

  function selectMailbox(nextMailboxId: string) {
    setMailboxId(nextMailboxId);
    setSearch("");
    setForceEmptyList(false);
    setCheckedIds(new Set());
    if (useApi) {
      setSelectedMessageId("");
      return;
    }
    setMode(nextMailboxId === "spam" ? "error" : "loading");
    window.setTimeout(() => setMode(nextMailboxId === "spam" ? "error" : "normal"), 420);
  }

  function triggerLoading() {
    setForceEmptyList(false);
    setMode("loading");
    window.setTimeout(() => setMode("normal"), 420);
  }

  function retry() {
    if (useApi) {
      setReloadToken((value) => value + 1);
      return;
    }
    setForceEmptyList(false);
    triggerLoading();
    setMailboxId("inbox");
  }

  function handleSearch(nextSearch: string) {
    setForceEmptyList(false);
    setSearch(nextSearch);
  }

  function openCompose() {
    setReplyMessageId(null);
    setComposeOpen(true);
  }

  function openReply(messageId = selectedMessageId) {
    if (!messageId) return;
    setReplyMessageId(messageId);
    setComposeOpen(true);
  }

  function closeCompose() {
    setComposeOpen(false);
    setReplyMessageId(null);
  }

  async function sendDraft(draft: ComposeDraft) {
    try {
      await apiJSON("/api/send", {
        method: "POST",
        body: JSON.stringify(draft),
      });
    } catch (error) {
      if (isUnauthorized(error)) onSessionExpired?.();
      throw error;
    }
  }

  async function logout() {
    try {
      await apiJSON("/api/logout", { method: "POST" });
    } catch {
      // The local session should still be dropped if the server already expired it.
    } finally {
      onSessionExpired?.();
    }
  }

  function toggleChecked(id: string) {
    setCheckedIds((current) => {
      const next = new Set(current);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }

  function startResize(event: React.PointerEvent<HTMLDivElement>) {
    const startX = event.clientX;
    const startWidth = listWidth;
    event.currentTarget.setPointerCapture(event.pointerId);

    function move(moveEvent: PointerEvent) {
      const nextWidth = Math.min(460, Math.max(320, startWidth + moveEvent.clientX - startX));
      setListWidth(nextWidth);
    }

    function stop() {
      window.removeEventListener("pointermove", move);
      window.removeEventListener("pointerup", stop);
    }

    window.addEventListener("pointermove", move);
    window.addEventListener("pointerup", stop);
  }

  return (
    <div className="min-h-screen bg-white text-ink" style={{ "--list-width": `${listWidth}px` } as React.CSSProperties} ref={shellRef}>
      <MobileInbox
        account={selectedAccount}
        messages={visibleMessages}
        selectedMessage={selectedMessage}
        selectedId={selectedMessageId}
        checkedIds={checkedIds}
        search={search}
        mode={mode}
        onRetry={retry}
        onCompose={openCompose}
        onSelectMessage={setSelectedMessageId}
        onToggleChecked={toggleChecked}
      />
      <div className="hidden h-screen flex-col md:flex">
        <Toolbar search={search} searchInputRef={searchInputRef} onSearch={handleSearch} onCompose={openCompose} />
        <div className="min-h-0 flex flex-1">
          <Sidebar
            accounts={accounts}
            selectedAccount={selectedAccount}
            selectedMailboxId={mailboxId}
            onSelectAccount={selectAccount}
            onSelectMailbox={selectMailbox}
            onCompose={openCompose}
            onLogout={useApi ? logout : undefined}
          />
          <MessageList
            title={selectedMailbox?.label ?? "받은편지함"}
            unreadCount={visibleMessages.length === 0 && mode === "normal" && !search ? 0 : selectedAccount.unread}
            messages={visibleMessages}
            selectedId={selectedMessage?.id}
            checkedIds={checkedIds}
            search={search}
            mode={mode}
            onRetry={retry}
            onSelectMessage={setSelectedMessageId}
            onToggleChecked={toggleChecked}
            onClearChecked={() => setCheckedIds(new Set())}
          />
          <div className="hidden w-1 cursor-col-resize bg-white hover:bg-selected md:block" onPointerDown={startResize} aria-label="리스트 폭 조절" />
          <ReadingPane message={selectedMessage} mode={mode} onRetry={retry} onReply={() => openReply()} />
        </div>
      </div>
      {composeOpen ? <ComposePanel accounts={accounts} account={selectedAccount} message={composeMessage} onClose={closeCompose} onSend={useApi ? sendDraft : undefined} /> : null}
      {import.meta.env.DEV && !composeOpen ? <DevStateSwitcher states={QA_STATES} onApply={applyQaState} /> : null}
    </div>
  );
}

function isQaState(value: string | null): value is QaState {
  return QA_STATES.includes(value as QaState);
}

function getFocusedMessageId(target: EventTarget | null) {
  if (!(target instanceof Element)) return "";
  return target.closest<HTMLElement>("[data-message-id]")?.dataset.messageId ?? "";
}

function isTypingTarget(target: EventTarget | null) {
  if (!(target instanceof HTMLElement)) return false;
  const tagName = target.tagName.toLowerCase();
  return tagName === "input" || tagName === "textarea" || tagName === "select" || target.isContentEditable;
}

type ApiMessage = Omit<Message, "body"> & {
  textBody?: string[];
  body?: string[];
};

class ApiError extends Error {
  status: number;

  constructor(status: number) {
    super(`API request failed with ${status}`);
    this.status = status;
  }
}

async function apiJSON<T = unknown>(path: string, init: RequestInit = {}) {
  const headers = new Headers(init.headers);
  if (!headers.has("Content-Type")) headers.set("Content-Type", "application/json");
  const response = await fetch(path, {
    ...init,
    headers,
    credentials: "include",
  });
  if (!response.ok) throw new ApiError(response.status);
  return (await response.json()) as T;
}

function isUnauthorized(error: unknown) {
  return error instanceof ApiError && error.status === 401;
}

function summaryToMessage(message: ApiMessage): Message {
  return normalizeMessage({ ...message, body: message.body ?? [] });
}

function normalizeMessage(message: ApiMessage): Message {
  return {
    ...message,
    body: message.textBody ?? message.body ?? [],
  };
}
