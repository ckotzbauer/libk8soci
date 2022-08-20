package oci

type KubeCreds struct {
	SecretName      string
	SecretCredsData []byte
	IsLegacySecret  bool
}

type RegistryImage struct {
	ImageID     string
	Image       string
	PullSecrets []*KubeCreds
}
