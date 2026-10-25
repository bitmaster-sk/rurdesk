package mcp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func captureJSON(t *testing.T, c *gin.Context) map[string]any {
	t.Helper()
	raw, err := io.ReadAll(c.Request.Body)
	require.NoError(t, err)
	captured := map[string]any{}
	require.NoError(t, json.Unmarshal(raw, &captured))
	return captured
}

func TestListTags_CallsTagEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/api/private/tag", func(c *gin.Context) {
		c.String(http.StatusOK, `[{"idTag":3,"name":"next"}]`)
	})

	result, err := handleListTags(context.Background(), toolRequest("list_tags", map[string]any{}), NewDispatcher(engine))

	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, resultText(t, result), `"name":"next"`)
}

func TestCreateTag_SendsNameAndColor(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	var captured map[string]any
	engine.POST("/api/private/tag", func(c *gin.Context) {
		captured = captureJSON(t, c)
		c.String(http.StatusOK, `{"idTag":3}`)
	})

	_, err := handleCreateTag(context.Background(), toolRequest("create_tag", map[string]any{
		"project_id": float64(1),
		"name":       "next",
		"color":      "#16a34a",
	}), NewDispatcher(engine))

	require.NoError(t, err)
	assert.Equal(t, float64(1), captured["idProject"])
	assert.Equal(t, "next", captured["name"])
	assert.Equal(t, "#16a34a", captured["color"])
}

func TestUpdateTag_KeepsTheFieldsItDoesNotChange(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/api/private/tag", func(c *gin.Context) {
		c.String(http.StatusOK, `[{"idTag":3,"idProject":1,"name":"next","color":"#16a34a","orderRank":2}]`)
	})
	var captured map[string]any
	engine.PATCH("/api/private/tag/:idTag", func(c *gin.Context) {
		captured = captureJSON(t, c)
		c.String(http.StatusOK, `{"idTag":3}`)
	})

	result, err := handleUpdateTag(context.Background(), toolRequest("update_tag", map[string]any{
		"project_id": float64(1),
		"tag_id":     float64(3),
		"name":       "v1.6.0",
	}), NewDispatcher(engine))

	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Equal(t, "v1.6.0", captured["name"])
	assert.Equal(t, "#16a34a", captured["color"])
	assert.Equal(t, float64(2), captured["orderRank"])
}

func TestUpdateTag_UnknownTagIsAnError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/api/private/tag", func(c *gin.Context) {
		c.String(http.StatusOK, `[]`)
	})

	result, err := handleUpdateTag(context.Background(), toolRequest("update_tag", map[string]any{
		"project_id": float64(1),
		"tag_id":     float64(3),
		"name":       "v1.6.0",
	}), NewDispatcher(engine))

	require.NoError(t, err)
	assert.True(t, result.IsError)
}

func TestListIssues_ForwardsTagFilter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	var query map[string][]string
	engine.GET("/api/private/project/:id/issue", func(c *gin.Context) {
		query = c.Request.URL.Query()
		c.String(http.StatusOK, `{"items":[]}`)
	})

	_, err := handleListIssues(context.Background(), toolRequest("list_issues", map[string]any{
		"project_id": float64(1),
		"tag_ids":    []any{float64(3), float64(4)},
		"tag_unset":  true,
	}), NewDispatcher(engine))

	require.NoError(t, err)
	assert.Equal(t, []string{"3", "4"}, query["idsTag"])
	assert.Equal(t, []string{"true"}, query["tagUnset"])
}

func TestUpdateIssue_EmptyTagListClearsTags(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	var captured map[string]any
	engine.PATCH("/api/private/project/:id/issue/:issueId", func(c *gin.Context) {
		captured = captureJSON(t, c)
		c.String(http.StatusOK, `{"idIssue":5}`)
	})

	_, err := handleUpdateIssue(context.Background(), toolRequest("update_issue", map[string]any{
		"project_id": float64(1),
		"issue_id":   float64(99),
		"tag_ids":    []any{},
	}), NewDispatcher(engine))

	require.NoError(t, err)
	tags, ok := captured["idsTag"].([]any)
	require.True(t, ok, "an empty tag_ids must be sent as an empty idsTag, not dropped")
	assert.Empty(t, tags)
}

func TestUpdateIssue_OmittedTagsAreNotSent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	var captured map[string]any
	engine.PATCH("/api/private/project/:id/issue/:issueId", func(c *gin.Context) {
		captured = captureJSON(t, c)
		c.String(http.StatusOK, `{"idIssue":5}`)
	})

	_, err := handleUpdateIssue(context.Background(), toolRequest("update_issue", map[string]any{
		"project_id": float64(1),
		"issue_id":   float64(99),
		"title":      "renamed",
	}), NewDispatcher(engine))

	require.NoError(t, err)
	assert.NotContains(t, captured, "idsTag")
}

func TestBulkUpdateIssues_ForwardsTagAddAndRemove(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	var captured map[string]any
	engine.PATCH("/api/private/project/:id/issue/batch", func(c *gin.Context) {
		captured = captureJSON(t, c)
		c.String(http.StatusOK, `[]`)
	})

	_, err := handleBulkUpdateIssues(context.Background(), toolRequest("bulk_update_issues", map[string]any{
		"project_id":     float64(1),
		"issue_ids":      []any{float64(10), float64(11)},
		"add_tag_ids":    []any{float64(3)},
		"remove_tag_ids": []any{float64(4)},
	}), NewDispatcher(engine))

	require.NoError(t, err)
	issues, ok := captured["issues"].([]any)
	require.True(t, ok)
	require.Len(t, issues, 2)
	for _, raw := range issues {
		entry := raw.(map[string]any)
		assert.Equal(t, []any{float64(3)}, entry["addIdsTag"])
		assert.Equal(t, []any{float64(4)}, entry["removeIdsTag"])
	}
}
