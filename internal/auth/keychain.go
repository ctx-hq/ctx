package auth

// Keychain abstracts secure credential storage.
type Keychain interface {
	Get(service, account string) (string, error)
	Set(service, account, secret string) error
	Delete(service, account string) error
}

const (
	keychainService = "ctx-cli"
	keychainAccount = "auth-token"
)

// defaultKeychain is set by platform-specific init code.
// Falls back to file-based storage if no platform keychain is available.
var defaultKeychain Keychain

// getKeychain returns the platform keychain, falling back to file-based.
func getKeychain() Keychain {
	if defaultKeychain != nil {
		return defaultKeychain
	}
	return &fileKeychain{}
}
