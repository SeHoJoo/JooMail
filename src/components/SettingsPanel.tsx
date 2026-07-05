import { useEffect, useMemo, useState } from "react";
import type { Account, MailRule, Mailbox, RuleActionType, RuleConditionField, RuleMatch } from "../types";
import packageJSON from "../../package.json";
import { Icon } from "./Icon";

type SettingsPanelProps = {
  account: Account;
  displayName: string;
  onDisplayNameChange: (value: string) => void;
  remoteImagesEnabled: boolean;
  onRemoteImagesChange: (enabled: boolean) => void;
  mailboxes: Mailbox[];
  onLoadRules?: () => Promise<MailRule[]>;
  onSaveRules?: (rules: MailRule[]) => Promise<MailRule[]>;
  onLogout?: () => void;
  onClose: () => void;
};

export function SettingsPanel({ account, displayName, onDisplayNameChange, remoteImagesEnabled, onRemoteImagesChange, mailboxes, onLoadRules, onSaveRules, onLogout, onClose }: SettingsPanelProps) {
  const [rules, setRules] = useState<MailRule[]>([]);
  const [rulesLoading, setRulesLoading] = useState(false);
  const [rulesSaving, setRulesSaving] = useState(false);
  const [rulesMessage, setRulesMessage] = useState("");
  const selectableMailboxes = useMemo(() => mailboxes.filter((mailbox) => mailbox.selectable !== false), [mailboxes]);
  const firstMailboxId = selectableMailboxes[0]?.id ?? "";
  const rulesDisabled = !onLoadRules || !onSaveRules;
  const rulesInvalid = rules.some((rule) => !rule.condition.value.trim() || (rule.action.type === "move" && !rule.action.mailboxId));

  useEffect(() => {
    if (!onLoadRules) {
      setRules([]);
      setRulesMessage("제품 연결 후 사용할 수 있습니다.");
      return;
    }
    let cancelled = false;
    setRulesLoading(true);
    setRulesMessage("");
    onLoadRules()
      .then((nextRules) => {
        if (cancelled) return;
        setRules(nextRules);
        setRulesMessage(nextRules.length ? "" : "저장된 규칙이 없습니다.");
      })
      .catch((error) => {
        if (cancelled) return;
        setRules([]);
        setRulesMessage(ruleErrorMessage(error));
      })
      .finally(() => {
        if (!cancelled) setRulesLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [onLoadRules]);

  function addRule() {
    setRules((current) => [
      ...current,
      {
        condition: { field: "senderDomain", match: "contains", value: "" },
        action: firstMailboxId ? { type: "move", mailboxId: firstMailboxId } : { type: "moveSpam" },
      },
    ]);
    setRulesMessage("");
  }

  function updateRule(index: number, updater: (rule: MailRule) => MailRule) {
    setRules((current) => current.map((rule, ruleIndex) => (ruleIndex === index ? updater(rule) : rule)));
    setRulesMessage("");
  }

  function removeRule(index: number) {
    setRules((current) => current.filter((_, ruleIndex) => ruleIndex !== index));
    setRulesMessage("");
  }

  async function saveRules() {
    if (!onSaveRules || rulesInvalid) return;
    setRulesSaving(true);
    setRulesMessage("");
    try {
      const savedRules = await onSaveRules(rules);
      setRules(savedRules);
      setRulesMessage("규칙을 저장했습니다.");
    } catch (error) {
      setRulesMessage(ruleErrorMessage(error));
    } finally {
      setRulesSaving(false);
    }
  }

  return (
    <div className="fixed inset-0 z-30 hidden bg-black/10 md:block" role="presentation" onMouseDown={onClose}>
      <section
        className="absolute right-4 top-[60px] flex max-h-[calc(100vh-76px)] w-[460px] flex-col rounded-lg border border-line bg-white shadow-compose"
        aria-label="설정"
        onMouseDown={(event) => event.stopPropagation()}
      >
        <header className="flex h-12 items-center border-b border-line px-4">
          <h2 className="text-[14px] font-bold text-ink">설정</h2>
          <button className="ml-auto flex h-8 w-8 items-center justify-center rounded-md text-muted hover:bg-[#f7f8f9] hover:text-text" aria-label="설정 닫기" onClick={onClose} type="button">
            <Icon name="close" className="h-4 w-4" />
          </button>
        </header>
        <div className="min-h-0 overflow-y-auto px-4 py-3">
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

          <div className="border-b border-line py-3">
            <div className="flex items-center gap-2">
              <div>
                <div className="text-[12px] font-bold text-[#5b6169]">규칙</div>
                <div className="mt-0.5 text-[11.5px] leading-4 text-muted">발신자와 제목으로 메일을 폴더에 분류합니다.</div>
              </div>
              <button
                className="ml-auto h-8 rounded-md border border-line px-3 text-[12px] font-medium text-text hover:bg-[#f7f8f9] disabled:cursor-not-allowed disabled:text-muted"
                disabled={rulesDisabled || rulesLoading}
                onClick={addRule}
                type="button"
              >
                추가
              </button>
            </div>

            {rulesLoading ? <div className="mt-3 h-9 rounded-md bg-[#f3f4f6]" /> : null}

            {!rulesLoading && rules.length ? (
              <div className="mt-3 space-y-2">
                {rules.map((rule, index) => (
                  <RuleRow
                    key={index}
                    rule={rule}
                    mailboxes={selectableMailboxes}
                    disabled={rulesSaving}
                    onChange={(nextRule) => updateRule(index, () => nextRule)}
                    onRemove={() => removeRule(index)}
                  />
                ))}
              </div>
            ) : null}

            {rulesMessage ? <div className="mt-2 text-[11.5px] leading-4 text-muted">{rulesMessage}</div> : null}

            <div className="mt-3 flex items-center justify-between">
              <div className="text-[11.5px] text-muted">라벨과 영구 삭제 규칙은 제외</div>
              <button
                className="h-8 rounded-md bg-accent px-3 text-[12px] font-bold text-white hover:bg-accent-strong disabled:cursor-not-allowed disabled:bg-[#c6ccd4]"
                disabled={rulesDisabled || rulesLoading || rulesSaving || rulesInvalid}
                onClick={saveRules}
                type="button"
              >
                {rulesSaving ? "저장 중" : "저장"}
              </button>
            </div>
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

function RuleRow({ rule, mailboxes, disabled, onChange, onRemove }: { rule: MailRule; mailboxes: Mailbox[]; disabled: boolean; onChange: (rule: MailRule) => void; onRemove: () => void }) {
  const field = normalizeField(rule.condition.field);
  const match = field === "subject" ? "contains" : normalizeMatch(rule.condition.match);
  const actionType = normalizeActionType(rule.action.type);
  const mailboxId = rule.action.mailboxId ?? mailboxes[0]?.id ?? "";

  function changeField(nextField: RuleConditionField) {
    onChange({
      ...rule,
      condition: {
        ...rule.condition,
        field: nextField,
        match: nextField === "subject" ? "contains" : match,
      },
    });
  }

  function changeAction(nextType: RuleActionType) {
    onChange({
      ...rule,
      action: nextType === "move" ? { type: "move", mailboxId } : { type: nextType },
    });
  }

  return (
    <div className="rounded-md border border-line bg-[#fbfbfc] p-2">
      <div className="grid grid-cols-[1fr_96px] gap-2">
        <select
          className="h-8 rounded-md border border-[#dfe2e6] bg-white px-2 text-[12px] text-ink outline-none focus:border-accent"
          value={field}
          disabled={disabled}
          onChange={(event) => changeField(normalizeField(event.target.value))}
        >
          <option value="senderDomain">발신 도메인</option>
          <option value="senderEmail">발신 주소</option>
          <option value="subject">제목</option>
        </select>
        <select
          className="h-8 rounded-md border border-[#dfe2e6] bg-white px-2 text-[12px] text-ink outline-none focus:border-accent disabled:text-muted"
          value={match}
          disabled={disabled || field === "subject"}
          onChange={(event) =>
            onChange({
              ...rule,
              condition: { ...rule.condition, match: normalizeMatch(event.target.value) },
            })
          }
        >
          <option value="contains">포함</option>
          <option value="equals">일치</option>
        </select>
      </div>
      <input
        className="mt-2 h-8 w-full rounded-md border border-[#dfe2e6] bg-white px-2 text-[12px] text-ink outline-none placeholder:text-muted focus:border-accent"
        value={rule.condition.value}
        disabled={disabled}
        onChange={(event) =>
          onChange({
            ...rule,
            condition: { ...rule.condition, field, match, value: event.target.value },
          })
        }
        placeholder={field === "subject" ? "제목 키워드" : field === "senderEmail" ? "name@example.com" : "example.com"}
      />
      <div className="mt-2 grid grid-cols-[106px_1fr_32px] gap-2">
        <select
          className="h-8 rounded-md border border-[#dfe2e6] bg-white px-2 text-[12px] text-ink outline-none focus:border-accent"
          value={actionType}
          disabled={disabled}
          onChange={(event) => changeAction(normalizeActionType(event.target.value))}
        >
          <option value="move">이동</option>
          <option value="moveSpam">스팸</option>
          <option value="moveTrash">휴지통</option>
        </select>
        <select
          className="h-8 rounded-md border border-[#dfe2e6] bg-white px-2 text-[12px] text-ink outline-none focus:border-accent disabled:text-muted"
          value={actionType === "move" ? mailboxId : ""}
          disabled={disabled || actionType !== "move"}
          onChange={(event) =>
            onChange({
              ...rule,
              action: { type: "move", mailboxId: event.target.value },
            })
          }
        >
          {actionType !== "move" ? <option value="">자동 선택</option> : null}
          {mailboxes.map((mailbox) => (
            <option key={mailbox.id} value={mailbox.id}>
              {mailbox.label}
            </option>
          ))}
        </select>
        <button className="flex h-8 w-8 items-center justify-center rounded-md text-muted hover:bg-white hover:text-danger" aria-label="규칙 삭제" disabled={disabled} onClick={onRemove} type="button">
          <Icon name="close" className="h-4 w-4" />
        </button>
      </div>
    </div>
  );
}

function normalizeField(value: string): RuleConditionField {
  if (value === "senderEmail" || value === "subject") return value;
  return "senderDomain";
}

function normalizeMatch(value: string): RuleMatch {
  return value === "equals" ? "equals" : "contains";
}

function normalizeActionType(value: string): RuleActionType {
  if (value === "moveSpam" || value === "moveTrash") return value;
  return "move";
}

function ruleErrorMessage(error: unknown) {
  if (error instanceof Error && error.message === "rules unavailable") {
    return "ManageSieve 설정 후 사용할 수 있습니다.";
  }
  if (error instanceof Error && error.message === "rules conflict") {
    return "기존 활성 Sieve 스크립트에 JooMail 관리 블록이 없습니다.";
  }
  return "규칙을 불러오지 못했습니다.";
}
