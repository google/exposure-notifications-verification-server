package database

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type TransmissionRisk struct {
	Risk               int `json:"risk"`
	PastRollingPeriods int `json:"pastRollingPeriods"`
}

type IssuedPIN interface {
	Pin() string
	TransmissionRisks() []TransmissionRisk
	Claims() map[string]string
	EndTime() time.Time
	IsClaimed() bool
}

type NewDBFunc func(context.Context) (Database, error)

type Database interface {
	InsertPIN(pin string, risks []TransmissionRisk, addClaims map[string]string, duration time.Duration) (IssuedPIN, error)
	RetrievePIN(pin string) (IssuedPIN, error)
	MarkPINClaimed(pin string) error
}

var (
	registry = map[string]NewDBFunc{}
	mu       sync.Mutex
)

func RegisterDatabase(name string, f NewDBFunc) error {
	mu.Lock()
	defer mu.Unlock()

	if _, ok := registry[name]; ok {
		return fmt.Errorf("database is already registered: `%v`", name)
	}
	registry[name] = f
	return nil
}

func New(ctx context.Context, name string) (Database, error) {
	mu.Lock()
	defer mu.Unlock()

	if f, ok := registry[name]; ok {
		return f(ctx)
	}
	return nil, fmt.Errorf("database driver not found: `%v`", name)
}

func NewDefault(ctx context.Context) (Database, error) {
	mu.Lock()
	defer mu.Unlock()

	if len(registry) != 1 {
		return nil, fmt.Errorf("cannot use default database, more than one registered")
	}
	for _, v := range registry {
		return v(ctx)
	}
	return nil, fmt.Errorf("you reached unreachable code, congratulations.")
}
