import type {
  TranslationKey,
} from "./en";

export const ko: Record<TranslationKey, string> = {
  "common.appLabel": "Mission",
  "common.language": "언어",
  "common.loading": "불러오는 중",
  "common.na": "해당 없음",
  "common.unknown": "알 수 없음",
  "common.online": "온라인",
  "common.offline": "오프라인",
  "common.yes": "예",
  "common.no": "아니오",
  "layout.nav.tenants": "테넌트",
  "layout.nav.questions": "질문하기",
  "layout.nav.review": "리뷰함",
  "layout.nav.semanticLayer": "시맨틱 레이어",
  "layout.nav.agents": "에이전트",
  "layout.theme.label": "테마",
  "layout.theme.system": "시스템",
  "layout.theme.light": "라이트",
  "layout.theme.dark": "다크",
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
  "chat.hero.title": "질문하기",
  "chat.hero.subtitle":
    "자연어 질문을 읽기 전용 SQL로 바꾸고, 안전하게 실행한 뒤, 생성된 SQL과 재시도 기록까지 함께 보여줍니다.",
  "chat.tenants.title": "테넌트 선택",
  "chat.tenants.subtitle": "질문을 보낼 작업 공간을 고르세요.",
  "chat.tenants.empty": "사용 가능한 테넌트가 없습니다.",
  "chat.tenants.guardrail":
    "SELECT, WITH, SHOW만 허용됩니다. 위험한 구문은 차단되고, 필요하면 LIMIT 1000이 자동 적용됩니다.",
  "chat.form.title": "질문 작성",
  "chat.form.subtitle.selected": "{tenant}에 대해 자연어로 질문하세요.",
  "chat.form.subtitle.unselected": "먼저 테넌트를 선택하세요.",
  "chat.form.label": "질문",
  "chat.form.defaultQuestion": "지난 30일 동안 측정소별 평균 pH를 보여줘",
  "chat.form.placeholder":
    "예: 지난 분기 공정별 평균 수질 점수와 가장 문제가 많은 측정소를 보여줘",
  "chat.form.help":
    "결과에는 생성된 SQL, 실제 실행 SQL, 시도 기록이 함께 표시됩니다.",
  "chat.form.submit": "질문 보내기",
  "chat.form.submitting": "질문 처리 중...",
  "chat.persistent.title": "내 최근 질문",
  "chat.persistent.subtitle":
    "이 테넌트에 대한 개인용 서버 기록입니다. 메타데이터만 저장되며, 실제 행 결과를 다시 보려면 질문을 재실행하세요.",
  "chat.persistent.loading": "최근 질문 기록을 불러오는 중...",
  "chat.persistent.empty":
    "이 테넌트에 저장된 최근 질문이 아직 없습니다.",
  "chat.persistent.rerun": "다시 실행",
  "chat.persistent.context": "프롬프트 컨텍스트",
  "chat.persistent.metadataOnly": "메타데이터만 저장",
  "chat.persistent.createdAt": "생성 시각",
  "chat.persistent.completedAt": "완료 시각",
  "chat.persistent.failure": "마지막 실패",
  "chat.persistent.detailsTitle": "저장된 SQL과 시도 기록",
  "chat.persistent.status.running": "실행 중",
  "chat.persistent.status.succeeded": "완료",
  "chat.persistent.status.failed": "실패",
  "chat.persistent.source.approved": "승인된 시맨틱 레이어",
  "chat.persistent.source.draft": "초안 시맨틱 레이어",
  "chat.persistent.source.rawSchema": "원본 스키마 대체",
  "chat.history.title": "현재 세션",
  "chat.history.subtitle":
    "이 브라우저 세션에서 받은 실시간 결과를 임시로 보여줍니다.",
  "chat.history.empty":
    "질문을 보내면 현재 세션의 요약, 생성된 SQL, 결과 행이 여기에 나타납니다.",
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
  "review.hero.title": "리뷰함",
  "review.hero.subtitle":
    "실패한 실행과 수정된 답변을 제품 지식으로 전환하는 오너 전용 검토 대기열입니다.",
  "review.tenants.ownerOnly":
    "이 화면에는 오너 권한이 있는 작업 공간만 표시됩니다. 실패를 정리하거나, 표준 예시로 승격하거나, 검토 완료로 닫을 수 있습니다.",
  "review.tenants.emptyOwners":
    "검토할 수 있는 오너 범위 작업 공간이 아직 없습니다.",
  "review.queue.title": "오너 리뷰 대기열",
  "review.queue.subtitle":
    "가장 최근의 피드백 신호부터 보여줍니다. `열린 리뷰`에서는 이미 해결된 실행을 숨깁니다.",
  "review.queue.loading": "리뷰 항목을 불러오는 중...",
  "review.queue.emptyOpen":
    "지금은 미해결 리뷰 항목이 없습니다. 실패한 실행, 낮은 평가, 수정 SQL이 자동으로 여기에 쌓입니다.",
  "review.queue.emptyAllRecent":
    "최근 리뷰 신호가 있는 실행이 아직 없습니다.",
  "review.filters.open": "열린 리뷰",
  "review.filters.allRecent": "최근 전체",
  "review.status.hasFeedback": "피드백 있음",
  "review.status.activeExample": "승인 예시 생성됨",
  "review.status.reviewed": "검토 완료",
  "review.status.needsReview": "오너 검토 필요",
  "review.feedback.title": "최신 피드백",
  "review.feedback.subtitle":
    "가장 최근의 검토 신호를 기준으로 수정, 예시 승격, 혹은 종료 여부를 결정하세요.",
  "review.feedback.empty":
    "이 실행에는 명시적인 피드백이 없습니다. 실행 자체가 실패했기 때문에 대기열에 포함되었습니다.",
  "review.feedback.noComment": "작성된 코멘트가 없습니다.",
  "review.feedback.correctedSql": "수정된 SQL",
  "review.actions.title": "오너 액션",
  "review.actions.subtitle":
    "표준 예시로 저장하거나, 승격 없이 검토 완료로 처리할 수 있습니다.",
  "review.actions.openInChat": "채팅에서 열기",
  "review.actions.markReviewed": "검토 완료 처리",
  "review.actions.markingReviewed": "검토 완료 처리 중...",
  "review.actions.resolvedWithExample":
    "이 실행에서 승인 예시를 저장하여 해결된 항목입니다.",
  "review.actions.resolvedByReview":
    "표준 예시 없이 오너 검토만으로 해결된 항목입니다.",
  "review.examples.create": "승인 예시 저장",
  "review.examples.notesPlaceholder":
    "이 쿼리를 언제 재사용해야 하는지, 혹은 피해야 하는지를 남겨 두세요.",
  "starterQuestions.title": "바로 실행해 볼 질문",
  "starterQuestions.subtitle":
    "승인된 시맨틱 레이어를 바탕으로 실제 테이블을 쓰는 질문만 골랐습니다. 하나를 눌러 바로 결과를 확인하세요.",
  "starterQuestions.regenerate": "다시 추천받기",
  "starterQuestions.loading":
    "이 작업 공간에 맞는 시작 질문을 만드는 중입니다...",
  "starterQuestions.empty": "지금은 추천 질문이 없습니다.",
  "starterQuestions.error": "추천 질문을 불러오지 못했습니다.",
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
  "semantic.success.draftCreated": "시맨틱 레이어 초안을 생성했습니다.",
  "semantic.success.saved": "수정 내용을 새 초안으로 저장했습니다.",
  "semantic.success.approved": "시맨틱 레이어를 승인했습니다.",
  "semantic.meta.approvedBy": "승인 사용자",
  "semantic.entities.empty": "표시할 엔터티가 없습니다.",
  "semantic.metrics.empty": "표시할 후보 지표가 없습니다.",
  "semantic.layer.none": "현재 편집 가능한 레이어가 없습니다.",
  "semantic.state.pendingDraft": "최신 스키마 초안이 필요합니다.",
  "semantic.state.dirty": "저장되지 않은 변경 사항이 있습니다.",
  "semantic.meta.columnCount": "컬럼 수",
  "semantic.notice.cacheUsage": "캐시 사용량",
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
  "onboarding.workspacePicker.title":
    "이어서 진행할 작업 공간을 선택하세요",
  "onboarding.workspacePicker.loadingTitle":
    "온보딩 상태를 확인하는 중입니다",
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
  "onboarding.step2.troubleshoot.1.title":
    "아웃바운드 443 또는 방화벽 차단",
  "onboarding.step2.troubleshoot.1.body":
    "서버가 HTTPS로 컨트롤 플레인에 나갈 수 없으면 에이전트가 아웃바운드 스트림을 열지 못합니다.",
  "onboarding.step2.troubleshoot.1.fix":
    "curl -I https://your-control-plane.example.com/healthz",
  "onboarding.step2.troubleshoot.2.title": "잘못된 테넌트 토큰 사용",
  "onboarding.step2.troubleshoot.2.body":
    "토큰이 중간에 잘렸거나 수정되면 컨트롤 플레인이 에이전트 연결을 거부합니다.",
  "onboarding.step2.troubleshoot.2.fix":
    "docker rm -f <workspace>-agent && 온보딩 화면의 명령을 다시 정확히 붙여 넣으세요",
  "onboarding.step2.troubleshoot.3.title":
    "Docker 미설치 또는 데몬 중지",
  "onboarding.step2.troubleshoot.3.body":
    "이 명령은 동작 중인 Docker CLI와 데몬이 있는 서버에서만 실행할 수 있습니다.",
  "onboarding.step2.troubleshoot.3.fix":
    "docker version && sudo systemctl start docker",
  "onboarding.step2.troubleshoot.4.title":
    "시계 오차 또는 시간 동기화 문제",
  "onboarding.step2.troubleshoot.4.body":
    "서버 시간이 크게 어긋나면 운영 환경에서 mTLS나 짧은 수명의 토큰 검증이 실패할 수 있습니다.",
  "onboarding.step2.troubleshoot.4.fix":
    "timedatectl set-ntp true && timedatectl status",
  "onboarding.step2.troubleshoot.5.title":
    "레지스트리 이미지 pull 인증 실패",
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
  "onboarding.step3.suggestedConnectionString": "추천 연결 문자열",
  "onboarding.step3.suggestedConnectionBody":
    "위에서 만든 읽기 전용 사용자, 비밀번호, 호스트, 포트, 데이터베이스를 합쳐 Mission이 이 DSN을 만들었습니다.",
  "onboarding.step3.connectionPlaceholder":
    "username:password@tcp(host:3306)/database?parseTime=true",
  "onboarding.step3.useSuggestedConnection": "이 연결 문자열 사용",
  "onboarding.step3.connectionRequired":
    "데이터베이스 검증 전에 연결 문자열을 생성하거나 붙여 넣어 주세요.",
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
  "onboarding.step7.invitePlaceholder": "alex@example.com\nops@example.com",
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
