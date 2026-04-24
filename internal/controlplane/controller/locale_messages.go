package controller

import (
	"fmt"

	"github.com/bryanbaek/mission/internal/controlplane/model"
	"github.com/bryanbaek/mission/internal/sqlguard"
)

// ---------------------------------------------------------------------------
// Database configuration error messages (onboarding flow)
// ---------------------------------------------------------------------------

func databaseErrorMessage(code AgentConfigureDatabaseErrorCode, locale model.Locale) string {
	if locale == model.LocaleEnglish {
		return databaseErrorMessageEN(code)
	}
	return databaseErrorMessageKO(code)
}

func databaseErrorMessageEN(code AgentConfigureDatabaseErrorCode) string {
	switch code {
	case AgentConfigureDatabaseErrorCodeInvalidDSN:
		return "The connection string format is invalid. Please double-check the value you pasted."
	case AgentConfigureDatabaseErrorCodeConnectFailed:
		return "Could not connect to the MySQL server. Please check the host, port, firewall, and network access."
	case AgentConfigureDatabaseErrorCodeAuthFailed:
		return "MySQL authentication failed. Please verify the username and password you created."
	case AgentConfigureDatabaseErrorCodePrivilegeError:
		return "Read-only privilege check failed. Please ensure only SELECT privileges are granted."
	case AgentConfigureDatabaseErrorCodeWriteConfig:
		return "Failed to save the agent configuration file. Please check the Docker volume path and write permissions."
	case AgentConfigureDatabaseErrorCodeTimeout:
		return "The database verification timed out. Please check the network and server responsiveness."
	default:
		return "Database connection verification failed. Please review your inputs and MySQL permissions."
	}
}

// ---------------------------------------------------------------------------
// Query pipeline warnings
// ---------------------------------------------------------------------------

func warnSummaryFailed(locale model.Locale, err error) string {
	if locale == model.LocaleEnglish {
		return fmt.Sprintf("Summary generation failed: %v", err)
	}
	return fmt.Sprintf("요약 생성에 실패했습니다: %v", err)
}

func warnLimitInjected(locale model.Locale) string {
	if locale == model.LocaleEnglish {
		return fmt.Sprintf("A LIMIT of %d was automatically applied for safety.", sqlguard.DefaultRowLimit)
	}
	return fmt.Sprintf("안전을 위해 LIMIT %d을(를) 자동 적용했습니다.", sqlguard.DefaultRowLimit)
}

// ---------------------------------------------------------------------------
// Query preparation warnings
// ---------------------------------------------------------------------------

func warnUsedDraftLayer(locale model.Locale) string {
	if locale == model.LocaleEnglish {
		return "No approved semantic layer found; the draft layer was used instead."
	}
	return "승인된 시맨틱 레이어가 없어 초안(draft) 레이어를 사용했습니다."
}

func warnUsedRawSchema(locale model.Locale) string {
	if locale == model.LocaleEnglish {
		return "No semantic layer found; SQL was generated from the raw schema only. Accuracy may be lower."
	}
	return "시맨틱 레이어가 없어 원본 스키마만으로 SQL을 생성했습니다. 정확도가 낮을 수 있습니다."
}
