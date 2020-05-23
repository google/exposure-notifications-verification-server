package gcpkms

import (
	"context"
	"crypto"

	kms "cloud.google.com/go/kms/apiv1"
	"github.com/mikehelmick/tek-verification-server/pkg/signer"
	"github.com/sethvargo/go-gcpkms/pkg/gcpkms"
)

func init() {
	signer.RegisterKeyManager("gcpkms", New)
}

type GCPKeyManager struct {
	client *kms.KeyManagementClient
}

func New(ctx context.Context) (signer.KeyManager, error) {
	client, err := kms.NewKeyManagementClient(ctx)
	if err != nil {
		return nil, err
	}
	return &GCPKeyManager{client}, nil
}

func (g *GCPKeyManager) NewSigner(ctx context.Context, keyID string) (crypto.Signer, error) {
	signer, err := gcpkms.NewSigner(ctx, g.client, keyID)
	if err != nil {
		return nil, err
	}
	return signer, nil
}
