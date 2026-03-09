package auth

type AuthType string

const (
	AuthTypePAT            AuthType = "pat"
	AuthTypeOAuth          AuthType = "oauth"
	AuthTypeManagedIdentity AuthType = "managed-identity"
	AuthTypeSPN            AuthType = "spn"
)

type AuthConfig struct {
	Type          AuthType `json:"type" yaml:"type"`
	PAT           string   `json:"pat,omitempty" yaml:"pat,omitempty"`
	ClientID      string   `json:"clientId,omitempty" yaml:"clientId,omitempty"`
	ClientSecret  string   `json:"clientSecret,omitempty" yaml:"clientSecret,omitempty"`
	TenantID      string   `json:"tenantId,omitempty" yaml:"tenantId,omitempty"`
	ManagedID     string   `json:"managedId,omitempty" yaml:"managedId,omitempty"`
	Scopes        []string `json:"scopes,omitempty" yaml:"scopes,omitempty"`
}

func (a *AuthConfig) Validate() error {
	if a == nil {
		return nil
	}

	switch a.Type {
	case AuthTypePAT:
		if a.PAT == "" {
			return ErrPATRequired
		}
	case AuthTypeOAuth:
		if a.ClientID == "" {
			return ErrClientIDRequired
		}
	case AuthTypeManagedIdentity:
	case AuthTypeSPN:
		if a.ClientID == "" || a.TenantID == "" {
			return ErrClientIDAndTenantRequired
		}
	default:
		return ErrInvalidAuthType
	}

	return nil
}

func (a *AuthConfig) GetScopes() []string {
	defaultScopes := []string{
		"vso.packaging",
		"vso.code",
		"vso.project",
		"vso.build",
		"vso.release",
	}

	if a.Scopes != nil && len(a.Scopes) > 0 {
		return a.Scopes
	}

	switch a.Type {
	case AuthTypePAT:
		return []string{"vso.packaging", "vso.code", "vso.project"}
	case AuthTypeManagedIdentity:
		return []string{".default"}
	default:
		return defaultScopes
	}
}

type AuthHeaders map[string]string

type Authenticator interface {
	Validate() bool
	Authenticate() (AuthHeaders, error)
	GetHeaders() AuthHeaders
	GetScopes() []string
	GetExpiration() *int64
}

var (
	ErrPATRequired              = &AuthError{message: "PAT is required for PAT authentication"}
	ErrClientIDRequired         = &AuthError{message: "Client ID is required for OAuth authentication"}
	ErrClientIDAndTenantRequired = &AuthError{message: "Client ID and Tenant ID are required for service principal authentication"}
	ErrInvalidAuthType          = &AuthError{message: "Invalid authentication type"}
)

type AuthError struct {
	message string
}

func (e *AuthError) Error() string {
	return e.message
}

func NewAuthConfig(authType AuthType) *AuthConfig {
	return &AuthConfig{
		Type:   authType,
		Scopes: []string{"vso.packaging", "vso.code", "vso.project"},
	}
}
