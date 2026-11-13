package oci

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

const digest = "@sha256:0000000000000000000000000000000000000000000000000000000000000000"

func pullSecret(t *testing.T, users map[string]string) KubeCreds {
	auths := map[string]map[string]string{}
	for key, user := range users {
		auths[key] = map[string]string{"auth": base64.StdEncoding.EncodeToString([]byte(user + ":password"))}
	}

	data, err := json.Marshal(map[string]interface{}{"auths": auths})
	assert.NoError(t, err)
	return KubeCreds{SecretName: "test", SecretCredsData: data}
}

func TestResolveAuthConfigWithPullSecret(t *testing.T) {
	tests := []struct {
		name         string
		imageID      string
		auths        map[string]string
		expectedUser string
		expectedHost string
	}{
		{
			name:         "docker hub default key",
			imageID:      "docker.io/library/alpine" + digest,
			auths:        map[string]string{"https://index.docker.io/v1/": "hub"},
			expectedUser: "hub",
			expectedHost: "index.docker.io",
		},
		{
			name:         "docker hub alias key",
			imageID:      "docker.io/library/busybox" + digest,
			auths:        map[string]string{"docker.io": "hub"},
			expectedUser: "hub",
			expectedHost: "index.docker.io",
		},
		{
			name:         "host key",
			imageID:      "registry.example.com/project/registry/app" + digest,
			auths:        map[string]string{"registry.example.com": "registry", "https://index.docker.io/v1/": "hub"},
			expectedUser: "registry",
			expectedHost: "registry.example.com",
		},
		{
			name:         "path key",
			imageID:      "registry.example.com/project/registry/app" + digest,
			auths:        map[string]string{"registry.example.com/project/registry": "project"},
			expectedUser: "project",
			expectedHost: "registry.example.com",
		},
		{
			name:    "path key of another repository",
			imageID: "registry.example.com/other/registry/app" + digest,
			auths:   map[string]string{"registry.example.com/project/registry": "project"},
		},
		{
			name:    "key of another registry",
			imageID: "registry.example.com/project/registry/app" + digest,
			auths:   map[string]string{"other.example.com": "other"},
		},
		{
			name:    "key with another port",
			imageID: "registry.example.com:5000/app" + digest,
			auths:   map[string]string{"registry.example.com": "registry"},
		},
		{
			name:         "longest path wins",
			imageID:      "registry.example.com/project/registry/app" + digest,
			auths:        map[string]string{"registry.example.com": "registry", "https://registry.example.com/project/registry": "project", "registry.example.com/project": "parent"},
			expectedUser: "project",
			expectedHost: "registry.example.com",
		},
		{
			name:         "wildcard host",
			imageID:      "registry.example.com/project/registry/app" + digest,
			auths:        map[string]string{"*.example.com": "wildcard"},
			expectedUser: "wildcard",
			expectedHost: "registry.example.com",
		},
		{
			name:    "wildcard host with another label count",
			imageID: "registry.example.com/project/registry/app" + digest,
			auths:   map[string]string{"*.com": "wildcard"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := ResolveAuthConfigWithPullSecret(RegistryImage{ImageID: tt.imageID}, pullSecret(t, tt.auths))
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedUser, cfg.Username)
			assert.Equal(t, tt.expectedHost, cfg.ServerAddress)
		})
	}
}

func TestResolveAuthConfigWithLegacyPullSecret(t *testing.T) {
	secret := KubeCreds{
		SecretName:      "legacy",
		SecretCredsData: []byte(`{"https://index.docker.io/v1/":{"auth":"am9lam9lOmhlbGxv"}}`),
		IsLegacySecret:  true,
	}

	cfg, err := ResolveAuthConfigWithPullSecret(RegistryImage{ImageID: "docker.io/library/alpine" + digest}, secret)
	assert.NoError(t, err)
	assert.Equal(t, "joejoe", cfg.Username)
	assert.Equal(t, "index.docker.io", cfg.ServerAddress)
}

func TestConvertSecretsUsesRegistryHostAsAuthority(t *testing.T) {
	img := RegistryImage{
		ImageID:     "docker.io/library/alpine" + digest,
		PullSecrets: []*KubeCreds{{SecretName: "hub", SecretCredsData: pullSecret(t, map[string]string{"https://index.docker.io/v1/": "hub"}).SecretCredsData}},
	}

	creds := ConvertSecrets(img, map[string]string{})
	assert.Len(t, creds, 1)
	assert.Equal(t, "index.docker.io", creds[0].Authority)
	assert.Equal(t, "hub", creds[0].Username)
}
