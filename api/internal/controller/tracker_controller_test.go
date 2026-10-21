package controller

import (
	"testing"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/stretchr/testify/assert"
)

func Test_resolveTrackTimes(t *testing.T) {
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)

	t.Run("tracked with startAt anchors the entry on the sent day", func(t *testing.T) {
		tracked := int64(7200)
		startAt := now.AddDate(0, 0, -2)
		tr := &model.Track{Tracked: &tracked, StartAt: &startAt}
		resolveTrackTimes(tr, now)
		assert.True(t, tr.StartAt.Equal(startAt))
		assert.True(t, tr.EndAt.Equal(startAt.Add(2*time.Hour)))
	})

	t.Run("tracked without startAt keeps ending at now", func(t *testing.T) {
		tracked := int64(3600)
		tr := &model.Track{Tracked: &tracked}
		resolveTrackTimes(tr, now)
		assert.True(t, tr.EndAt.Equal(now))
		assert.True(t, tr.StartAt.Equal(now.Add(-time.Hour)))
	})

	t.Run("tracked with endAt and without startAt subtracts from endAt", func(t *testing.T) {
		tracked := int64(1800)
		endAt := now.Add(-time.Hour)
		tr := &model.Track{Tracked: &tracked, EndAt: &endAt}
		resolveTrackTimes(tr, now)
		assert.True(t, tr.EndAt.Equal(endAt))
		assert.True(t, tr.StartAt.Equal(endAt.Add(-30*time.Minute)))
	})

	t.Run("startAt without tracked derives tracked from the span", func(t *testing.T) {
		startAt := now.Add(-90 * time.Minute)
		tr := &model.Track{StartAt: &startAt}
		resolveTrackTimes(tr, now)
		assert.True(t, tr.EndAt.Equal(now))
		assert.Equal(t, int64(5400), *tr.Tracked)
	})
}

func Test_validateTrackTimes_rejectsFutureStartAt(t *testing.T) {
	tracked := int64(3600)
	future := time.Now().UTC().Add(time.Hour)
	past := time.Now().UTC().Add(-time.Hour)

	assert.Error(t, validateTrackTimes(&tracked, &future, nil))
	assert.Error(t, validateTrackTimes(nil, &future, nil))
	assert.NoError(t, validateTrackTimes(&tracked, &past, nil))
	assert.NoError(t, validateTrackTimes(&tracked, nil, nil))
	assert.NoError(t, validateTrackTimes(nil, &past, nil))
}
