package common

import (
	"context"
	"fmt"
	"maps"

	"github.com/rs/zerolog/log"
)

// A model the adapter could not ask about goes to unanswered and is asked again; a model it cannot tell about is left out of both.
type ModelThinkingProber interface {
	ModelThinking(ctx context.Context, models []Model) (isThinkingByModel map[string]bool, unanswered []Model)
}

type Adapter interface {
	Agent
	ModelThinkingProber
}

// Only the models the adapter answered for are reported, so a model it could not be asked about keeps its stored state.
type ModelThinkingReport struct {
	tracker           *TrackerClient
	prober            ModelThinkingProber
	pending           []Model
	isThinkingByModel map[string]bool
	isSent            bool
}

func NewModelThinkingReport(models []Model, prober ModelThinkingProber, tracker *TrackerClient) *ModelThinkingReport {
	return &ModelThinkingReport{tracker: tracker, prober: prober, pending: models, isThinkingByModel: map[string]bool{}, isSent: true}
}

func (r *ModelThinkingReport) Run(ctx context.Context) {
	if !RetryWithBackoff(ctx, "model thinking", r.Attempt) && ctx.Err() == nil {
		log.Warn().Msg("model thinking report gave up; the unanswered models stay unknown")
	}
}

func (r *ModelThinkingReport) Attempt(ctx context.Context) error {
	answered, unanswered := r.prober.ModelThinking(ctx, r.pending)
	r.pending = unanswered
	if len(answered) > 0 {
		maps.Copy(r.isThinkingByModel, answered)
		r.isSent = false
	}
	if !r.isSent {
		if err := r.tracker.ReportModelThinking(ctx, r.isThinkingByModel); err != nil {
			return err
		}
		r.isSent = true
	}
	if len(r.pending) > 0 {
		return fmt.Errorf("%d models did not answer", len(r.pending))
	}
	return nil
}
