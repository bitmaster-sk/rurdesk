package notifytest

import (
	"sync"

	"github.com/bitmaster-sk/rurdesk/api/internal/notify"
)

var _ notify.INotifier = (*Recorder)(nil)

type Recorder struct {
	mu      sync.Mutex
	notices []*notify.Notice
}

func (r *Recorder) Send(notice *notify.Notice) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.notices = append(r.notices, notice)
}

func (r *Recorder) Notices() []*notify.Notice {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]*notify.Notice(nil), r.notices...)
}

func (r *Recorder) Drain() []*notify.Notice {
	r.mu.Lock()
	defer r.mu.Unlock()
	drained := r.notices
	r.notices = nil
	return drained
}
