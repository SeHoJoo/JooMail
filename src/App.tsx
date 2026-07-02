import { useEffect, useMemo, useRef, useState } from "react";
import { accounts as mockAccounts, messages as mockMessages } from "./data";
import type { Account, ComposeDraft, Mailbox, Message, MockMode } from "./types";
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
  const flatMailboxes = useMemo(() => flattenMailboxes(selectedAccount.mailboxes), [selectedAccount.mailboxes]);

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
        if (!cancelled) {
          const message = normalizeMessage(body.message);
          setSelectedMessageDetail(message);
          setApiMessages((current) => current.map((item) => (item.id === message.id ? { ...item, unread: false } : item)));
        }
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
      setReloadToken((value) => value + 1);
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

  function toggleAllChecked() {
    setCheckedIds((current) => {
      const visibleIds = visibleMessages.map((message) => message.id);
      if (visibleIds.length === 0) return current;
      const allChecked = visibleIds.every((id) => current.has(id));
      if (allChecked) {
        const next = new Set(current);
        visibleIds.forEach((id) => next.delete(id));
        return next;
      }
      return new Set([...current, ...visibleIds]);
    });
  }

  async function toggleFlagged(message: Message) {
    const nextFlagged = !message.flagged;
    updateMessageFlagged(message.id, nextFlagged);
    if (!useApi) return;
    try {
      await apiJSON(`/api/messages/${encodeURIComponent(message.id)}/flagged`, {
        method: "PATCH",
        body: JSON.stringify({ flagged: nextFlagged }),
      });
    } catch (error) {
      updateMessageFlagged(message.id, Boolean(message.flagged));
      if (isUnauthorized(error)) onSessionExpired?.();
    }
  }

  function updateMessageFlagged(id: string, flagged: boolean) {
    setApiMessages((current) => current.map((item) => (item.id === id ? { ...item, flagged } : item)));
    setSelectedMessageDetail((current) => (current?.id === id ? { ...current, flagged } : current));
  }

  async function markUnread(message: Message) {
    updateMessageUnread(message.id, true);
    if (!useApi) return;
    try {
      await apiJSON(`/api/messages/${encodeURIComponent(message.id)}/seen`, {
        method: "PATCH",
        body: JSON.stringify({ seen: false }),
      });
    } catch (error) {
      updateMessageUnread(message.id, false);
      if (isUnauthorized(error)) onSessionExpired?.();
      throw error;
    }
  }

  function updateMessageUnread(id: string, unread: boolean) {
    setApiMessages((current) => current.map((item) => (item.id === id ? { ...item, unread } : item)));
    setSelectedMessageDetail((current) => (current?.id === id ? { ...current, unread } : current));
  }

  async function moveMessage(message: Message, targetMailboxId: string) {
    if (!targetMailboxId) throw new Error("missing target mailbox");
    if (!useApi) {
      setSelectedMessageId("");
      return;
    }
    const previousMessages = apiMessages;
    const previousDetail = selectedMessageDetail;
    setApiMessages((current) => current.filter((item) => item.id !== message.id));
    setSelectedMessageDetail(undefined);
    setSelectedMessageId("");
    try {
      await apiJSON(`/api/messages/${encodeURIComponent(message.id)}/move`, {
        method: "POST",
        body: JSON.stringify({ mailboxId: targetMailboxId }),
      });
    } catch (error) {
      setApiMessages(previousMessages);
      setSelectedMessageDetail(previousDetail);
      setSelectedMessageId(message.id);
      if (isUnauthorized(error)) onSessionExpired?.();
      throw error;
    }
  }

  function moveToKind(message: Message, kind: Mailbox["kind"]) {
    const mailbox = flatMailboxes.find((item) => item.kind === kind);
    if (!mailbox) throw new Error(`missing ${kind} mailbox`);
    return moveMessage(message, mailbox.id);
  }

  function bulkMoveToKind(kind: Mailbox["kind"]) {
    const mailbox = flatMailboxes.find((item) => item.kind === kind);
    if (!mailbox) throw new Error(`missing ${kind} mailbox`);
    return bulkMoveMessages(mailbox.id);
  }

  async function bulkMoveMessages(targetMailboxId: string) {
    const selectedMessages = visibleMessages.filter((message) => checkedIds.has(message.id));
    if (!selectedMessages.length) return;
    if (!useApi) {
      setCheckedIds(new Set());
      setSelectedMessageId("");
      return;
    }
    const selectedIds = new Set(selectedMessages.map((message) => message.id));
    const previousMessages = apiMessages;
    const previousDetail = selectedMessageDetail;
    const previousSelectedId = selectedMessageId;
    setApiMessages((current) => current.filter((message) => !selectedIds.has(message.id)));
    setSelectedMessageDetail((current) => (current && selectedIds.has(current.id) ? undefined : current));
    setSelectedMessageId((current) => (selectedIds.has(current) ? "" : current));
    try {
      await Promise.all(
        selectedMessages.map((message) =>
          apiJSON(`/api/messages/${encodeURIComponent(message.id)}/move`, {
            method: "POST",
            body: JSON.stringify({ mailboxId: targetMailboxId }),
          }),
        ),
      );
    } catch (error) {
      setApiMessages(previousMessages);
      setSelectedMessageDetail(previousDetail);
      setSelectedMessageId(previousSelectedId);
      if (isUnauthorized(error)) onSessionExpired?.();
      throw error;
    }
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
        onToggleFlagged={toggleFlagged}
        onClearChecked={() => setCheckedIds(new Set())}
        onBulkArchive={() => bulkMoveToKind("archive")}
        onBulkTrash={() => bulkMoveToKind("trash")}
      />
      <div className="hidden h-screen flex-col md:flex">
        <Toolbar search={search} searchInputRef={searchInputRef} onSearch={handleSearch} onCompose={openCompose} onRefresh={retry} onSettings={() => window.alert("설정은 아직 준비 중입니다.")} />
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
            mailboxes={flatMailboxes}
            selectedId={selectedMessage?.id}
            checkedIds={checkedIds}
            search={search}
            mode={mode}
            onRetry={retry}
            onSelectMessage={setSelectedMessageId}
            onToggleAllChecked={toggleAllChecked}
            onToggleChecked={toggleChecked}
            onToggleFlagged={toggleFlagged}
            onBulkArchive={() => bulkMoveToKind("archive")}
            onBulkTrash={() => bulkMoveToKind("trash")}
            onBulkMove={bulkMoveMessages}
            onClearChecked={() => setCheckedIds(new Set())}
          />
          <div className="hidden w-1 cursor-col-resize bg-white hover:bg-selected md:block" onPointerDown={startResize} aria-label="리스트 폭 조절" />
          <ReadingPane
            message={selectedMessage}
            mode={mode}
            mailboxes={flatMailboxes}
            onRetry={retry}
            onReply={() => openReply()}
            onToggleFlagged={toggleFlagged}
            onArchive={(message) => moveToKind(message, "archive")}
            onTrash={(message) => moveToKind(message, "trash")}
            onMove={moveMessage}
            onMarkUnread={markUnread}
          />
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

function flattenMailboxes(mailboxes: Mailbox[]): Mailbox[] {
  return mailboxes.flatMap((mailbox) => [mailbox, ...flattenMailboxes(mailbox.children ?? [])]);
}
