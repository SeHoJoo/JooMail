import { useEffect, useMemo, useRef, useState } from "react";
import { accounts, messages as allMessages } from "./data";
import type { MockMode } from "./types";
import { ComposePanel } from "./components/ComposePanel";
import { MessageList } from "./components/MessageList";
import { MobileInbox } from "./components/MobileInbox";
import { ReadingPane } from "./components/ReadingPane";
import { Sidebar } from "./components/Sidebar";
import { Toolbar } from "./components/Toolbar";
import { DevStateSwitcher, type QaState } from "./components/DevStateSwitcher";

const LIST_WIDTH_KEY = "joomail:list-width";
const QA_STATES: QaState[] = ["normal", "loading", "error", "empty", "empty-reading", "search", "search-empty", "multiselect", "compose"];
const SEARCH_QA_QUERY = "MIME";
const SEARCH_EMPTY_QA_QUERY = "qa-no-results-000";

export default function App() {
  return <AppShell />;
}

export function AppShell() {
  const [accountId, setAccountId] = useState(accounts[0].id);
  const [mailboxId, setMailboxId] = useState("inbox");
  const [selectedMessageId, setSelectedMessageId] = useState("m1");
  const [checkedIds, setCheckedIds] = useState<Set<string>>(new Set());
  const [search, setSearch] = useState("");
  const [composeOpen, setComposeOpen] = useState(false);
  const [replyMessageId, setReplyMessageId] = useState<string | null>(null);
  const [mode, setMode] = useState<MockMode>("normal");
  const [listWidth, setListWidth] = useState(() => Number(localStorage.getItem(LIST_WIDTH_KEY)) || 388);
  const [forceEmptyList, setForceEmptyList] = useState(false);
  const shellRef = useRef<HTMLDivElement>(null);
  const searchInputRef = useRef<HTMLInputElement>(null);
  const initialQaAppliedRef = useRef(false);

  const selectedAccount = accounts.find((account) => account.id === accountId) ?? accounts[0];

  const visibleMessages = useMemo(() => {
    if (forceEmptyList) return [];
    const accountMessages = allMessages.filter((message) => message.accountId === accountId);
    if (search.trim()) {
      const query = search.trim().toLowerCase();
      return accountMessages.filter((message) =>
        [message.sender, message.subject, message.snippet, ...message.body].some((field) => field.toLowerCase().includes(query)),
      );
    }
    return accountMessages.filter((message) => message.mailboxId === mailboxId);
  }, [accountId, forceEmptyList, mailboxId, search]);

  const selectedMessage = visibleMessages.find((message) => message.id === selectedMessageId);
  const composeMessage = replyMessageId ? allMessages.find((message) => message.id === replyMessageId) : undefined;
  const selectedMailbox = selectedAccount.mailboxes
    .flatMap((mailbox) => [mailbox, ...(mailbox.children ?? [])])
    .find((mailbox) => mailbox.id === mailboxId);

  useEffect(() => {
    localStorage.setItem(LIST_WIDTH_KEY, String(listWidth));
  }, [listWidth]);

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
    const accountMessages = allMessages.filter((message) => message.accountId === accountId);
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
    triggerLoading();
  }

  function selectMailbox(nextMailboxId: string) {
    setMailboxId(nextMailboxId);
    setSearch("");
    setForceEmptyList(false);
    setCheckedIds(new Set());
    setMode(nextMailboxId === "spam" ? "error" : "loading");
    window.setTimeout(() => setMode(nextMailboxId === "spam" ? "error" : "normal"), 420);
  }

  function triggerLoading() {
    setForceEmptyList(false);
    setMode("loading");
    window.setTimeout(() => setMode("normal"), 420);
  }

  function retry() {
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
      {composeOpen ? <ComposePanel account={selectedAccount} message={composeMessage} onClose={closeCompose} /> : null}
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
