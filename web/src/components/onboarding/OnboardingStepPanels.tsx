import { Link } from "react-router-dom";

import type { OnboardingState } from "../../gen/onboarding/v1/onboarding_pb";
import { useI18n } from "../../lib/i18n";
import StarterQuestions from "../StarterQuestions";
import { formatTimestamp, styles } from "./onboardingStepUtils";

export function WelcomeStep({
  workspaceName,
  confirmedLanguage,
  onWorkspaceNameChange,
  onConfirmedLanguageChange,
  onNext,
}: {
  workspaceName: string;
  confirmedLanguage: boolean;
  onWorkspaceNameChange: (value: string) => void;
  onConfirmedLanguageChange: (value: boolean) => void;
  onNext: () => void;
}) {
  const { t } = useI18n();

  return (
    <section className={styles.card}>
      <div className="grid gap-5 lg:grid-cols-[minmax(0,1fr)_280px]">
        <div className="space-y-4">
          <label className="grid gap-2 text-sm font-medium text-slate-800">
            <span>{t("onboarding.step1.workspaceName")}</span>
            <input
              className={styles.input}
              value={workspaceName}
              onChange={(event) => onWorkspaceNameChange(event.target.value)}
              placeholder={t("onboarding.step1.workspacePlaceholder")}
            />
          </label>
          <label className="flex items-start gap-3 rounded-3xl border border-slate-200 bg-slate-50 p-4">
            <input
              type="checkbox"
              checked={confirmedLanguage}
              onChange={(event) =>
                onConfirmedLanguageChange(event.target.checked)
              }
              className="mt-1 h-4 w-4 rounded border-slate-300"
            />
            <span className="text-sm leading-6 text-slate-700">
              {t("onboarding.step1.languageConfirm")}
            </span>
          </label>
        </div>
        <div className="rounded-3xl bg-slate-50 p-5 text-sm leading-6 text-slate-700">
          <p className="font-semibold text-slate-900">
            {t("onboarding.step1.summaryTitle")}
          </p>
          <p className="mt-3">{t("onboarding.step1.summaryBody")}</p>
        </div>
      </div>
      <div className="mt-6 flex justify-end">
        <button
          type="button"
          className={styles.buttonPrimary}
          onClick={onNext}
        >
          {t("onboarding.common.next")}
        </button>
      </div>
    </section>
  );
}

export function InstallStep({
  state,
  copiedKey,
  onCopy,
}: {
  state: OnboardingState;
  copiedKey: string | null;
  onCopy: (key: string, text: string) => void;
}) {
  const { t } = useI18n();

  return (
    <>
      <section className={styles.card}>
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div>
            <h2 className="text-lg font-semibold text-slate-900">
              {t("onboarding.step2.commandTitle")}
            </h2>
            <p className="mt-2 text-sm leading-6 text-slate-600">
              {t("onboarding.step2.commandBody")}
            </p>
          </div>
          <button
            type="button"
            className={styles.buttonSecondary}
            onClick={() => onCopy("docker", state.dockerRunCommand)}
          >
            {copiedKey === "docker"
              ? t("onboarding.common.copied")
              : t("onboarding.common.copy")}
          </button>
        </div>
        <pre className={`${styles.codeBlock} mt-4`}>
          <code>{state.dockerRunCommand}</code>
        </pre>
      </section>

      <section className={styles.card}>
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div>
            <h2 className="text-lg font-semibold text-slate-900">
              {t("onboarding.step2.statusTitle")}
            </h2>
            <p className="mt-2 text-sm text-slate-600">
              {state.agentConnected
                ? t("onboarding.step2.statusConnected")
                : t("onboarding.step2.statusWaiting")}
            </p>
          </div>
          <span
            className={[
              "rounded-full px-3 py-1 text-xs font-semibold uppercase tracking-[0.18em]",
              state.agentConnected
                ? "bg-emerald-100 text-emerald-700"
                : "bg-amber-100 text-amber-700",
            ].join(" ")}
          >
            {state.agentConnected
              ? t("onboarding.common.connected")
              : t("onboarding.common.pending")}
          </span>
        </div>
        {state.agentConnectedAt ? (
          <p className="mt-4 text-sm text-slate-600">
            {t("onboarding.step2.connectedAt", {
              time: formatTimestamp(state.agentConnectedAt),
            })}
          </p>
        ) : null}
      </section>

      <section className={styles.card}>
        <details open={state.agentConnectionTimedOut}>
          <summary className="cursor-pointer text-lg font-semibold text-slate-900">
            {t("onboarding.step2.troubleshootingTitle")}
          </summary>
          <div className="mt-4 grid gap-4">
            {[1, 2, 3, 4, 5].map((index) => (
              <article
                key={index}
                className="rounded-3xl border border-slate-200 bg-slate-50 p-4"
              >
                <p className="font-medium text-slate-900">
                  {t(`onboarding.step2.troubleshoot.${index}.title` as never)}
                </p>
                <p className="mt-2 text-sm leading-6 text-slate-600">
                  {t(`onboarding.step2.troubleshoot.${index}.body` as never)}
                </p>
                <div className="mt-3 flex justify-end">
                  <button
                    type="button"
                    className={styles.buttonSecondary}
                    onClick={() =>
                      onCopy(
                        `trouble-${index}`,
                        t(
                          `onboarding.step2.troubleshoot.${index}.fix` as never,
                        ),
                      )
                    }
                  >
                    {t("onboarding.common.copyFix")}
                  </button>
                </div>
                <pre className={`${styles.codeBlock} mt-3`}>
                  <code>
                    {t(`onboarding.step2.troubleshoot.${index}.fix` as never)}
                  </code>
                </pre>
              </article>
            ))}
          </div>
        </details>
      </section>
    </>
  );
}

export function DatabaseStep({
  state,
  copiedKey,
  dbHost,
  dbPort,
  dbName,
  connectionString,
  dbSubmitting,
  suggestedConnectionString,
  onDbHostChange,
  onDbPortChange,
  onDbNameChange,
  onConnectionStringChange,
  onCopy,
  onGenerateSql,
  onUseSuggested,
  onVerify,
}: {
  state: OnboardingState;
  copiedKey: string | null;
  dbHost: string;
  dbPort: string;
  dbName: string;
  connectionString: string;
  dbSubmitting: boolean;
  suggestedConnectionString: string;
  onDbHostChange: (value: string) => void;
  onDbPortChange: (value: string) => void;
  onDbNameChange: (value: string) => void;
  onConnectionStringChange: (value: string) => void;
  onCopy: (key: string, text: string) => void;
  onGenerateSql: () => void;
  onUseSuggested: () => void;
  onVerify: () => void;
}) {
  const { t } = useI18n();

  return (
    <>
      <section className={styles.card}>
        <div className="grid gap-4 md:grid-cols-3">
          <label className="grid gap-2 text-sm font-medium text-slate-800">
            <span>{t("onboarding.step3.host")}</span>
            <input
              className={styles.input}
              value={dbHost}
              onChange={(event) => onDbHostChange(event.target.value)}
              placeholder={t("onboarding.step3.hostPlaceholder")}
            />
          </label>
          <label className="grid gap-2 text-sm font-medium text-slate-800">
            <span>{t("onboarding.step3.port")}</span>
            <input
              className={styles.input}
              value={dbPort}
              onChange={(event) => onDbPortChange(event.target.value)}
            />
          </label>
          <label className="grid gap-2 text-sm font-medium text-slate-800">
            <span>{t("onboarding.step3.database")}</span>
            <input
              className={styles.input}
              value={dbName}
              onChange={(event) => onDbNameChange(event.target.value)}
              placeholder={t("onboarding.step3.databasePlaceholder")}
            />
          </label>
        </div>
        <div className="mt-6 flex justify-end">
          <button
            type="button"
            className={styles.buttonPrimary}
            disabled={dbSubmitting}
            onClick={onGenerateSql}
          >
            {t("onboarding.step3.generateSql")}
          </button>
        </div>
      </section>

      <section className={styles.card}>
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div>
            <h2 className="text-lg font-semibold text-slate-900">
              {t("onboarding.step3.sqlTitle")}
            </h2>
            <p className="mt-2 text-sm leading-6 text-slate-600">
              {t("onboarding.step3.sqlBody")}
            </p>
          </div>
          <button
            type="button"
            className={styles.buttonSecondary}
            onClick={() => onCopy("sql", state.dbSetupSql)}
          >
            {copiedKey === "sql"
              ? t("onboarding.common.copied")
              : t("onboarding.common.copy")}
          </button>
        </div>
        <pre className={`${styles.codeBlock} mt-4`}>
          <code>{state.dbSetupSql}</code>
        </pre>
        <div className="mt-4 grid gap-3 lg:grid-cols-3">
          <div className="rounded-3xl bg-slate-50 p-4 text-sm leading-6 text-slate-700">
            {t("onboarding.step3.whyReadOnly")}
          </div>
          <div className="rounded-3xl bg-slate-50 p-4 text-sm leading-6 text-slate-700">
            {t("onboarding.step3.whySeparateUser")}
          </div>
          <div className="rounded-3xl bg-slate-50 p-4 text-sm leading-6 text-slate-700">
            {t("onboarding.step3.whyReplica")}
          </div>
        </div>
        <div className="mt-4 rounded-3xl border border-sky-200 bg-sky-50 p-4 text-sm leading-6 text-sky-900">
          {t("onboarding.step3.transport")}
        </div>
      </section>

      <section className={styles.card}>
        {suggestedConnectionString ? (
          <div className="mb-6 rounded-3xl border border-slate-200 bg-slate-50 p-4">
            <div className="flex flex-wrap items-center justify-between gap-3">
              <div>
                <h2 className="text-lg font-semibold text-slate-900">
                  {t("onboarding.step3.suggestedConnectionString")}
                </h2>
                <p className="mt-2 text-sm leading-6 text-slate-600">
                  {t("onboarding.step3.suggestedConnectionBody")}
                </p>
              </div>
              <div className="flex flex-wrap gap-2">
                <button
                  type="button"
                  className={styles.buttonSecondary}
                  onClick={() => onCopy("dsn", suggestedConnectionString)}
                >
                  {copiedKey === "dsn"
                    ? t("onboarding.common.copied")
                    : t("onboarding.common.copy")}
                </button>
                <button
                  type="button"
                  className={styles.buttonSecondary}
                  onClick={onUseSuggested}
                >
                  {t("onboarding.step3.useSuggestedConnection")}
                </button>
              </div>
            </div>
            <pre className={`${styles.codeBlock} mt-4`}>
              <code>{suggestedConnectionString}</code>
            </pre>
          </div>
        ) : null}
        <label className="grid gap-2 text-sm font-medium text-slate-800">
          <span>{t("onboarding.step3.connectionString")}</span>
          <textarea
            className={styles.textarea}
            value={connectionString}
            onChange={(event) => onConnectionStringChange(event.target.value)}
            placeholder={t("onboarding.step3.connectionPlaceholder")}
          />
        </label>
        {state.dbErrorMessageKo ? (
          <div className={`${styles.bannerError} mt-4`}>
            {state.dbErrorMessageKo}
          </div>
        ) : null}
        {state.dbVerifiedAt ? (
          <div className={`${styles.bannerSuccess} mt-4`}>
            {t("onboarding.step3.verifiedAt", {
              time: formatTimestamp(state.dbVerifiedAt),
            })}
          </div>
        ) : null}
        <div className="mt-6 flex justify-end">
          <button
            type="button"
            className={styles.buttonPrimary}
            disabled={dbSubmitting}
            onClick={onVerify}
          >
            {t("onboarding.step3.verify")}
          </button>
        </div>
      </section>
    </>
  );
}

export function SchemaStep({
  state,
  schemaSubmitting,
  holdSchemaSummary,
  onRun,
  onNext,
}: {
  state: OnboardingState;
  schemaSubmitting: boolean;
  holdSchemaSummary: boolean;
  onRun: () => void;
  onNext: () => void;
}) {
  const { t } = useI18n();

  return (
    <>
      <section className={styles.card}>
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div>
            <h2 className="text-lg font-semibold text-slate-900">
              {t("onboarding.step4.title")}
            </h2>
            <p className="mt-2 text-sm leading-6 text-slate-600">
              {t("onboarding.step4.body")}
            </p>
          </div>
          <button
            type="button"
            className={styles.buttonPrimary}
            disabled={schemaSubmitting}
            onClick={onRun}
          >
            {schemaSubmitting
              ? t("onboarding.step4.running")
              : t("onboarding.step4.run")}
          </button>
        </div>
      </section>

      {(holdSchemaSummary || !!state.schemaVersionId) && (
        <section className={styles.card}>
          <div className="grid gap-4 md:grid-cols-3">
            <div className="rounded-3xl bg-slate-50 p-5">
              <p className="text-xs uppercase tracking-[0.14em] text-slate-400">
                {t("onboarding.step4.tables")}
              </p>
              <p className="mt-2 text-2xl font-semibold text-slate-900">
                {state.schemaTableCount}
              </p>
            </div>
            <div className="rounded-3xl bg-slate-50 p-5">
              <p className="text-xs uppercase tracking-[0.14em] text-slate-400">
                {t("onboarding.step4.columns")}
              </p>
              <p className="mt-2 text-2xl font-semibold text-slate-900">
                {state.schemaColumnCount}
              </p>
            </div>
            <div className="rounded-3xl bg-slate-50 p-5">
              <p className="text-xs uppercase tracking-[0.14em] text-slate-400">
                {t("onboarding.step4.foreignKeys")}
              </p>
              <p className="mt-2 text-2xl font-semibold text-slate-900">
                {state.schemaForeignKeyCount}
              </p>
            </div>
          </div>
          <div className="mt-6 flex justify-end">
            <button
              type="button"
              className={styles.buttonPrimary}
              onClick={onNext}
            >
              {t("onboarding.common.next")}
            </button>
          </div>
        </section>
      )}
    </>
  );
}

export function StarterStep({
  tenantId,
  onPick,
  onComplete,
}: {
  tenantId: string;
  onPick: (text: string) => void;
  onComplete: () => void;
}) {
  const { t } = useI18n();

  return (
    <section className={styles.card}>
      <div className="grid gap-6 lg:grid-cols-[minmax(0,1fr)_320px]">
        <div>
          <h2 className="text-lg font-semibold text-slate-900">
            {t("onboarding.step6.readyTitle")}
          </h2>
          <p className="mt-3 max-w-2xl text-sm leading-6 text-slate-700">
            {t("onboarding.step6.readyBody")}
          </p>
          <div className="mt-6">
            <StarterQuestions tenantId={tenantId} onPick={onPick} />
          </div>
        </div>
        <div className="rounded-3xl bg-emerald-50 p-5">
          <p className="text-sm leading-6 text-emerald-900">
            {t("onboarding.step6.next")}
          </p>
          <button
            type="button"
            className={`${styles.buttonPrimary} mt-4 w-full`}
            onClick={onComplete}
          >
            {t("onboarding.step6.finish")}
          </button>
        </div>
      </div>
    </section>
  );
}

export function CompleteStep({
  state,
  runningStep7,
  inviteModalOpen,
  inviteEmails,
  inviteSubmitting,
  onOpenInviteModal,
  onCloseInviteModal,
  onInviteEmailsChange,
  onComplete,
  onCreateInvites,
}: {
  state: OnboardingState;
  runningStep7: boolean;
  inviteModalOpen: boolean;
  inviteEmails: string;
  inviteSubmitting: boolean;
  onOpenInviteModal: () => void;
  onCloseInviteModal: () => void;
  onInviteEmailsChange: (value: string) => void;
  onComplete: () => void;
  onCreateInvites: () => void;
}) {
  const { t } = useI18n();

  return (
    <>
      <section className={styles.card}>
        <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
          <div className="rounded-3xl bg-slate-50 p-5">
            <p className="text-xs uppercase tracking-[0.14em] text-slate-400">
              {t("onboarding.step7.workspace")}
            </p>
            <p className="mt-2 text-lg font-semibold text-slate-900">
              {state.name}
            </p>
          </div>
          <div className="rounded-3xl bg-emerald-50 p-5">
            <p className="text-xs uppercase tracking-[0.14em] text-emerald-700">
              {t("onboarding.step7.agent")}
            </p>
            <p className="mt-2 text-lg font-semibold text-emerald-900">
              {state.agentConnected
                ? t("onboarding.common.connected")
                : t("onboarding.common.pending")}
            </p>
          </div>
          <div className="rounded-3xl bg-emerald-50 p-5">
            <p className="text-xs uppercase tracking-[0.14em] text-emerald-700">
              {t("onboarding.step7.database")}
            </p>
            <p className="mt-2 text-lg font-semibold text-emerald-900">
              {state.dbVerifiedAt
                ? t("onboarding.common.connected")
                : t("onboarding.common.pending")}
            </p>
          </div>
          <div className="rounded-3xl bg-slate-50 p-5">
            <p className="text-xs uppercase tracking-[0.14em] text-slate-400">
              {t("onboarding.step7.semanticLayer")}
            </p>
            <p className="mt-2 break-all text-sm font-semibold text-slate-900">
              {state.semanticLayerId || t("common.na")}
            </p>
          </div>
        </div>
        <div className="mt-6 flex flex-wrap gap-3">
          <Link to="/chat" className={styles.buttonPrimary}>
            {t("onboarding.step7.chatCta")}
          </Link>
          <button
            type="button"
            className={styles.buttonSecondary}
            onClick={onOpenInviteModal}
          >
            {t("onboarding.step7.inviteCta")}
          </button>
          {!state.onboardingComplete ? (
            <button
              type="button"
              className={styles.buttonSecondary}
              disabled={runningStep7}
              onClick={onComplete}
            >
              {t("onboarding.step7.complete")}
            </button>
          ) : null}
        </div>
      </section>

      <section className={styles.card}>
        <h2 className="text-lg font-semibold text-slate-900">
          {t("onboarding.step7.invitesTitle")}
        </h2>
        <div className="mt-4 flex flex-col gap-3">
          {state.invites.length === 0 ? (
            <p className="text-sm text-slate-600">
              {t("onboarding.step7.invitesEmpty")}
            </p>
          ) : (
            state.invites.map((invite) => (
              <div
                key={invite.id}
                className="rounded-3xl border border-slate-200 bg-slate-50 px-4 py-3"
              >
                <div className="font-medium text-slate-900">
                  {invite.email}
                </div>
                <div className="mt-1 text-xs text-slate-500">
                  {formatTimestamp(invite.createdAt)}
                </div>
              </div>
            ))
          )}
        </div>
      </section>

      {inviteModalOpen ? (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/40 p-4">
          <div className="w-full max-w-xl rounded-[32px] bg-white p-6 shadow-2xl">
            <h2 className="text-xl font-semibold text-slate-900">
              {t("onboarding.step7.inviteModalTitle")}
            </h2>
            <p className="mt-2 text-sm leading-6 text-slate-600">
              {t("onboarding.step7.inviteModalBody")}
            </p>
            <textarea
              className={`${styles.textarea} mt-4`}
              value={inviteEmails}
              onChange={(event) => onInviteEmailsChange(event.target.value)}
              placeholder={t("onboarding.step7.invitePlaceholder")}
            />
            <div className="mt-6 flex justify-end gap-3">
              <button
                type="button"
                className={styles.buttonSecondary}
                onClick={onCloseInviteModal}
              >
                {t("onboarding.common.cancel")}
              </button>
              <button
                type="button"
                className={styles.buttonPrimary}
                disabled={inviteSubmitting}
                onClick={onCreateInvites}
              >
                {t("onboarding.step7.saveInvites")}
              </button>
            </div>
          </div>
        </div>
      ) : null}
    </>
  );
}
