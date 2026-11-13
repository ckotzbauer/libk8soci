package oci

import (
	"bytes"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/anchore/stereoscope/pkg/image"
	"github.com/docker/cli/cli/config"
	"github.com/docker/cli/cli/config/configfile"
	"github.com/docker/cli/cli/config/types"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/sirupsen/logrus"

	"github.com/novln/docker-parser/docker"
)

// ResolveAuthConfigWithPullSecret selects the pull secret entry like the kubelet: longest matching path wins
func ResolveAuthConfigWithPullSecret(image RegistryImage, pullSecret KubeCreds) (types.AuthConfig, error) {
	var cf *configfile.ConfigFile
	var err error

	if pullSecret.IsLegacySecret {
		cf = configfile.New("")
		err = LegacyLoadFromReader(bytes.NewReader(pullSecret.SecretCredsData), cf)
	} else {
		cf, err = config.LoadFromReader(bytes.NewReader(pullSecret.SecretCredsData))
	}

	if err != nil {
		return types.AuthConfig{}, err
	}

	ref, err := name.ParseReference(image.ImageID)
	if err != nil {
		return types.AuthConfig{}, err
	}

	registry := ref.Context().RegistryStr()
	repository := ref.Context().RepositoryStr()

	var match types.AuthConfig
	matchKey := ""
	matchPathLen := -1
	for key, authConfig := range cf.GetAuthConfigs() {
		host, path, ok := parseAuthKey(key)
		if !ok || !hostMatches(host, registry) || !strings.HasPrefix(repository, path) {
			continue
		}

		if len(path) > matchPathLen || (len(path) == matchPathLen && key < matchKey) {
			match, matchKey, matchPathLen = authConfig, key, len(path)
		}
	}

	if matchPathLen < 0 {
		return types.AuthConfig{}, nil
	}

	match.ServerAddress = registry
	return match, nil
}

// parseAuthKey splits a docker config key into host and path, like the kubelet
func parseAuthKey(key string) (string, string, bool) {
	if !strings.Contains(key, "://") {
		key = "https://" + key
	}

	parsed, err := url.Parse(key)
	if err != nil || parsed.Host == "" {
		return "", "", false
	}

	path := parsed.Path
	if strings.HasPrefix(path, "/v1/") || strings.HasPrefix(path, "/v2/") {
		path = path[3:]
	}

	return parsed.Host, strings.Trim(path, "/"), true
}

// hostMatches matches each label as a glob pattern, Docker Hub aliases being equivalent
func hostMatches(configHost, registry string) bool {
	if dockerHubHosts[configHost] && dockerHubHosts[registry] {
		return true
	}

	configLabels := strings.Split(configHost, ".")
	registryLabels := strings.Split(registry, ".")
	if len(configLabels) != len(registryLabels) {
		return false
	}

	for i := range configLabels {
		if matched, err := filepath.Match(configLabels[i], registryLabels[i]); err != nil || !matched {
			return false
		}
	}

	return true
}

var dockerHubHosts = map[string]bool{
	"docker.io":            true,
	"index.docker.io":      true,
	"registry-1.docker.io": true,
}

func ResolveAuthConfig(image RegistryImage) (types.AuthConfig, error) {
	// to not break JobImages this function needs to redirect to the actual resolve-function, using the first pullSecret from the list if exists
	if len(image.PullSecrets) > 0 {
		return ResolveAuthConfigWithPullSecret(image, *image.PullSecrets[0])
	} else {
		return types.AuthConfig{}, nil
	}
}

func ConvertSecrets(img RegistryImage, proxyRegistryMap map[string]string) []image.RegistryCredentials {
	credentials := make([]image.RegistryCredentials, 0)
	for _, secret := range img.PullSecrets {
		cfg, err := ResolveAuthConfigWithPullSecret(img, *secret)
		if err != nil {
			logrus.WithError(err).Warnf("image: %s, Read authentication configuration from secret: %s failed", img.ImageID, secret.SecretName)
			continue
		}

		for registryToReplace, proxyRegistry := range proxyRegistryMap {
			if cfg.ServerAddress == registryToReplace ||
				(strings.Contains(cfg.ServerAddress, docker.DefaultHostname) && strings.Contains(registryToReplace, docker.DefaultHostname)) {
				cfg.ServerAddress = proxyRegistry
			}
		}

		credentials = append(credentials, image.RegistryCredentials{
			Username:  cfg.Username,
			Password:  cfg.Password,
			Token:     cfg.RegistryToken,
			Authority: cfg.ServerAddress,
		})
	}

	return credentials
}
