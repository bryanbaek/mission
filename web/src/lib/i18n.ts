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

const en = {
  ...commonEn,
  ...layoutEn,
  ...tenantsEn,
  ...agentsEn,
  ...queryDebugEn,
  ...chatEn,
  ...semanticEn,
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

const ko: Record<TranslationKey, string> = {
  ...commonKo,
  ...layoutKo,
  ...tenantsKo,
  ...agentsKo,
  ...queryDebugKo,
  ...chatKo,
  ...semanticKo,
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
