package notify

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

type NoticeAction string
type NoticeSubject string

const (
	ActionCreate NoticeAction = "c"
	ActionDelete NoticeAction = "d"
	ActionUpdate NoticeAction = "u"
)

const (
	SubjectMessage          NoticeSubject = "message"
	SubjectProject          NoticeSubject = "project"
	SubjectIssue            NoticeSubject = "issue"
	SubjectNotification     NoticeSubject = "notification"
	SubjectRelation         NoticeSubject = "relation"
	SubjectAgentRun         NoticeSubject = "agent_run"
	SubjectAgentTask        NoticeSubject = "agent_task"
	SubjectAgentStats       NoticeSubject = "agent_stats"
	SubjectAgentThinking    NoticeSubject = "agent_thinking"
	SubjectIssueParticipant NoticeSubject = "issue_participant"
)

const (
	pongWait   = 60 * time.Second
	pingPeriod = (pongWait * 9) / 10

	sendBufferSize = 256
	mainBufferSize = 1024
)

type Notice struct {
	IdUser  int64         `json:"-"`
	IdsUser []int64       `json:"-"`
	Subject NoticeSubject `json:"subject"`
	Action  NoticeAction  `json:"action"`
	Payload any           `json:"payload"`
	Source  string        `json:"source,omitempty"` // "bot" when author is_bot=true
}

type NotifyConnection struct {
	idUser    int64
	notifier  *Notifier
	ws        *websocket.Conn
	send      chan []byte
	Close     chan bool
	closeOnce sync.Once
	mu        sync.Mutex
	closed    bool
}

type Notifier struct {
	conns map[int64]map[*NotifyConnection]bool
	Send  chan *Notice
	sync.RWMutex
}

func NewNotifyConnection(idUser int64, notifier *Notifier, ws *websocket.Conn) *NotifyConnection {
	c := &NotifyConnection{
		idUser:   idUser,
		notifier: notifier,
		ws:       ws,
		send:     make(chan []byte, sendBufferSize),
		Close:    make(chan bool),
	}
	go c.reader()
	go c.writer()
	return c
}

func (nc *NotifyConnection) reader() {
	defer nc.close()
	nc.ws.SetReadDeadline(time.Now().Add(pongWait))
	nc.ws.SetPongHandler(func(v string) error {
		nc.ws.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})
	for {
		if _, _, err := nc.ws.ReadMessage(); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Error().Err(err).Msg("websocket read error")
			}
			return
		}
	}
}

func (nc *NotifyConnection) writer() {
	ticker := time.NewTicker(pingPeriod)
	writeWait := viper.GetDuration("WEBSOCKET_WRITE_DEADLINE")
	if writeWait == 0 {
		writeWait = 10 * time.Second
	}

	defer nc.close()
	defer ticker.Stop()
	for {
		select {
		case m, ok := <-nc.send:
			if !ok {
				return
			}
			nc.ws.SetWriteDeadline(time.Now().Add(writeWait))
			if err := nc.ws.WriteMessage(websocket.TextMessage, m); err != nil {
				log.Error().Err(err).Msg("send notification error")
				return
			}
		case <-ticker.C:
			nc.ws.SetWriteDeadline(time.Now().Add(writeWait))
			if err := nc.ws.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
				log.Error().Err(err).Msg("send ping error")
				return
			}
		}

	}
}

func (nc *NotifyConnection) close() {
	nc.closeOnce.Do(func() {
		nc.notifier.Remove(nc)
		nc.ws.Close()
		nc.Close <- true
	})
}

// trySend delivers b without blocking the broadcast loop. It holds the connection
// lock and checks closed so it never sends on a channel Remove already closed
// (which would panic and kill the notifier goroutine for everyone), and drops the
// message on a full buffer instead of stalling delivery to other connections.
func (nc *NotifyConnection) trySend(b []byte) {
	nc.mu.Lock()
	defer nc.mu.Unlock()
	if nc.closed {
		return
	}
	select {
	case nc.send <- b:
	default:
		log.Warn().Int64("idUser", nc.idUser).Msg("notification channel full, dropping message")
	}
}

func NewNotifier() *Notifier {
	n := &Notifier{
		conns: make(map[int64]map[*NotifyConnection]bool),
		Send:  make(chan *Notice, mainBufferSize),
	}
	go n.listen()
	return n
}

func (n *Notifier) listen() {
	for {
		n.broadcast(<-n.Send)
	}
}

func (n *Notifier) broadcast(notice *Notice) {
	b, err := json.Marshal(notice)
	if err != nil {
		log.Error().Err(err).Msg("marshall notice error")
		return
	}

	for _, conn := range n.targetsFor(notice) {
		conn.trySend(b)
	}
}

// targetsFor snapshots the connections a notice should reach. It holds the read
// lock for the whole iteration so it never races Add/Remove — an unlocked read
// here would be a "concurrent map iteration and map write" fatal error.
func (n *Notifier) targetsFor(notice *Notice) []*NotifyConnection {
	n.RLock()
	defer n.RUnlock()

	var targets []*NotifyConnection
	switch {
	case len(notice.IdsUser) > 0:
		for _, idUser := range notice.IdsUser {
			for conn := range n.conns[idUser] {
				targets = append(targets, conn)
			}
		}
	case notice.IdUser == 0:
		// Unaddressed notice: fans out to every connected session. UNSAFE for
		// ACL-scoped data — use IdsUser or per-user fan-out instead.
		for _, conns := range n.conns {
			for conn := range conns {
				targets = append(targets, conn)
			}
		}
	default:
		for conn := range n.conns[notice.IdUser] {
			targets = append(targets, conn)
		}
	}
	return targets
}

func (n *Notifier) Remove(nc *NotifyConnection) {
	n.Lock()
	defer n.Unlock()

	if connMap, exist := n.conns[nc.idUser]; exist {
		if len(connMap) <= 1 {
			delete(n.conns, nc.idUser)
		} else {
			delete(n.conns[nc.idUser], nc)
		}
	}

	nc.mu.Lock()
	if !nc.closed {
		nc.closed = true
		close(nc.send)
	}
	nc.mu.Unlock()
}

func (n *Notifier) Add(nc *NotifyConnection) {
	n.Lock()
	defer n.Unlock()

	if _, exist := n.conns[nc.idUser]; exist {
		n.conns[nc.idUser][nc] = true
	} else {
		connMap := make(map[*NotifyConnection]bool)
		connMap[nc] = true
		n.conns[nc.idUser] = connMap
	}
}
