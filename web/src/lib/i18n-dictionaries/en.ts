export const commonEn = {
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

export const layoutEn = {
  "layout.nav.tenants": "Tenants",
  "layout.nav.questions": "Ask",
  "layout.nav.review": "Review",
  "layout.nav.semanticLayer": "Semantic Layer",
  "layout.nav.agents": "Agents",
  "layout.theme.label": "Theme",
  "layout.theme.system": "System",
  "layout.theme.light": "Light",
  "layout.theme.dark": "Dark",
} as const;

export const tenantsEn = {
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

export const agentsEn = {
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

export const queryDebugEn = {
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

export const chatEn = {
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
  "chat.persistent.title": "My recent queries",
  "chat.persistent.subtitle":
    "Server-backed personal history for this tenant. Stored metadata only; rerun a question to fetch live rows again.",
  "chat.persistent.loading": "Loading recent queries...",
  "chat.persistent.empty":
    "No recent queries have been stored for this tenant yet.",
  "chat.persistent.rerun": "Rerun",
  "chat.persistent.context": "Prompt context",
  "chat.persistent.metadataOnly": "Metadata only",
  "chat.persistent.createdAt": "Created",
  "chat.persistent.completedAt": "Completed",
  "chat.persistent.failure": "Last failure",
  "chat.persistent.detailsTitle": "Stored SQL and attempt history",
  "chat.persistent.status.running": "Running",
  "chat.persistent.status.succeeded": "Completed",
  "chat.persistent.status.failed": "Failed",
  "chat.persistent.source.approved": "Approved semantic layer",
  "chat.persistent.source.draft": "Draft semantic layer",
  "chat.persistent.source.rawSchema": "Raw schema fallback",
  "chat.history.title": "Current session",
  "chat.history.subtitle":
    "Live results from this browser session appear here temporarily.",
  "chat.history.empty":
    "Ask a question to see the live summary, generated SQL, and result rows for this session.",
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
  "review.hero.title": "Review inbox",
  "review.hero.subtitle":
    "Owner-only queue for failed runs and corrected answers that should become reusable product knowledge.",
  "review.tenants.ownerOnly":
    "Only owner-scoped workspaces appear here. Use this queue to resolve failures, save canonical fixes, or clear reviewed items.",
  "review.tenants.emptyOwners":
    "No owner-scoped workspaces are available for review yet.",
  "review.queue.title": "Owner review queue",
  "review.queue.subtitle":
    "Newest feedback signals first. Open review hides runs that are already resolved.",
  "review.queue.loading": "Loading review items...",
  "review.queue.emptyOpen":
    "No unresolved review items right now. Failed runs, down-ratings, and corrected SQL will land here automatically.",
  "review.queue.emptyAllRecent":
    "No recent review-signaled runs yet.",
  "review.filters.open": "Open review",
  "review.filters.allRecent": "All recent",
  "review.status.hasFeedback": "Feedback attached",
  "review.status.activeExample": "Approved example created",
  "review.status.reviewed": "Reviewed",
  "review.status.needsReview": "Needs owner review",
  "review.feedback.title": "Latest feedback",
  "review.feedback.subtitle":
    "Use the most recent reviewer signal to decide whether to fix, promote, or close the run.",
  "review.feedback.empty":
    "No explicit feedback was attached to this run. It is in the queue because the run failed.",
  "review.feedback.noComment": "No written comment was provided.",
  "review.feedback.correctedSql": "Corrected SQL",
  "review.actions.title": "Owner actions",
  "review.actions.subtitle":
    "Either save a canonical example or mark the run reviewed without promotion.",
  "review.actions.openInChat": "Open in chat",
  "review.actions.markReviewed": "Mark reviewed",
  "review.actions.markingReviewed": "Marking reviewed...",
  "review.actions.resolvedWithExample":
    "Resolved by saving an approved example from this run.",
  "review.actions.resolvedByReview":
    "Resolved by an owner review without creating a canonical example.",
  "review.examples.create": "Save approved example",
  "review.examples.notesPlaceholder":
    "What should future owners know about when to reuse or avoid this query?",
} as const;

export const starterQuestionsEn = {
  "starterQuestions.title": "Suggested starter questions",
  "starterQuestions.subtitle":
    "These are tenant-specific questions built from the approved semantic layer. Click one to see the product answer a real question immediately.",
  "starterQuestions.regenerate": "Regenerate",
  "starterQuestions.loading":
    "Generating starter questions for this workspace...",
  "starterQuestions.empty": "No starter questions are available yet.",
  "starterQuestions.error": "Failed to load starter questions.",
} as const;

export const semanticEn = {
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

export const onboardingEn = {
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
  "onboarding.step2.troubleshoot.3.title":
    "Docker is missing or the daemon is stopped",
  "onboarding.step2.troubleshoot.3.body":
    "The command requires a working Docker CLI and daemon on the target machine.",
  "onboarding.step2.troubleshoot.3.fix":
    "docker version && sudo systemctl start docker",
  "onboarding.step2.troubleshoot.4.title":
    "Clock skew or time sync drift",
  "onboarding.step2.troubleshoot.4.body":
    "Large clock drift can break mTLS and short-lived token validation in production.",
  "onboarding.step2.troubleshoot.4.fix":
    "timedatectl set-ntp true && timedatectl status",
  "onboarding.step2.troubleshoot.5.title":
    "Registry pull authentication failed",
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
  "onboarding.step3.suggestedConnectionString": "Suggested connection string",
  "onboarding.step3.suggestedConnectionBody":
    "Mission generated this DSN from the read-only user, password, host, port, and database above.",
  "onboarding.step3.connectionPlaceholder":
    "username:password@tcp(host:3306)/database?parseTime=true",
  "onboarding.step3.useSuggestedConnection": "Use this connection string",
  "onboarding.step3.connectionRequired":
    "Generate or paste a connection string before verifying the database.",
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
  "onboarding.step7.invitePlaceholder": "alex@example.com\nops@example.com",
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

export const en = {
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
export type TranslationDictionary = Record<TranslationKey, string>;
