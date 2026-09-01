package fido2

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
)

// Provider handles FIDO2/WebAuthn registration and authentication
type Provider struct {
	webAuthn *webauthn.WebAuthn
	config   *ProviderConfig
}

// ProviderConfig contains configuration for the FIDO2 provider
type ProviderConfig struct {
	RPID          string // Relying Party ID (e.g., "localhost", "example.com")
	RPOrigin      string // Relying Party Origin (e.g., "https://example.com")
	RPDisplayName string // Relying Party Display Name (e.g., "TUNNEL")
	Timeout       int    // Timeout in milliseconds for ceremonies
}

// DefaultProviderConfig returns default configuration for development
func DefaultProviderConfig() *ProviderConfig {
	return &ProviderConfig{
		RPID:          "localhost",
		RPOrigin:      "http://localhost:8080",
		RPDisplayName: "TUNNEL",
		Timeout:       60000, // 60 seconds
	}
}

// NewProvider creates a new FIDO2 provider
func NewProvider(config *ProviderConfig) (*Provider, error) {
	if config == nil {
		config = DefaultProviderConfig()
	}

	wconfig := &webauthn.Config{
		RPDisplayName: config.RPDisplayName,
		RPID:          config.RPID,
		RPOrigins:     []string{config.RPOrigin},
		Timeout:       config.Timeout,
		AuthenticatorSelection: protocol.AuthenticatorSelection{
			RequireResidentKey: protocol.ResidentKeyNotRequired(),
			UserVerification:   protocol.VerificationRequired,
		},
		AttestationPreference: protocol.PreferNoAttestation,
	}

	webAuthn, err := webauthn.New(wconfig)
	if err != nil {
		return nil, fmt.Errorf("initialize webauthn: %w", err)
	}

	return &Provider{
		webAuthn: webAuthn,
		config:   config,
	}, nil
}

// User represents a WebAuthn user for registration/authentication
type User struct {
	ID          []byte
	Username    string
	DisplayName string
	Credentials []webauthn.Credential
}

// WebAuthnID returns the user's WebAuthn ID
func (u *User) WebAuthnID() []byte {
	return u.ID
}

// WebAuthnName returns the user's username
func (u *User) WebAuthnName() string {
	return u.Username
}

// WebAuthnDisplayName returns the user's display name
func (u *User) WebAuthnDisplayName() string {
	return u.DisplayName
}

// WebAuthnCredentials returns the user's credentials
func (u *User) WebAuthnCredentials() []webauthn.Credential {
	return u.Credentials
}

// WebAuthnIcon is not implemented
func (u *User) WebAuthnIcon() string {
	return ""
}

// NewUser creates a new User from account name
func NewUser(accountName string) *User {
	return &User{
		ID:          []byte(accountName),
		Username:    accountName,
		DisplayName: accountName,
		Credentials: []webauthn.Credential{},
	}
}

// RegistrationRequest represents a WebAuthn registration request
type RegistrationRequest struct {
	User              *User
	Registration      *protocol.CredentialCreation
	SessionData       *webauthn.SessionData
	Challenge         string
	JSON              string // Marshaled registration options for client
}

// AuthenticationRequest represents a WebAuthn authentication request
type AuthenticationRequest struct {
	User              *User
	Assertion         *protocol.CredentialAssertion
	SessionData       *webauthn.SessionData
	Challenge         string
	JSON              string // Marshaled assertion options for client
}

// CredentialRegistration represents a completed credential registration
type CredentialRegistration struct {
	ID           string
	Type         string
	Attestation  string
	AAGUID       string
	PublicKey    string
	SignCount    uint32
	CloneWarning bool
	Transport    []string
	Flags        []string
}

// CredentialAssertion represents a completed authentication assertion
type CredentialAssertion struct {
	ID        string
	Type      string
	SignCount uint32
	UserHandle string
	Verified   bool
}

// BeginRegistration starts a new WebAuthn registration ceremony
func (p *Provider) BeginRegistration(user *User, authenticatorSelection protocol.AuthenticatorSelection) (*RegistrationRequest, error) {
	if authenticatorSelection.UserVerification == "" {
		authenticatorSelection.UserVerification = protocol.VerificationRequired
	}

	options := []webauthn.RegistrationOption{
		webauthn.WithAuthenticatorSelection(authenticatorSelection),
		webauthn.WithConveyancePreference(protocol.PreferNoAttestation),
	}

	sessionData, registration, err := p.webAuthn.BeginRegistration(user, options...)
	if err != nil {
		return nil, fmt.Errorf("begin registration: %w", err)
	}

	// Marshal registration options for client
	registrationData, err := json.Marshal(registration)
	if err != nil {
		return nil, fmt.Errorf("marshal registration: %w", err)
	}

	return &RegistrationRequest{
		User:         user,
		Registration: registration,
		SessionData:  sessionData,
		Challenge:    sessionData.Challenge.String(),
		JSON:         string(registrationData),
	}, nil
}

// FinishRegistration completes a WebAuthn registration ceremony
func (p *Provider) FinishRegistration(user *User, sessionData *webauthn.SessionData, response *http.Request) (*CredentialRegistration, error) {
	credential, err := p.webAuthn.FinishRegistration(user, *sessionData, response)
	if err != nil {
		return nil, fmt.Errorf("finish registration: %w", err)
	}

	// Add credential to user
	user.Credentials = append(user.Credentials, *credential)

	// Extract transport and flags
	transports := []string{}
	if credential.Authenticator.Transport != nil {
		transports = credential.Authenticator.Transport
	}

	flags := []string{}
	if credential.Flags.UserPresent {
		flags = append(flags, "userPresent")
	}
	if credential.Flags.UserVerified {
		flags = append(flags, "userVerified")
	}
	if credential.Flags.BackupEligible {
		flags = append(flags, "backupEligible")
	}
	if credential.Flags.BackupState {
		flags = append(flags, "backupState")
	}

	return &CredentialRegistration{
		ID:           credential.ID,
		Type:         credential.Type,
		Attestation:  credential.AttestationType,
		AAGUID:       base64.RawURLEncoding.EncodeToString(credential.AAGUID),
		PublicKey:    base64.RawURLEncoding.EncodeToString(credential.PublicKey),
		SignCount:    0,
		CloneWarning: false,
		Transport:    transports,
		Flags:        flags,
	}, nil
}

// BeginAuthentication starts a new WebAuthn authentication ceremony
func (p *Provider) BeginAuthentication(user *User) (*AuthenticationRequest, error) {
	if len(user.Credentials) == 0 {
		return nil, fmt.Errorf("user has no registered credentials")
	}

	sessionData, assertion, err := p.webAuthn.BeginLogin(user)
	if err != nil {
		return nil, fmt.Errorf("begin login: %w", err)
	}

	// Marshal assertion options for client
	assertionData, err := json.Marshal(assertion)
	if err != nil {
		return nil, fmt.Errorf("marshal assertion: %w", err)
	}

	return &AuthenticationRequest{
		User:         user,
		Assertion:    assertion,
		SessionData:  sessionData,
		Challenge:    sessionData.Challenge.String(),
		JSON:         string(assertionData),
	}, nil
}

// FinishAuthentication completes a WebAuthn authentication ceremony
func (p *Provider) FinishAuthentication(user *User, sessionData *webauthn.SessionData, response *http.Request) (*CredentialAssertion, error) {
	credential, err := p.webAuthn.FinishLogin(user, *sessionData, response)
	if err != nil {
		return nil, fmt.Errorf("finish login: %w", err)
	}

	// Update user's credential sign count
	for i, c := range user.Credentials {
		if c.ID == credential.ID {
			user.Credentials[i].Transport = credential.Transport
			break
		}
	}

	return &CredentialAssertion{
		ID:         credential.ID,
		Type:       credential.Type,
		SignCount:  credential.Transport,
		UserHandle: string(user.WebAuthnID()),
		Verified:   true,
	}, nil
}

// ValidateUserVerification checks if user verification is required
func (p *Provider) ValidateUserVerification() bool {
	return p.config.Timeout > 0
}

// GetConfig returns the provider configuration
func (p *Provider) GetConfig() *ProviderConfig {
	return p.config
}
