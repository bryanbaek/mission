package controller

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/bryanbaek/mission/internal/controlplane/model"
)

func validateStarterQuestions(
	questions []starterQuestionCandidate,
	layer model.SemanticLayerContent,
) error {
	if len(layer.Tables) == 0 {
		return fmt.Errorf("semantic layer must contain at least one table")
	}
	if len(questions) != 10 {
		return fmt.Errorf("expected exactly 10 questions, got %d", len(questions))
	}

	validTables := make(map[string]struct{}, len(layer.Tables))
	for _, table := range layer.Tables {
		name := strings.TrimSpace(table.TableName)
		if name == "" {
			continue
		}
		validTables[name] = struct{}{}
	}
	if len(validTables) == 0 {
		return fmt.Errorf("semantic layer must contain at least one named table")
	}

	distinctCategories := make(map[string]struct{})
	distinctTables := make(map[string]struct{})
	allowedCategories := make(map[string]struct{}, len(starterQuestionCategoryValues()))
	for _, category := range starterQuestionCategoryValues() {
		allowedCategories[category] = struct{}{}
	}

	for index, question := range questions {
		text := strings.TrimSpace(question.Text)
		category := strings.TrimSpace(question.Category)
		primaryTable := strings.TrimSpace(question.PrimaryTable)

		if !utf8.ValidString(text) {
			return fmt.Errorf("question %d text must be valid UTF-8", index+1)
		}
		if text == "" {
			return fmt.Errorf("question %d text is required", index+1)
		}
		if utf8.RuneCountInString(text) > 80 {
			return fmt.Errorf("question %d text exceeds 80 characters", index+1)
		}
		if _, ok := allowedCategories[category]; !ok {
			return fmt.Errorf("question %d category %q is invalid", index+1, category)
		}
		if _, ok := validTables[primaryTable]; !ok {
			return fmt.Errorf("question %d primary_table %q is not in the semantic layer", index+1, primaryTable)
		}

		distinctCategories[category] = struct{}{}
		distinctTables[primaryTable] = struct{}{}
	}

	if len(distinctCategories) < 3 {
		return fmt.Errorf("expected at least 3 distinct categories, got %d", len(distinctCategories))
	}

	minTables := 5
	if len(validTables) < minTables {
		minTables = len(validTables)
	}
	if len(distinctTables) < minTables {
		return fmt.Errorf(
			"expected at least %d distinct primary tables, got %d",
			minTables,
			len(distinctTables),
		)
	}

	return nil
}
