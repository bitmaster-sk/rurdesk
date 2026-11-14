package flow

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/login"
	"github.com/go-redis/redis/v8"
	"golang.org/x/oauth2"
)

const (
	StateTtl = 10 * time.Minute
	CodeTtl  = 60 * time.Second
	TestTtl  = 10 * time.Minute

	statePrefix = "sso:state:"
	codePrefix  = "sso:code:"
	testPrefix  = "sso:test:"
)

var ErrNotFound = errors.New("sso flow entry not found or expired")

type State struct {
	Nonce       string `json:"nonce"`
	Verifier    string `json:"verifier"`
	BindingHash string `json:"bindingHash"`
	ReturnUrl   string `json:"returnUrl"`
	IsExtended  bool   `json:"isExtended"`
	IsTest      bool   `json:"isTest"`
	TestHash    string `json:"testHash"`
	RequestId   string `json:"requestId"`
}

type Code struct {
	IdUser      int64  `json:"idUser"`
	IsExtended  bool   `json:"isExtended"`
	ReturnUrl   string `json:"returnUrl"`
	BindingHash string `json:"bindingHash"`
}

type IStore interface {
	SaveState(ctx context.Context, state State) (string, error)
	TakeState(ctx context.Context, key string) (State, error)
	SaveCode(ctx context.Context, code Code) (string, error)
	TakeCode(ctx context.Context, key string) (Code, error)
	SaveTestResult(ctx context.Context, result login.TestResult) (string, error)
	LoadTestResult(ctx context.Context, id string) (login.TestResult, error)
}

type Store struct {
	cache *redis.Client
}

var _ IStore = (*Store)(nil)

func NewStore(cache *redis.Client) *Store {
	return &Store{cache: cache}
}

func NewState(bindingHash, returnUrl string, isExtended bool) (State, error) {
	nonce, err := RandomToken()
	if err != nil {
		return State{}, err
	}
	return State{
		Nonce:       nonce,
		Verifier:    oauth2.GenerateVerifier(),
		BindingHash: bindingHash,
		ReturnUrl:   SafeReturnUrl(returnUrl),
		IsExtended:  isExtended,
	}, nil
}

func NewTestState(testHash string) (State, error) {
	nonce, err := RandomToken()
	if err != nil {
		return State{}, err
	}
	return State{Nonce: nonce, Verifier: oauth2.GenerateVerifier(), ReturnUrl: "/", IsTest: true, TestHash: testHash}, nil
}

func (s *Store) SaveState(ctx context.Context, state State) (string, error) {
	return save(ctx, s.cache, statePrefix, state, StateTtl)
}

func (s *Store) TakeState(ctx context.Context, key string) (State, error) {
	return decode[State](s.cache.GetDel(ctx, statePrefix+key).Bytes())
}

func (s *Store) SaveCode(ctx context.Context, code Code) (string, error) {
	return save(ctx, s.cache, codePrefix, code, CodeTtl)
}

func (s *Store) TakeCode(ctx context.Context, key string) (Code, error) {
	return decode[Code](s.cache.GetDel(ctx, codePrefix+key).Bytes())
}

func (s *Store) SaveTestResult(ctx context.Context, result login.TestResult) (string, error) {
	return save(ctx, s.cache, testPrefix, result, TestTtl)
}

func (s *Store) LoadTestResult(ctx context.Context, id string) (login.TestResult, error) {
	return decode[login.TestResult](s.cache.Get(ctx, testPrefix+id).Bytes())
}

func RandomToken() (string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("generating random token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

func save(ctx context.Context, cache *redis.Client, prefix string, value any, ttl time.Duration) (string, error) {
	key, err := RandomToken()
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("encoding sso flow entry: %w", err)
	}
	if err := cache.Set(ctx, prefix+key, payload, ttl).Err(); err != nil {
		return "", fmt.Errorf("saving sso flow entry: %w", err)
	}
	return key, nil
}

func decode[T any](payload []byte, err error) (T, error) {
	var value T
	if errors.Is(err, redis.Nil) {
		return value, ErrNotFound
	}
	if err != nil {
		return value, fmt.Errorf("reading sso flow entry: %w", err)
	}
	if err := json.Unmarshal(payload, &value); err != nil {
		return value, fmt.Errorf("decoding sso flow entry: %w", err)
	}
	return value, nil
}
