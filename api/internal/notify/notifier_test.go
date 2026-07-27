package notify

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// addConn registers a fake connection (no websocket) for idUser with a buffered
// send channel so broadcast() can deliver without blocking.
func addConn(n *Notifier, idUser int64) *NotifyConnection {
	conn := &NotifyConnection{idUser: idUser, send: make(chan []byte, 1)}
	if n.conns[idUser] == nil {
		n.conns[idUser] = make(map[*NotifyConnection]bool)
	}
	n.conns[idUser][conn] = true
	return conn
}

// A notice carrying IdsUser must reach only the listed users — BroadcastIssueUpdate
// relies on this to keep issue payloads inside the project.
func TestBroadcast_IdsUserDeliversOnlyToListedUsers(t *testing.T) {
	n := &Notifier{conns: make(map[int64]map[*NotifyConnection]bool)}
	c1 := addConn(n, 1)
	c2 := addConn(n, 2)
	c3 := addConn(n, 3)

	n.broadcast(&Notice{
		IdsUser: []int64{1, 2},
		Subject: SubjectIssue,
		Action:  ActionUpdate,
	})

	assert.Len(t, c1.send, 1, "user 1 (listed) should receive")
	assert.Len(t, c2.send, 1, "user 2 (listed) should receive")
	assert.Empty(t, c3.send, "user 3 (not listed) must NOT receive")
}

// A targeted broadcast must never send on a channel Remove already closed — the
// old bare `conn.send <- b` panicked on a race-window close and killed the
// notifier goroutine for every user. trySend's closed-guard absorbs it.
func TestBroadcast_ClosedConnectionDoesNotPanic(t *testing.T) {
	n := &Notifier{conns: make(map[int64]map[*NotifyConnection]bool)}

	// A connection in the state Remove leaves behind: closed flag set and send
	// channel closed, but still referenced by a target snapshot.
	closedConn := &NotifyConnection{idUser: 1, send: make(chan []byte, 1), closed: true}
	close(closedConn.send)
	n.conns[1] = map[*NotifyConnection]bool{closedConn: true}

	assert.NotPanics(t, func() {
		n.broadcast(&Notice{IdsUser: []int64{1}, Subject: SubjectIssue, Action: ActionUpdate})
	})
}

// A slow client's full send buffer must not stall delivery for everyone else.
// The old IdsUser branch blocked on a full buffer; trySend drops instead.
func TestBroadcast_FullBufferDropsWithoutBlocking(t *testing.T) {
	n := &Notifier{conns: make(map[int64]map[*NotifyConnection]bool)}
	conn := addConn(n, 1) // send buffer capacity 1
	conn.send <- []byte("prefill")

	done := make(chan struct{})
	go func() {
		n.broadcast(&Notice{IdsUser: []int64{1}, Subject: SubjectIssue, Action: ActionUpdate})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("broadcast blocked on a full send buffer instead of dropping")
	}
	assert.Len(t, conn.send, 1, "the second message must be dropped, buffer stays at 1")
}

// Broadcasting concurrently with Add/Remove must not race on n.conns. The old
// IdsUser branch read the map without the lock — "concurrent map iteration and
// map write". Requires -race to catch a regression.
func TestBroadcast_ConcurrentAddRemoveNoRace(t *testing.T) {
	n := &Notifier{conns: make(map[int64]map[*NotifyConnection]bool)}

	stop := make(chan struct{})
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				conn := &NotifyConnection{idUser: 1, send: make(chan []byte, 8)}
				n.Add(conn)
				n.Remove(conn)
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 5000; i++ {
			n.broadcast(&Notice{IdsUser: []int64{1}, Subject: SubjectIssue, Action: ActionUpdate})
		}
		close(stop)
	}()

	wg.Wait()
}
