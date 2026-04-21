import {
  createContext,
  createElement,
  useContext,
  useEffect,
  useMemo,
  useState,
  type PropsWithChildren,
} from "react";

export type Locale = "en" | "ko";

export const defaultLocale: Locale = "en";
export const localeStorageKey = "mission.frontend.locale";

type TranslationParams = Record<string, number | string>;

const commonEn = {
  "common.appLabel": "Mission",
  "common.language": "Language",
  "common.loading": "loading",
  "common.na": "n/a",
  "common.unknown": "unknown",
  "common.online": "online",
  "common.offline": "offline",
  "common.yes": "Yes",
  "common.no": "No",
} as const;

const layoutEn = {
  "layout.nav.tenants": "Tenants",
  "layout.nav.questions": "Ask",
  "layout.nav.semanticLayer": "Semantic Layer",
  "layout.nav.agents": "Agents",
  "layout.theme.label": "Theme",
  "layout.theme.system": "System",
  "layout.theme.light": "Light",
  "layout.theme.dark": "Dark",
} as const;

const tenantsEn = {
  "tenants.hero.title": "Tenants & agent tokens",
  "tenants.hero.subtitle":
    "Create a tenant, issue a scoped token, and use it to boot an edge agent. Plaintext tokens appear exactly once; copy immediately or revoke and re-issue.",
  "tenants.list.title": "Tenants",
  "tenants.list.subtitle": "Workspaces you belong to.",
  "tenants.list.empty": "No tenants yet. Create one below.",
  "tenants.form.slug": "Slug",
  "tenants.form.name": "Name",
  "tenants.form.slugPlaceholder": "ecotech-demo",
  "tenants.form.namePlaceholder": "Ecotech demo tenant",
  "tenants.form.create": "Create tenant",
  "tenants.form.creating": "Creating...",
  "tenants.detail.selectPrompt": "Select a tenant",
  "tenants.detail.subtitle": "Agent tokens scoped to this tenant.",
  "tenants.tokens.copyNow":
    "Copy this token now. It will not be shown again.",
  "tenants.tokens.copy": "Copy",
  "tenants.tokens.dismiss": "Dismiss",
  "tenants.tokens.empty": "No tokens issued for this tenant yet.",
  "tenants.tokens.id": "id",
  "tenants.tokens.issued": "issued",
  "tenants.tokens.lastUsed": "last used",
  "tenants.tokens.revokedAt": "revoked",
  "tenants.tokens.revoke": "Revoke",
  "tenants.tokens.revoking": "Revoking...",
  "tenants.tokens.revoked": "Revoked",
  "tenants.tokens.label": "Label",
  "tenants.tokens.labelPlaceholder": "edge-01",
  "tenants.tokens.issue": "Issue token",
  "tenants.tokens.issuing": "Issuing...",
  "tenants.tokens.pickTenant": "Pick a tenant to manage its tokens.",
} as const;

const agentsEn = {
  "agents.hero.title": "Week 2.1 agent tunnel debug surface",
  "agents.hero.subtitle":
    "Poll the control plane every 5 seconds for live agent presence and expose a manual ping to validate the outbound tunnel end-to-end.",
  "agents.health.api": "API",
  "agents.health.database": "Postgres",
  "agents.section.title": "Agent sessions",
  "agents.section.subtitle":
    "One runtime-scoped row per tenant token. Offline rows remain until replaced.",
  "agents.section.empty": "No edge agents connected yet.",
  "agents.meta.host": "Host",
  "agents.meta.version": "Version",
  "agents.meta.tokenLabel": "Token label",
  "agents.meta.tenant": "Tenant",
  "agents.meta.connected": "Connected",
  "agents.meta.lastHeartbeat": "Last heartbeat",
  "agents.meta.disconnected": "Disconnected",
  "agents.meta.lastPing": "Last ping",
  "agents.meta.lastPingValue": "{ms} ms at {time}",
  "agents.button.ping": "Ping agent",
  "agents.button.pinging": "Pinging...",
  "agents.count.one": "{count} session",
  "agents.count.other": "{count} sessions",
} as const;

const queryDebugEn = {
  "queryDebug.hero.title": "Read-only MySQL debug runner",
  "queryDebug.hero.subtitle":
    "Owner-only tooling for validating the edge-agent query path end to end. SQL is sent to a live tenant agent, executed on the local MySQL connection, and rendered back here as JSON.",
  "queryDebug.tenants.title": "Tenants",
  "queryDebug.tenants.subtitle": "Pick an owner-scoped workspace.",
  "queryDebug.tenants.empty": "No tenants available yet.",
  "queryDebug.detail.selectPrompt": "Select a tenant",
  "queryDebug.detail.subtitle":
    "Status reflects the most recent agent session for this tenant.",
  "queryDebug.meta.host": "Host",
  "queryDebug.meta.session": "Session",
  "queryDebug.meta.version": "Version",
  "queryDebug.meta.tokenLabel": "Token label",
  "queryDebug.form.sql": "SQL",
  "queryDebug.form.sqlPlaceholder": "SELECT 1",
  "queryDebug.form.run": "Run query",
  "queryDebug.form.running": "Running...",
  "queryDebug.form.available": "Query execution is available.",
  "queryDebug.form.unavailable": "Bring an edge agent online to run SQL.",
  "queryDebug.result.database": "Database",
  "queryDebug.result.user": "User",
  "queryDebug.result.elapsed": "Elapsed",
  "queryDebug.result.columns": "Columns",
} as const;

const chatEn = {
  "chat.hero.title": "Ask",
  "chat.hero.subtitle":
    "Turn a natural-language question into read-only SQL, run it safely, and show the result with the generated SQL and retry trace.",
  "chat.tenants.title": "Tenant selection",
  "chat.tenants.subtitle": "Choose the workspace to query.",
  "chat.tenants.empty": "No tenants are available.",
  "chat.tenants.guardrail":
    "Only SELECT, WITH, and SHOW are allowed. Unsafe constructs are blocked, and LIMIT 1000 may be injected automatically.",
  "chat.form.title": "Compose question",
  "chat.form.subtitle.selected":
    "Ask a natural-language question about {tenant}.",
  "chat.form.subtitle.unselected": "Select a tenant first.",
  "chat.form.label": "Question",
  "chat.form.defaultQuestion":
    "Show the average pH by station for the last 30 days.",
  "chat.form.placeholder":
    "Example: Show the average water-quality score by process and the stations with the highest issues last quarter.",
  "chat.form.help":
    "The result includes the generated SQL, executed SQL, and the attempt history.",
  "chat.form.submit": "Ask question",
  "chat.form.submitting": "Processing question...",
  "chat.history.title": "Recent questions",
  "chat.history.subtitle":
    "Questions and responses from this browser session are shown temporarily here.",
  "chat.history.empty":
    "Send your first question to see the summary, generated SQL, and result table here.",
  "chat.history.count.one": "{count} item",
  "chat.history.count.other": "{count} items",
  "chat.card.question": "Question",
  "chat.card.success": "Completed",
  "chat.card.error": "Failed",
  "chat.result.summaryTitle": "Summary (backend response)",
  "chat.result.rowCount": "Rows",
  "chat.result.elapsed": "Elapsed",
  "chat.result.safetyLimit": "Safety limit",
  "chat.result.limitInjected": "LIMIT 1000 applied automatically",
  "chat.result.noLimit": "No additional limit",
  "chat.result.dataTitle": "Result data",
  "chat.result.noRows": "No rows matched this query.",
  "chat.result.sqlAttemptsTitle": "Generated SQL and attempt history",
  "chat.result.originalSql": "Original SQL",
  "chat.result.executedSql": "Executed SQL",
  "chat.result.attempts": "Attempt history",
  "chat.attempt.success": "Success",
  "chat.attempt.failure": "Failed",
  "chat.attempt.successHelp":
    "This SQL passed validation and execution.",
  "chat.attempt.title": "Attempt {index} · {stage}",
  "chat.stage.generation": "generation",
  "chat.stage.validation": "validation",
  "chat.stage.execution": "execution",
  "chat.feedback.title": "Teach this result",
  "chat.feedback.subtitle":
    "Save a quick review so future answers in this tenant get better.",
  "chat.feedback.ratingHelpful": "Helpful",
  "chat.feedback.ratingNeedsWork": "Needs work",
  "chat.feedback.commentLabel": "Comment",
  "chat.feedback.commentPlaceholder":
    "What was useful or what should change next time?",
  "chat.feedback.correctedSqlLabel": "Corrected SQL (optional)",
  "chat.feedback.correctedSqlPlaceholder":
    "Paste the SQL you wish the system had produced.",
  "chat.feedback.submit": "Submit review",
  "chat.feedback.submitting": "Saving review...",
  "chat.feedback.success": "Saved your review for this query run.",
  "chat.examples.title": "Approved examples",
  "chat.examples.subtitle":
    "Owner-approved query examples that can be reused for similar future questions.",
  "chat.examples.loading": "Loading approved examples...",
  "chat.examples.empty": "No approved examples yet.",
  "chat.examples.sqlPreview": "Preview SQL",
  "chat.examples.archive": "Archive",
  "chat.examples.archiving": "Archiving...",
  "chat.examples.createTitle": "Save as approved example",
  "chat.examples.createSubtitle":
    "Owners can turn this run into reusable tenant memory.",
  "chat.examples.questionLabel": "Canonical question",
  "chat.examples.sqlLabel": "Canonical SQL",
  "chat.examples.sqlPlaceholder": "SELECT ...",
  "chat.examples.notesLabel": "Notes",
  "chat.examples.notesPlaceholder":
    "Explain when this example is safe to reuse.",
  "chat.examples.create": "Save example",
  "chat.examples.creating": "Saving example...",
  "chat.examples.createSuccess": "Saved a new approved example.",
} as const;

const starterQuestionsEn = {
  "starterQuestions.title": "Suggested starter questions",
  "starterQuestions.subtitle":
    "These are tenant-specific questions built from the approved semantic layer. Click one to see the product answer a real question immediately.",
  "starterQuestions.regenerate": "Regenerate",
  "starterQuestions.loading":
    "Generating starter questions for this workspace...",
  "starterQuestions.empty": "No starter questions are available yet.",
  "starterQuestions.error": "Failed to load starter questions.",
} as const;

const semanticEn = {
  "semantic.hero.title": "Semantic Layer",
  "semantic.hero.subtitle":
    "Draft Korean business descriptions from the latest schema version, review them, then save and approve.",
  "semantic.tenants.title": "Tenants",
  "semantic.tenants.subtitle": "Select the tenant to work on.",
  "semantic.tenants.empty": "No tenants are available.",
  "semantic.loading": "Loading...",
  "semantic.schemaNotCaptured.title": "Schema not captured yet",
  "semantic.schemaNotCaptured.body":
    "Run schema introspection first so the latest tenant_schemas version exists.",
  "semantic.draftNeeded.title": "Draft not created yet",
  "semantic.draftNeeded.body":
    "Generate a semantic-layer draft for the latest schema version before review starts.",
  "semantic.actions.generateDraft": "Generate draft",
  "semantic.actions.save": "Save",
  "semantic.actions.approve": "Approve",
  "semantic.actions.saving": "Saving...",
  "semantic.actions.approving": "Approving...",
  "semantic.actions.generating": "Generating...",
  "semantic.currentLayer.title": "Current editable layer",
  "semantic.approvedBaseline.title": "Approved baseline diff",
  "semantic.diff.none": "No differences to compare.",
  "semantic.entities.title": "Inferred core entities",
  "semantic.metrics.title": "Candidate metrics",
  "semantic.readOnly": "Read-only",
  "semantic.meta.schemaVersion": "Schema version",
  "semantic.meta.schemaCapturedAt": "Captured at",
  "semantic.meta.schemaHash": "Schema hash",
  "semantic.meta.databaseName": "Database",
  "semantic.meta.tableDescription": "Table description",
  "semantic.meta.columnDescription": "Column description",
  "semantic.meta.originalComment": "Original comment",
  "semantic.meta.dataType": "Data type",
  "semantic.meta.nullable": "Nullable",
  "semantic.status.draft": "Draft",
  "semantic.status.approved": "Approved",
  "semantic.status.archived": "Archived",
  "semantic.diff.added": "Added",
  "semantic.diff.changed": "Changed",
  "semantic.diff.removed": "Removed",
  "semantic.diff.table": "Table",
  "semantic.diff.column": "Column",
  "semantic.diff.before": "Before",
  "semantic.diff.after": "After",
  "semantic.loadErrorFallback": "Failed to load the semantic layer.",
  "semantic.success.draftCreated": "Semantic-layer draft created.",
  "semantic.success.saved": "Saved your edits as a new draft.",
  "semantic.success.approved": "Semantic layer approved.",
  "semantic.meta.approvedBy": "Approved by",
  "semantic.entities.empty": "No entities to display.",
  "semantic.metrics.empty": "No candidate metrics to display.",
  "semantic.layer.none": "No editable layer is available.",
  "semantic.state.pendingDraft": "A latest-schema draft is required.",
  "semantic.state.dirty": "You have unsaved changes.",
  "semantic.meta.columnCount": "Columns",
  "semantic.notice.cacheUsage": "Cache usage",
} as const;

const onboardingEn = {
  "onboarding.common.label": "Onboarding",
  "onboarding.common.loading": "Loading onboarding...",
  "onboarding.common.next": "Next",
  "onboarding.common.back": "Back",
  "onboarding.common.cancel": "Cancel",
  "onboarding.common.copy": "Copy",
  "onboarding.common.copyFix": "Copy fix",
  "onboarding.common.copied": "Copied",
  "onboarding.common.connected": "Connected",
  "onboarding.common.pending": "Pending",
  "onboarding.common.stepOfTotal": "Step {step} of {total}",
  "onboarding.common.justHappenedLabel": "What just happened",
  "onboarding.common.nextLabel": "What comes next",
  "onboarding.workspacePicker.title": "Choose a workspace to resume",
  "onboarding.workspacePicker.loadingTitle": "Checking onboarding status",
  "onboarding.workspacePicker.subtitle":
    "Owner accounts with unfinished setup can resume from the saved step here.",
  "onboarding.workspacePicker.resume": "Resume from step {step}.",
  "onboarding.workspacePicker.updatedAt": "Last saved: {time}",
  "onboarding.workspacePicker.open": "Open workspace",
  "onboarding.waiting.title": "Setup is still in progress",
  "onboarding.waiting.subtitle":
    "A workspace owner is still finishing setup for this workspace.",
  "onboarding.waiting.justHappened":
    "You opened an onboarding route for a workspace that has not finished setup yet.",
  "onboarding.waiting.next":
    "Once the owner finishes setup, chat and analytics screens will open normally.",
  "onboarding.waiting.body":
    "{workspace} is currently being configured. Please wait for the workspace owner to finish the remaining setup steps.",
  "onboarding.waiting.updatedAt": "Last update: {time}",
  "onboarding.step1.title": "Welcome and workspace naming",
  "onboarding.step1.subtitle":
    "Confirm that Korean is the primary language and choose the workspace name users will see.",
  "onboarding.step1.justHappened":
    "Your sign-in was verified and we created a resumable onboarding record for this workspace.",
  "onboarding.step1.next":
    "After this, you will install the edge agent with a ready-to-run Docker command.",
  "onboarding.step1.workspaceName": "Workspace name",
  "onboarding.step1.workspacePlaceholder": "Ecotech quality workspace",
  "onboarding.step1.languageConfirm":
    "I confirm that Korean is the primary language for this workspace.",
  "onboarding.step1.summaryTitle": "Why we ask this now",
  "onboarding.step1.summaryBody":
    "This name is reused in setup commands, invite screens, and the chat workspace label. You can still rename it later in product settings.",
  "onboarding.step2.title": "Install the edge agent",
  "onboarding.step2.subtitle":
    "Run the prepared Docker command on the customer environment. The control plane will detect the connection automatically.",
  "onboarding.step2.justHappened":
    "We issued an onboarding-scoped agent token for this workspace and embedded it into the install command.",
  "onboarding.step2.next":
    "As soon as the outbound stream opens, onboarding moves to database setup automatically.",
  "onboarding.step2.commandTitle": "Copy this Docker command",
  "onboarding.step2.commandBody":
    "Paste this command exactly into the server where the agent should run. It already includes the control-plane URL, pinned agent version, and the persistent /etc/agent and /var/lib/agent mounts.",
  "onboarding.step2.statusTitle": "Live connection status",
  "onboarding.step2.statusWaiting":
    "We are polling every 5 seconds for the agent session opened by this onboarding token.",
  "onboarding.step2.statusConnected":
    "The agent connected successfully. The next step is ready.",
  "onboarding.step2.connectedAt": "Connected at {time}",
  "onboarding.step2.troubleshootingTitle": "Troubleshooting",
  "onboarding.step2.troubleshoot.1.title": "Outbound 443 is blocked",
  "onboarding.step2.troubleshoot.1.body":
    "If the server cannot reach the control plane over HTTPS, the agent never opens its outbound stream.",
  "onboarding.step2.troubleshoot.1.fix":
    "curl -I https://your-control-plane.example.com/healthz",
  "onboarding.step2.troubleshoot.2.title": "Wrong tenant token",
  "onboarding.step2.troubleshoot.2.body":
    "If the token was edited or partially copied, the control plane will reject the agent stream.",
  "onboarding.step2.troubleshoot.2.fix":
    "docker rm -f <workspace>-agent && paste the exact command again from onboarding",
  "onboarding.step2.troubleshoot.3.title": "Docker is missing or the daemon is stopped",
  "onboarding.step2.troubleshoot.3.body":
    "The command requires a working Docker CLI and daemon on the target machine.",
  "onboarding.step2.troubleshoot.3.fix":
    "docker version && sudo systemctl start docker",
  "onboarding.step2.troubleshoot.4.title": "Clock skew or time sync drift",
  "onboarding.step2.troubleshoot.4.body":
    "Large clock drift can break mTLS and short-lived token validation in production.",
  "onboarding.step2.troubleshoot.4.fix":
    "timedatectl set-ntp true && timedatectl status",
  "onboarding.step2.troubleshoot.5.title": "Registry pull authentication failed",
  "onboarding.step2.troubleshoot.5.body":
    "If the target host cannot pull the image from the registry, the container never starts.",
  "onboarding.step2.troubleshoot.5.fix":
    "docker login registry.digitalocean.com",
  "onboarding.step3.title": "Connect the database safely",
  "onboarding.step3.subtitle":
    "Create a dedicated read-only MySQL user, then send the connection string through the control plane to the connected agent.",
  "onboarding.step3.justHappened":
    "The agent is online, so it can receive a database configuration command over the existing outbound stream.",
  "onboarding.step3.next":
    "Once the connection test passes, onboarding unlocks schema introspection.",
  "onboarding.step3.host": "MySQL host",
  "onboarding.step3.port": "Port",
  "onboarding.step3.database": "Database name",
  "onboarding.step3.hostPlaceholder": "db.example.local",
  "onboarding.step3.databasePlaceholder": "mission_app",
  "onboarding.step3.generateSql": "Generate SQL",
  "onboarding.step3.sqlTitle": "Run this SQL as MySQL root",
  "onboarding.step3.sqlBody":
    "Paste the exact SQL below into MySQL to create a dedicated read-only user for Mission.",
  "onboarding.step3.whyReadOnly":
    "Read-only access reduces risk. The agent only needs SELECT privileges to answer questions and capture schema metadata.",
  "onboarding.step3.whySeparateUser":
    "A separate account keeps application credentials isolated and makes future auditing easier.",
  "onboarding.step3.whyReplica":
    "If a read replica is available, using it avoids putting reporting load on the primary database.",
  "onboarding.step3.connectionString": "Connection string to send to the agent",
  "onboarding.step3.connectionPlaceholder":
    "username:password@tcp(host:3306)/database?parseTime=true",
  "onboarding.step3.verifiedAt": "Verified at {time}",
  "onboarding.step3.verify": "Verify and continue",
  "onboarding.step3.transport":
    "This browser sends the connection string to the control plane, and the control plane forwards it to the connected agent over the existing outbound command stream.",
  "onboarding.step4.title": "Run schema introspection",
  "onboarding.step4.subtitle":
    "Capture the current schema through the connected agent and review the detected table summary.",
  "onboarding.step4.justHappened":
    "The database connection was verified with SELECT 1 and privilege validation.",
  "onboarding.step4.next":
    "The captured schema becomes the input for the draft semantic layer in the next step.",
  "onboarding.step4.body":
    "This can take up to about a minute depending on schema size. If it fails, you can retry here.",
  "onboarding.step4.run": "Run introspection",
  "onboarding.step4.running": "Running introspection...",
  "onboarding.step4.tables": "Tables found",
  "onboarding.step4.columns": "Columns found",
  "onboarding.step4.foreignKeys": "Foreign keys found",
  "onboarding.step5.title": "Review the semantic layer",
  "onboarding.step5.subtitle":
    "Generate or refine the Korean semantic draft, then approve the permanent version when it looks right.",
  "onboarding.step5.justHappened":
    "We captured the latest schema and made it available to the semantic-layer workflow.",
  "onboarding.step5.next":
    "Once approved, back navigation is removed and onboarding moves to the final starter screen.",
  "onboarding.step6.title": "You are ready to ask questions",
  "onboarding.step6.subtitle":
    "The core setup is complete. You can open chat now or continue to the final onboarding summary.",
  "onboarding.step6.justHappened":
    "The semantic layer was approved, so the NL-to-SQL flow now has a durable business context.",
  "onboarding.step6.next":
    "The final step records completion and optionally saves teammate invite emails for follow-up.",
  "onboarding.step6.readyTitle": "Setup is ready",
  "onboarding.step6.readyBody":
    "You can go straight to chat and ask a first question now, or open the completion summary before inviting teammates.",
  "onboarding.step6.chatCta": "Go to chat",
  "onboarding.step6.finish": "Continue to summary",
  "onboarding.step7.title": "Workspace setup summary",
  "onboarding.step7.subtitle":
    "Review the final workspace status, record teammate invites, and mark onboarding complete.",
  "onboarding.step7.justHappened":
    "All required setup steps finished successfully for this workspace.",
  "onboarding.step7.next":
    "After completion, owners are no longer redirected into onboarding at sign-in.",
  "onboarding.step7.workspace": "Workspace",
  "onboarding.step7.agent": "Agent",
  "onboarding.step7.database": "Database",
  "onboarding.step7.semanticLayer": "Semantic layer version",
  "onboarding.step7.chatCta": "Open chat",
  "onboarding.step7.inviteCta": "Invite teammates",
  "onboarding.step7.complete": "Mark onboarding complete",
  "onboarding.step7.completed": "Onboarding is marked complete.",
  "onboarding.step7.invitesTitle": "Saved teammate invites",
  "onboarding.step7.invitesEmpty": "No invite emails have been saved yet.",
  "onboarding.step7.inviteModalTitle": "Save teammate emails",
  "onboarding.step7.inviteModalBody":
    "Enter comma-separated or line-separated emails. This step only records them; it does not send messages yet.",
  "onboarding.step7.invitePlaceholder":
    "alex@example.com\nops@example.com",
  "onboarding.step7.saveInvites": "Save emails",
  "onboarding.error.workspacePicker":
    "Failed to load your onboarding workspaces. Please refresh and try again.",
  "onboarding.error.stateLoad":
    "Failed to load onboarding state. Please refresh and try again.",
  "onboarding.error.welcome":
    "We could not save the welcome step. Check the workspace name and try again.",
  "onboarding.error.install":
    "We could not prepare the install bundle. Please try again.",
  "onboarding.error.status":
    "We could not refresh the agent status. Polling will continue automatically.",
  "onboarding.error.database":
    "We could not verify the database configuration. Check the values and try again.",
  "onboarding.error.schema":
    "Schema introspection failed. Please retry after checking the database connection.",
  "onboarding.error.semantic":
    "The semantic-layer step failed. Please retry after the current draft finishes loading.",
  "onboarding.error.invites":
    "We could not save teammate emails. Check the addresses and try again.",
  "onboarding.error.complete":
    "We could not mark onboarding complete. Please try again.",
  "onboarding.error.permission":
    "You do not have permission to edit onboarding for this workspace.",
  "onboarding.error.invalidStep":
    "This onboarding step is not available yet.",
  "onboarding.error.stepLocked":
    "Earlier steps are locked after semantic approval.",
  "onboarding.error.workspaceNameRequired": "Workspace name is required.",
  "onboarding.error.primaryLanguage":
    "Please confirm Korean as the primary language to continue.",
  "onboarding.error.inviteEmail":
    "One or more invite emails are not valid.",
  "onboarding.error.semanticNotApproved":
    "Approve the semantic layer before moving forward.",
  "onboarding.error.generic":
    "Something went wrong during onboarding. Please try again.",
} as const;

const en = {
  ...commonEn,
  ...layoutEn,
  ...tenantsEn,
  ...agentsEn,
  ...queryDebugEn,
  ...chatEn,
  ...starterQuestionsEn,
  ...semanticEn,
  ...onboardingEn,
} as const;

export type TranslationKey = keyof typeof en;

const commonKo: Record<keyof typeof commonEn, string> = {
  "common.appLabel": "Mission",
  "common.language": "언어",
  "common.loading": "불러오는 중",
  "common.na": "해당 없음",
  "common.unknown": "알 수 없음",
  "common.online": "온라인",
  "common.offline": "오프라인",
  "common.yes": "예",
  "common.no": "아니오",
};

const layoutKo: Record<keyof typeof layoutEn, string> = {
  "layout.nav.tenants": "테넌트",
  "layout.nav.questions": "질문하기",
  "layout.nav.semanticLayer": "시맨틱 레이어",
  "layout.nav.agents": "에이전트",
  "layout.theme.label": "테마",
  "layout.theme.system": "시스템",
  "layout.theme.light": "라이트",
  "layout.theme.dark": "다크",
};

const tenantsKo: Record<keyof typeof tenantsEn, string> = {
  "tenants.hero.title": "테넌트 및 에이전트 토큰",
  "tenants.hero.subtitle":
    "테넌트를 만들고, 범위가 제한된 토큰을 발급한 뒤, 그것으로 에지 에이전트를 부팅합니다. 평문 토큰은 한 번만 표시되므로 즉시 복사하거나 폐기 후 다시 발급해야 합니다.",
  "tenants.list.title": "테넌트",
  "tenants.list.subtitle": "내가 속한 작업 공간입니다.",
  "tenants.list.empty": "아직 테넌트가 없습니다. 아래에서 생성하세요.",
  "tenants.form.slug": "슬러그",
  "tenants.form.name": "이름",
  "tenants.form.slugPlaceholder": "ecotech-demo",
  "tenants.form.namePlaceholder": "에코텍 데모 테넌트",
  "tenants.form.create": "테넌트 생성",
  "tenants.form.creating": "생성 중...",
  "tenants.detail.selectPrompt": "테넌트를 선택하세요",
  "tenants.detail.subtitle": "이 테넌트에 속한 에이전트 토큰입니다.",
  "tenants.tokens.copyNow":
    "이 토큰은 지금 복사해야 합니다. 다시 표시되지 않습니다.",
  "tenants.tokens.copy": "복사",
  "tenants.tokens.dismiss": "닫기",
  "tenants.tokens.empty": "이 테넌트에 발급된 토큰이 아직 없습니다.",
  "tenants.tokens.id": "id",
  "tenants.tokens.issued": "발급",
  "tenants.tokens.lastUsed": "마지막 사용",
  "tenants.tokens.revokedAt": "폐기",
  "tenants.tokens.revoke": "폐기",
  "tenants.tokens.revoking": "폐기 중...",
  "tenants.tokens.revoked": "폐기됨",
  "tenants.tokens.label": "라벨",
  "tenants.tokens.labelPlaceholder": "edge-01",
  "tenants.tokens.issue": "토큰 발급",
  "tenants.tokens.issuing": "발급 중...",
  "tenants.tokens.pickTenant": "토큰을 관리할 테넌트를 선택하세요.",
};

const agentsKo: Record<keyof typeof agentsEn, string> = {
  "agents.hero.title": "2.1주차 에이전트 터널 디버그 화면",
  "agents.hero.subtitle":
    "5초마다 컨트롤 플레인을 폴링해 실시간 에이전트 연결 상태를 확인하고, 수동 ping으로 아웃바운드 터널 전체 경로를 검증합니다.",
  "agents.health.api": "API",
  "agents.health.database": "Postgres",
  "agents.section.title": "에이전트 세션",
  "agents.section.subtitle":
    "테넌트 토큰마다 런타임 범위의 행이 하나씩 표시됩니다. 오프라인 행은 새 세션으로 대체될 때까지 남아 있습니다.",
  "agents.section.empty": "연결된 에지 에이전트가 아직 없습니다.",
  "agents.meta.host": "호스트",
  "agents.meta.version": "버전",
  "agents.meta.tokenLabel": "토큰 라벨",
  "agents.meta.tenant": "테넌트",
  "agents.meta.connected": "연결됨",
  "agents.meta.lastHeartbeat": "마지막 하트비트",
  "agents.meta.disconnected": "연결 종료",
  "agents.meta.lastPing": "마지막 ping",
  "agents.meta.lastPingValue": "{time} 기준 {ms}ms",
  "agents.button.ping": "에이전트 ping",
  "agents.button.pinging": "ping 중...",
  "agents.count.one": "세션 {count}개",
  "agents.count.other": "세션 {count}개",
};

const queryDebugKo: Record<keyof typeof queryDebugEn, string> = {
  "queryDebug.hero.title": "읽기 전용 MySQL 디버그 실행기",
  "queryDebug.hero.subtitle":
    "에지 에이전트 쿼리 경로를 끝까지 검증하기 위한 오너 전용 도구입니다. SQL을 살아 있는 테넌트 에이전트로 보내 로컬 MySQL 연결에서 실행한 뒤 JSON으로 보여줍니다.",
  "queryDebug.tenants.title": "테넌트",
  "queryDebug.tenants.subtitle": "오너 범위의 작업 공간을 선택하세요.",
  "queryDebug.tenants.empty": "사용 가능한 테넌트가 아직 없습니다.",
  "queryDebug.detail.selectPrompt": "테넌트를 선택하세요",
  "queryDebug.detail.subtitle":
    "상태는 이 테넌트의 가장 최근 에이전트 세션을 기준으로 합니다.",
  "queryDebug.meta.host": "호스트",
  "queryDebug.meta.session": "세션",
  "queryDebug.meta.version": "버전",
  "queryDebug.meta.tokenLabel": "토큰 라벨",
  "queryDebug.form.sql": "SQL",
  "queryDebug.form.sqlPlaceholder": "SELECT 1",
  "queryDebug.form.run": "쿼리 실행",
  "queryDebug.form.running": "실행 중...",
  "queryDebug.form.available": "쿼리 실행이 가능합니다.",
  "queryDebug.form.unavailable":
    "SQL을 실행하려면 에지 에이전트를 온라인 상태로 올리세요.",
  "queryDebug.result.database": "데이터베이스",
  "queryDebug.result.user": "사용자",
  "queryDebug.result.elapsed": "실행 시간",
  "queryDebug.result.columns": "컬럼",
};

const chatKo: Record<keyof typeof chatEn, string> = {
  "chat.hero.title": "질문하기",
  "chat.hero.subtitle":
    "자연어 질문을 읽기 전용 SQL로 바꾸고, 안전하게 실행한 뒤, 생성된 SQL과 재시도 기록까지 함께 보여줍니다.",
  "chat.tenants.title": "테넌트 선택",
  "chat.tenants.subtitle": "질문을 보낼 작업 공간을 고르세요.",
  "chat.tenants.empty": "사용 가능한 테넌트가 없습니다.",
  "chat.tenants.guardrail":
    "SELECT, WITH, SHOW만 허용됩니다. 위험한 구문은 차단되고, 필요하면 LIMIT 1000이 자동 적용됩니다.",
  "chat.form.title": "질문 작성",
  "chat.form.subtitle.selected":
    "{tenant}에 대해 자연어로 질문하세요.",
  "chat.form.subtitle.unselected": "먼저 테넌트를 선택하세요.",
  "chat.form.label": "질문",
  "chat.form.defaultQuestion":
    "지난 30일 동안 측정소별 평균 pH를 보여줘",
  "chat.form.placeholder":
    "예: 지난 분기 공정별 평균 수질 점수와 가장 문제가 많은 측정소를 보여줘",
  "chat.form.help":
    "결과에는 생성된 SQL, 실제 실행 SQL, 시도 기록이 함께 표시됩니다.",
  "chat.form.submit": "질문 보내기",
  "chat.form.submitting": "질문 처리 중...",
  "chat.history.title": "최근 질문",
  "chat.history.subtitle":
    "같은 브라우저 세션에서 보낸 질문과 응답을 임시로 보여줍니다.",
  "chat.history.empty":
    "첫 질문을 보내면 요약, 생성된 SQL, 결과 테이블이 여기에 나타납니다.",
  "chat.history.count.one": "{count}개",
  "chat.history.count.other": "{count}개",
  "chat.card.question": "질문",
  "chat.card.success": "완료",
  "chat.card.error": "실패",
  "chat.result.summaryTitle": "요약 (백엔드 응답)",
  "chat.result.rowCount": "행 수",
  "chat.result.elapsed": "실행 시간",
  "chat.result.safetyLimit": "안전 제한",
  "chat.result.limitInjected": "LIMIT 1000 자동 적용",
  "chat.result.noLimit": "추가 제한 없음",
  "chat.result.dataTitle": "결과 데이터",
  "chat.result.noRows": "조건에 맞는 결과가 없습니다.",
  "chat.result.sqlAttemptsTitle": "생성된 SQL과 시도 기록",
  "chat.result.originalSql": "원본 SQL",
  "chat.result.executedSql": "실행 SQL",
  "chat.result.attempts": "시도 기록",
  "chat.attempt.success": "성공",
  "chat.attempt.failure": "실패",
  "chat.attempt.successHelp": "검증과 실행을 통과한 SQL입니다.",
  "chat.attempt.title": "시도 {index} · {stage}",
  "chat.stage.generation": "생성",
  "chat.stage.validation": "검증",
  "chat.stage.execution": "실행",
  "chat.feedback.title": "이 결과를 학습시키기",
  "chat.feedback.subtitle":
    "짧은 리뷰를 남기면 같은 테넌트의 다음 답변 품질을 높일 수 있습니다.",
  "chat.feedback.ratingHelpful": "도움 됨",
  "chat.feedback.ratingNeedsWork": "개선 필요",
  "chat.feedback.commentLabel": "코멘트",
  "chat.feedback.commentPlaceholder":
    "무엇이 좋았고 다음에는 무엇이 달라져야 하는지 적어 주세요.",
  "chat.feedback.correctedSqlLabel": "수정된 SQL (선택)",
  "chat.feedback.correctedSqlPlaceholder":
    "시스템이 원래 생성했어야 하는 SQL을 붙여 넣으세요.",
  "chat.feedback.submit": "리뷰 저장",
  "chat.feedback.submitting": "리뷰 저장 중...",
  "chat.feedback.success": "이 쿼리 실행에 대한 리뷰를 저장했습니다.",
  "chat.examples.title": "승인된 예시",
  "chat.examples.subtitle":
    "비슷한 질문에 재사용할 수 있도록 소유자가 승인한 테넌트 전용 예시입니다.",
  "chat.examples.loading": "승인된 예시를 불러오는 중...",
  "chat.examples.empty": "승인된 예시가 아직 없습니다.",
  "chat.examples.sqlPreview": "SQL 보기",
  "chat.examples.archive": "보관",
  "chat.examples.archiving": "보관 중...",
  "chat.examples.createTitle": "승인된 예시로 저장",
  "chat.examples.createSubtitle":
    "소유자는 이 실행 결과를 재사용 가능한 테넌트 메모리로 저장할 수 있습니다.",
  "chat.examples.questionLabel": "표준 질문",
  "chat.examples.sqlLabel": "표준 SQL",
  "chat.examples.sqlPlaceholder": "SELECT ...",
  "chat.examples.notesLabel": "노트",
  "chat.examples.notesPlaceholder":
    "언제 이 예시를 안전하게 재사용할 수 있는지 적어 주세요.",
  "chat.examples.create": "예시 저장",
  "chat.examples.creating": "예시 저장 중...",
  "chat.examples.createSuccess": "새 승인 예시를 저장했습니다.",
};

const starterQuestionsKo: Record<keyof typeof starterQuestionsEn, string> = {
  "starterQuestions.title": "바로 실행해 볼 질문",
  "starterQuestions.subtitle":
    "승인된 시맨틱 레이어를 바탕으로 실제 테이블을 쓰는 질문만 골랐습니다. 하나를 눌러 바로 결과를 확인하세요.",
  "starterQuestions.regenerate": "다시 추천받기",
  "starterQuestions.loading":
    "이 작업 공간에 맞는 시작 질문을 만드는 중입니다...",
  "starterQuestions.empty": "지금은 추천 질문이 없습니다.",
  "starterQuestions.error": "추천 질문을 불러오지 못했습니다.",
};

const semanticKo: Record<keyof typeof semanticEn, string> = {
  "semantic.hero.title": "시맨틱 레이어",
  "semantic.hero.subtitle":
    "최신 스키마 버전을 기준으로 한국어 설명 초안을 만들고, 검토 후 저장 및 승인합니다.",
  "semantic.tenants.title": "테넌트",
  "semantic.tenants.subtitle": "작업할 테넌트를 선택하세요.",
  "semantic.tenants.empty": "사용 가능한 테넌트가 없습니다.",
  "semantic.loading": "불러오는 중...",
  "semantic.schemaNotCaptured.title": "스키마가 아직 없습니다",
  "semantic.schemaNotCaptured.body":
    "먼저 스키마 인트로스펙션을 실행해 최신 tenant_schemas 버전을 만들어 주세요.",
  "semantic.draftNeeded.title": "초안이 아직 없습니다",
  "semantic.draftNeeded.body":
    "최신 스키마 버전에 대한 시맨틱 레이어 초안을 생성한 뒤 검토를 시작할 수 있습니다.",
  "semantic.actions.generateDraft": "초안 생성",
  "semantic.actions.save": "저장",
  "semantic.actions.approve": "승인",
  "semantic.actions.saving": "저장 중...",
  "semantic.actions.approving": "승인 중...",
  "semantic.actions.generating": "생성 중...",
  "semantic.currentLayer.title": "현재 편집 중인 레이어",
  "semantic.approvedBaseline.title": "승인 기준선 비교",
  "semantic.diff.none": "비교할 변경 사항이 없습니다.",
  "semantic.entities.title": "추론된 핵심 엔터티",
  "semantic.metrics.title": "후보 지표",
  "semantic.readOnly": "읽기 전용",
  "semantic.meta.schemaVersion": "스키마 버전",
  "semantic.meta.schemaCapturedAt": "캡처 시각",
  "semantic.meta.schemaHash": "스키마 해시",
  "semantic.meta.databaseName": "데이터베이스",
  "semantic.meta.tableDescription": "테이블 설명",
  "semantic.meta.columnDescription": "컬럼 설명",
  "semantic.meta.originalComment": "원본 코멘트",
  "semantic.meta.dataType": "데이터 타입",
  "semantic.meta.nullable": "NULL 허용",
  "semantic.status.draft": "초안",
  "semantic.status.approved": "승인됨",
  "semantic.status.archived": "보관됨",
  "semantic.diff.added": "추가",
  "semantic.diff.changed": "변경",
  "semantic.diff.removed": "삭제",
  "semantic.diff.table": "테이블",
  "semantic.diff.column": "컬럼",
  "semantic.diff.before": "이전",
  "semantic.diff.after": "현재",
  "semantic.loadErrorFallback": "시맨틱 레이어를 불러오지 못했습니다.",
  "semantic.success.draftCreated":
    "시맨틱 레이어 초안을 생성했습니다.",
  "semantic.success.saved":
    "수정 내용을 새 초안으로 저장했습니다.",
  "semantic.success.approved": "시맨틱 레이어를 승인했습니다.",
  "semantic.meta.approvedBy": "승인 사용자",
  "semantic.entities.empty": "표시할 엔터티가 없습니다.",
  "semantic.metrics.empty": "표시할 후보 지표가 없습니다.",
  "semantic.layer.none": "현재 편집 가능한 레이어가 없습니다.",
  "semantic.state.pendingDraft": "최신 스키마 초안이 필요합니다.",
  "semantic.state.dirty": "저장되지 않은 변경 사항이 있습니다.",
  "semantic.meta.columnCount": "컬럼 수",
  "semantic.notice.cacheUsage": "캐시 사용량",
};

const onboardingKo: Record<keyof typeof onboardingEn, string> = {
  "onboarding.common.label": "온보딩",
  "onboarding.common.loading": "온보딩 정보를 불러오는 중입니다...",
  "onboarding.common.next": "다음",
  "onboarding.common.back": "이전 단계",
  "onboarding.common.cancel": "취소",
  "onboarding.common.copy": "복사",
  "onboarding.common.copyFix": "해결 명령 복사",
  "onboarding.common.copied": "복사했습니다",
  "onboarding.common.connected": "연결됨",
  "onboarding.common.pending": "대기 중",
  "onboarding.common.stepOfTotal": "{total}단계 중 {step}단계",
  "onboarding.common.justHappenedLabel": "방금 한 일",
  "onboarding.common.nextLabel": "다음 단계",
  "onboarding.workspacePicker.title": "이어서 진행할 작업 공간을 선택하세요",
  "onboarding.workspacePicker.loadingTitle": "온보딩 상태를 확인하는 중입니다",
  "onboarding.workspacePicker.subtitle":
    "설치가 끝나지 않은 오너 작업 공간은 여기서 저장된 단계부터 다시 시작할 수 있습니다.",
  "onboarding.workspacePicker.resume": "{step}단계부터 이어서 진행합니다.",
  "onboarding.workspacePicker.updatedAt": "마지막 저장: {time}",
  "onboarding.workspacePicker.open": "작업 공간 열기",
  "onboarding.waiting.title": "설정이 아직 진행 중입니다",
  "onboarding.waiting.subtitle":
    "이 작업 공간의 오너가 아직 초기 설정을 마치지 않았습니다.",
  "onboarding.waiting.justHappened":
    "아직 설정이 완료되지 않은 작업 공간의 온보딩 화면에 들어왔습니다.",
  "onboarding.waiting.next":
    "오너가 설정을 완료하면 채팅과 분석 화면을 정상적으로 사용할 수 있습니다.",
  "onboarding.waiting.body":
    "{workspace} 작업 공간은 현재 설정 중입니다. 작업 공간 오너가 남은 단계를 끝낼 때까지 기다려 주세요.",
  "onboarding.waiting.updatedAt": "마지막 업데이트: {time}",
  "onboarding.step1.title": "환영합니다. 작업 공간 이름을 정하세요",
  "onboarding.step1.subtitle":
    "이 작업 공간의 기본 언어가 한국어인지 확인하고, 사용자가 보게 될 작업 공간 이름을 정합니다.",
  "onboarding.step1.justHappened":
    "로그인이 확인되었고, 이 작업 공간에 대해 이어서 진행할 수 있는 온보딩 상태를 만들었습니다.",
  "onboarding.step1.next":
    "다음 단계에서는 바로 실행할 수 있는 Docker 명령으로 에지 에이전트를 설치합니다.",
  "onboarding.step1.workspaceName": "작업 공간 이름",
  "onboarding.step1.workspacePlaceholder": "에코텍 품질 작업 공간",
  "onboarding.step1.languageConfirm":
    "이 작업 공간의 기본 언어가 한국어임을 확인합니다.",
  "onboarding.step1.summaryTitle": "지금 이 정보를 받는 이유",
  "onboarding.step1.summaryBody":
    "이 이름은 설치 명령, 초대 화면, 채팅 작업 공간 이름에 함께 사용됩니다. 나중에 설정에서 다시 바꿀 수 있습니다.",
  "onboarding.step2.title": "에지 에이전트를 설치하세요",
  "onboarding.step2.subtitle":
    "고객 환경에서 준비된 Docker 명령을 실행하세요. 연결이 열리면 컨트롤 플레인이 자동으로 감지합니다.",
  "onboarding.step2.justHappened":
    "이 작업 공간용 온보딩 전용 에이전트 토큰을 발급했고, 설치 명령에 이미 넣어 두었습니다.",
  "onboarding.step2.next":
    "아웃바운드 스트림이 열리는 즉시 데이터베이스 설정 단계로 자동 이동합니다.",
  "onboarding.step2.commandTitle": "이 Docker 명령을 복사하세요",
  "onboarding.step2.commandBody":
    "에이전트를 실행할 서버에서 이 명령을 그대로 붙여 넣으세요. 컨트롤 플레인 URL, 고정된 에이전트 버전, 그리고 /etc/agent 및 /var/lib/agent 영구 마운트가 이미 포함되어 있습니다.",
  "onboarding.step2.statusTitle": "실시간 연결 상태",
  "onboarding.step2.statusWaiting":
    "이 온보딩 토큰으로 열린 에이전트 세션을 5초마다 확인하고 있습니다.",
  "onboarding.step2.statusConnected":
    "에이전트 연결이 확인되었습니다. 다음 단계로 진행할 준비가 됐습니다.",
  "onboarding.step2.connectedAt": "{time}에 연결됨",
  "onboarding.step2.troubleshootingTitle": "문제 해결",
  "onboarding.step2.troubleshoot.1.title": "아웃바운드 443 또는 방화벽 차단",
  "onboarding.step2.troubleshoot.1.body":
    "서버가 HTTPS로 컨트롤 플레인에 나갈 수 없으면 에이전트가 아웃바운드 스트림을 열지 못합니다.",
  "onboarding.step2.troubleshoot.1.fix":
    "curl -I https://your-control-plane.example.com/healthz",
  "onboarding.step2.troubleshoot.2.title": "잘못된 테넌트 토큰 사용",
  "onboarding.step2.troubleshoot.2.body":
    "토큰이 중간에 잘렸거나 수정되면 컨트롤 플레인이 에이전트 연결을 거부합니다.",
  "onboarding.step2.troubleshoot.2.fix":
    "docker rm -f <workspace>-agent && 온보딩 화면의 명령을 다시 정확히 붙여 넣으세요",
  "onboarding.step2.troubleshoot.3.title": "Docker 미설치 또는 데몬 중지",
  "onboarding.step2.troubleshoot.3.body":
    "이 명령은 동작 중인 Docker CLI와 데몬이 있는 서버에서만 실행할 수 있습니다.",
  "onboarding.step2.troubleshoot.3.fix":
    "docker version && sudo systemctl start docker",
  "onboarding.step2.troubleshoot.4.title": "시계 오차 또는 시간 동기화 문제",
  "onboarding.step2.troubleshoot.4.body":
    "서버 시간이 크게 어긋나면 운영 환경에서 mTLS나 짧은 수명의 토큰 검증이 실패할 수 있습니다.",
  "onboarding.step2.troubleshoot.4.fix":
    "timedatectl set-ntp true && timedatectl status",
  "onboarding.step2.troubleshoot.5.title": "레지스트리 이미지 pull 인증 실패",
  "onboarding.step2.troubleshoot.5.body":
    "대상 서버가 레지스트리에서 이미지를 내려받지 못하면 컨테이너가 시작되지 않습니다.",
  "onboarding.step2.troubleshoot.5.fix":
    "docker login registry.digitalocean.com",
  "onboarding.step3.title": "안전하게 데이터베이스를 연결하세요",
  "onboarding.step3.subtitle":
    "전용 읽기 전용 MySQL 사용자를 만든 뒤, 연결 문자열을 컨트롤 플레인을 통해 연결된 에이전트로 보냅니다.",
  "onboarding.step3.justHappened":
    "에이전트가 온라인 상태이므로 기존 아웃바운드 스트림을 통해 데이터베이스 설정 명령을 받을 수 있습니다.",
  "onboarding.step3.next":
    "연결 검증이 끝나면 다음 단계에서 스키마 인트로스펙션을 실행할 수 있습니다.",
  "onboarding.step3.host": "MySQL 호스트",
  "onboarding.step3.port": "포트",
  "onboarding.step3.database": "데이터베이스 이름",
  "onboarding.step3.hostPlaceholder": "db.example.local",
  "onboarding.step3.databasePlaceholder": "mission_app",
  "onboarding.step3.generateSql": "SQL 생성",
  "onboarding.step3.sqlTitle": "MySQL root로 이 SQL을 실행하세요",
  "onboarding.step3.sqlBody":
    "아래 SQL을 그대로 붙여 넣어 Mission 전용 읽기 전용 사용자를 생성하세요.",
  "onboarding.step3.whyReadOnly":
    "읽기 전용 계정은 위험을 줄입니다. 에이전트는 질문 응답과 스키마 메타데이터 수집에 SELECT 권한만 필요합니다.",
  "onboarding.step3.whySeparateUser":
    "애플리케이션 계정과 분리된 전용 사용자를 쓰면 권한을 분명히 관리하고 나중에 감사하기도 쉬워집니다.",
  "onboarding.step3.whyReplica":
    "읽기 복제본이 있다면 우선 사용하세요. 운영 원본 DB에 분석 부하가 실리지 않게 할 수 있습니다.",
  "onboarding.step3.connectionString": "에이전트로 보낼 연결 문자열",
  "onboarding.step3.connectionPlaceholder":
    "username:password@tcp(host:3306)/database?parseTime=true",
  "onboarding.step3.verifiedAt": "{time}에 검증 완료",
  "onboarding.step3.verify": "검증 후 계속",
  "onboarding.step3.transport":
    "이 브라우저는 연결 문자열을 컨트롤 플레인으로 보내고, 컨트롤 플레인은 기존 아웃바운드 명령 스트림을 통해 연결된 에이전트에 전달합니다.",
  "onboarding.step4.title": "스키마 인트로스펙션 실행",
  "onboarding.step4.subtitle":
    "연결된 에이전트를 통해 현재 스키마를 수집하고, 감지된 테이블 요약을 확인합니다.",
  "onboarding.step4.justHappened":
    "SELECT 1 실행과 권한 검증으로 데이터베이스 연결을 확인했습니다.",
  "onboarding.step4.next":
    "수집한 스키마는 다음 단계에서 시맨틱 레이어 초안의 입력으로 사용됩니다.",
  "onboarding.step4.body":
    "스키마 크기에 따라 1분 정도 걸릴 수 있습니다. 실패하면 이 화면에서 다시 시도할 수 있습니다.",
  "onboarding.step4.run": "인트로스펙션 실행",
  "onboarding.step4.running": "인트로스펙션 실행 중...",
  "onboarding.step4.tables": "찾은 테이블 수",
  "onboarding.step4.columns": "찾은 컬럼 수",
  "onboarding.step4.foreignKeys": "찾은 외래 키 수",
  "onboarding.step5.title": "시맨틱 레이어를 검토하세요",
  "onboarding.step5.subtitle":
    "한국어 시맨틱 초안을 생성하거나 다듬은 뒤, 괜찮다면 영구 버전으로 승인하세요.",
  "onboarding.step5.justHappened":
    "최신 스키마를 수집했고, 시맨틱 레이어 워크플로에서 바로 사용할 수 있게 되었습니다.",
  "onboarding.step5.next":
    "이 단계를 승인하면 이전 단계로 돌아갈 수 없고, 마지막 시작 화면으로 이동합니다.",
  "onboarding.step6.title": "이제 질문할 준비가 되었습니다",
  "onboarding.step6.subtitle":
    "핵심 설정이 끝났습니다. 지금 바로 채팅을 열거나 마지막 요약 화면으로 이동할 수 있습니다.",
  "onboarding.step6.justHappened":
    "시맨틱 레이어가 승인되어 자연어 질의 흐름이 영구적인 비즈니스 맥락을 사용할 수 있게 되었습니다.",
  "onboarding.step6.next":
    "마지막 단계에서는 완료 상태를 기록하고 팀원 이메일을 저장할 수 있습니다.",
  "onboarding.step6.readyTitle": "설정이 완료되었습니다",
  "onboarding.step6.readyBody":
    "지금 바로 채팅으로 이동해 첫 질문을 보내거나, 완료 요약을 본 뒤 팀원 초대 이메일을 저장할 수 있습니다.",
  "onboarding.step6.chatCta": "채팅으로 이동",
  "onboarding.step6.finish": "요약으로 계속",
  "onboarding.step7.title": "작업 공간 설정 요약",
  "onboarding.step7.subtitle":
    "최종 상태를 확인하고, 팀원 이메일을 저장한 뒤, 온보딩 완료를 기록하세요.",
  "onboarding.step7.justHappened":
    "이 작업 공간에 필요한 모든 설정 단계가 정상적으로 끝났습니다.",
  "onboarding.step7.next":
    "완료를 기록하면 오너는 다음 로그인부터 온보딩으로 강제 이동되지 않습니다.",
  "onboarding.step7.workspace": "작업 공간",
  "onboarding.step7.agent": "에이전트",
  "onboarding.step7.database": "데이터베이스",
  "onboarding.step7.semanticLayer": "시맨틱 레이어 버전",
  "onboarding.step7.chatCta": "채팅 열기",
  "onboarding.step7.inviteCta": "팀원 초대",
  "onboarding.step7.complete": "온보딩 완료 기록",
  "onboarding.step7.completed": "온보딩 완료를 기록했습니다.",
  "onboarding.step7.invitesTitle": "저장된 팀원 초대",
  "onboarding.step7.invitesEmpty": "저장된 초대 이메일이 아직 없습니다.",
  "onboarding.step7.inviteModalTitle": "팀원 이메일 저장",
  "onboarding.step7.inviteModalBody":
    "쉼표나 줄바꿈으로 구분해 이메일을 입력하세요. 이 단계에서는 발송하지 않고 기록만 저장합니다.",
  "onboarding.step7.invitePlaceholder":
    "alex@example.com\nops@example.com",
  "onboarding.step7.saveInvites": "이메일 저장",
  "onboarding.error.workspacePicker":
    "온보딩 작업 공간 목록을 불러오지 못했습니다. 새로고침 후 다시 시도해 주세요.",
  "onboarding.error.stateLoad":
    "온보딩 상태를 불러오지 못했습니다. 새로고침 후 다시 시도해 주세요.",
  "onboarding.error.welcome":
    "환영 단계 저장에 실패했습니다. 작업 공간 이름을 확인한 뒤 다시 시도해 주세요.",
  "onboarding.error.install":
    "설치 명령을 준비하지 못했습니다. 잠시 후 다시 시도해 주세요.",
  "onboarding.error.status":
    "에이전트 상태를 새로고침하지 못했습니다. 폴링은 계속 시도됩니다.",
  "onboarding.error.database":
    "데이터베이스 설정을 검증하지 못했습니다. 입력값을 확인한 뒤 다시 시도해 주세요.",
  "onboarding.error.schema":
    "스키마 인트로스펙션에 실패했습니다. DB 연결 상태를 확인한 뒤 다시 시도해 주세요.",
  "onboarding.error.semantic":
    "시맨틱 레이어 단계 처리에 실패했습니다. 현재 초안 상태를 확인한 뒤 다시 시도해 주세요.",
  "onboarding.error.invites":
    "팀원 이메일을 저장하지 못했습니다. 주소를 확인한 뒤 다시 시도해 주세요.",
  "onboarding.error.complete":
    "온보딩 완료를 기록하지 못했습니다. 다시 시도해 주세요.",
  "onboarding.error.permission":
    "이 작업 공간의 온보딩을 수정할 권한이 없습니다.",
  "onboarding.error.invalidStep":
    "아직 열 수 없는 온보딩 단계입니다.",
  "onboarding.error.stepLocked":
    "시맨틱 레이어 승인 이후에는 이전 단계로 돌아갈 수 없습니다.",
  "onboarding.error.workspaceNameRequired": "작업 공간 이름을 입력해 주세요.",
  "onboarding.error.primaryLanguage":
    "계속하려면 기본 언어가 한국어임을 확인해 주세요.",
  "onboarding.error.inviteEmail":
    "초대 이메일 형식이 올바르지 않은 항목이 있습니다.",
  "onboarding.error.semanticNotApproved":
    "다음 단계로 가기 전에 시맨틱 레이어를 승인해 주세요.",
  "onboarding.error.generic":
    "온보딩 처리 중 문제가 발생했습니다. 다시 시도해 주세요.",
};

const ko: Record<TranslationKey, string> = {
  ...commonKo,
  ...layoutKo,
  ...tenantsKo,
  ...agentsKo,
  ...queryDebugKo,
  ...chatKo,
  ...starterQuestionsKo,
  ...semanticKo,
  ...onboardingKo,
};

const dictionaries: Record<Locale, Record<TranslationKey, string>> = {
  en,
  ko,
};

type I18nContextValue = {
  formatDateTime: (
    value: Date | number,
    options?: Intl.DateTimeFormatOptions,
  ) => string;
  formatNumber: (
    value: bigint | number,
    options?: Intl.NumberFormatOptions,
  ) => string;
  locale: Locale;
  localeTag: string;
  setLocale: (next: Locale) => void;
  t: (key: TranslationKey, params?: TranslationParams) => string;
};

const I18nContext = createContext<I18nContextValue | null>(null);

function interpolate(
  template: string,
  params?: TranslationParams,
): string {
  if (!params) {
    return template;
  }
  return template.replaceAll(/\{(\w+)\}/g, (match, key: string) => {
    const value = params[key];
    return value === undefined ? match : String(value);
  });
}

function readStoredLocale(initialLocale?: Locale): Locale {
  if (initialLocale) {
    return initialLocale;
  }
  if (typeof window === "undefined") {
    return defaultLocale;
  }
  const stored = window.localStorage.getItem(localeStorageKey);
  if (stored === "en" || stored === "ko") {
    return stored;
  }
  return defaultLocale;
}

function toLocaleTag(locale: Locale): string {
  return locale === "ko" ? "ko-KR" : "en-US";
}

export function I18nProvider({
  children,
  initialLocale,
}: PropsWithChildren<{ initialLocale?: Locale }>) {
  const [locale, setLocale] = useState<Locale>(() =>
    readStoredLocale(initialLocale),
  );

  useEffect(() => {
    if (typeof window === "undefined" || initialLocale) {
      return;
    }
    window.localStorage.setItem(localeStorageKey, locale);
  }, [initialLocale, locale]);

  const value = useMemo<I18nContextValue>(() => {
    const localeTag = toLocaleTag(locale);
    return {
      formatDateTime: (
        input: Date | number,
        options: Intl.DateTimeFormatOptions = {
          dateStyle: "medium",
          timeStyle: "short",
        },
      ) =>
        new Intl.DateTimeFormat(localeTag, options).format(input),
      formatNumber: (
        input: bigint | number,
        options?: Intl.NumberFormatOptions,
      ) => new Intl.NumberFormat(localeTag, options).format(input),
      locale,
      localeTag,
      setLocale,
      t: (key, params) => interpolate(dictionaries[locale][key], params),
    };
  }, [locale]);

  return createElement(I18nContext.Provider, { value }, children);
}

export function useI18n(): I18nContextValue {
  const value = useContext(I18nContext);
  if (!value) {
    throw new Error("useI18n must be used inside an I18nProvider");
  }
  return value;
}
