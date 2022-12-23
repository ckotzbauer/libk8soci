package oci

import (
	"bytes"
	"strings"

	"github.com/anchore/stereoscope/pkg/image"
	"github.com/docker/cli/cli/config"
	"github.com/docker/cli/cli/config/configfile"
	"github.com/docker/cli/cli/config/types"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/sirupsen/logrus"

	parser "github.com/novln/docker-parser"
	"github.com/novln/docker-parser/docker"
)

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

	fullRef, err := parser.Parse(image.ImageID)
	if err != nil {
		return types.AuthConfig{}, err
	}

	reg, err := name.NewRegistry(fullRef.Registry())
	if err != nil {
		return types.AuthConfig{}, err
	}

	regKey := reg.RegistryStr()

	if regKey == name.DefaultRegistry {
		regKey = authn.DefaultAuthKey
	}

	cfg, err := cf.GetAuthConfig(regKey)
	if err != nil {
		return types.AuthConfig{}, err
	}

	return cfg, nil
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
