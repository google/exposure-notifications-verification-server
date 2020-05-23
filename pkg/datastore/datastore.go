package datastore

import (
	"context"
	"fmt"
	"time"

	gcpdatastore "cloud.google.com/go/datastore"
	"github.com/mikehelmick/tek-verification-server/pkg/database"
)

func init() {
	database.RegisterDatabase("datastore", New)
}

type Datastore struct {
	client *gcpdatastore.Client
}

func New(ctx context.Context) (database.Database, error) {
	client, err := gcpdatastore.NewClient(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("error connecting to datastore: %w", err)
	}
	return &Datastore{client}, nil
}

func (d *Datastore) InsertPIN(pin string, risks []database.TransmissionRisk, addClaims map[string]string, duration time.Duration) (database.IssuedPIN, error) {
	return nil, nil
}

func (d *Datastore) RetrievePIN(pin string) (database.IssuedPIN, error) {
	return nil, nil
}
