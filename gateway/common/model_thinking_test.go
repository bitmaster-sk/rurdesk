package common

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

type fakeThinkingProber struct {
	isThinkingByName map[string]bool
}

func (p *fakeThinkingProber) ModelThinking(_ context.Context, models []Model) (map[string]bool, []Model) {
	isThinkingByModel := map[string]bool{}
	unanswered := []Model{}
	for _, model := range models {
		if isThinking, ok := p.isThinkingByName[model.Name]; ok {
			isThinkingByModel[model.Id()] = isThinking
		} else {
			unanswered = append(unanswered, model)
		}
	}
	return isThinkingByModel, unanswered
}

func TestModelThinkingReportRetriesOnlyTheUnansweredModels(t *testing.T) {
	var reports []map[string]bool
	tracker := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var body struct {
			IsThinkingByModel map[string]bool `json:"isThinkingByModel"`
		}
		_ = json.NewDecoder(request.Body).Decode(&body)
		reports = append(reports, body.IsThinkingByModel)
		writer.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(tracker.Close)
	prober := &fakeThinkingProber{isThinkingByName: map[string]bool{"llama3.3": false}}
	models := []Model{{Provider: "ollama", Name: "llama3.3"}, {Provider: "ollama", Name: "qwen3:8b"}}
	report := NewModelThinkingReport(models, prober, NewTrackerClient(&Config{TrackerAPIUrl: tracker.URL, WebhookSecret: []byte("secret")}))

	if err := report.Attempt(context.Background()); err == nil {
		t.Fatal("the first attempt must fail while qwen3:8b is unanswered")
	}
	prober.isThinkingByName["qwen3:8b"] = true
	if err := report.Attempt(context.Background()); err != nil {
		t.Fatalf("second attempt: %v", err)
	}
	if err := report.Attempt(context.Background()); err != nil {
		t.Fatalf("third attempt: %v", err)
	}

	want := []map[string]bool{
		{"ollama/llama3.3": false},
		{"ollama/llama3.3": false, "ollama/qwen3:8b": true},
	}
	if !reflect.DeepEqual(reports, want) {
		t.Errorf("reports = %v, want %v", reports, want)
	}
}

func TestModelThinkingReportSendsNothingWhenNoModelIsKnown(t *testing.T) {
	tracker := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
		t.Errorf("unexpected report to %s", request.URL.Path)
	}))
	t.Cleanup(tracker.Close)
	prober := &fakeThinkingProber{}
	report := NewModelThinkingReport(nil, prober, NewTrackerClient(&Config{TrackerAPIUrl: tracker.URL, WebhookSecret: []byte("secret")}))

	if err := report.Attempt(context.Background()); err != nil {
		t.Errorf("Attempt: %v", err)
	}
}
