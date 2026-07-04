package pkg

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReconNotReadySessionsWarning_NoLabel(t *testing.T) {
	for _, lang := range []string{LangEN, LangVI} {
		ctx := WithLanguage(context.Background(), lang)

		w := ReconNotReadySessionsWarning(ctx, 7, "  ", "Alice")
		assert.NotContains(t, w, `label ""`)
		assert.NotContains(t, w, `nhãn ""`)
		assert.NotContains(t, w, "label")
		assert.NotContains(t, w, "nhãn")
		assert.Contains(t, w, "#7")
		assert.Contains(t, w, "Alice")

		labeled := ReconNotReadySessionsWarning(ctx, 7, "shelf", "Alice")
		assert.Contains(t, labeled, `"shelf"`)
		assert.Contains(t, labeled, "Alice")
		if lang == LangEN {
			assert.True(t, strings.Contains(labeled, "label"))
		} else {
			assert.True(t, strings.Contains(labeled, "nhãn"))
		}
	}
}
