package providers

import "errors"

var (
	// Configuration errors
	ErrInvalidConfig = errors.New("invalid configuration")
	ErrNoConfig      = errors.New("no configuration found")
	ErrMissingName   = errors.New("provider name is required")
	ErrMissingToken  = errors.New("authentication token is required")
	ErrMissingKey    = errors.New("authentication key is required")

	// Installation errors
	ErrNotInstalled     = errors.New("provider not installed")
	ErrAlreadyInstalled = errors.New("provider already installed")
	ErrInstallFailed    = errors.New("installation failed")

	// Connection errors
	ErrNotConnected     = errors.New("provider not connected")
	ErrAlreadyConnected = errors.New("provider already connected")
	ErrConnectionFailed = errors.New("connection failed")

	// Provider errors
	ErrProviderNotFound = errors.New("provider not found")
	ErrCommandFailed    = errors.New("command execution failed")
	ErrInvalidResponse  = errors.New("invalid response from provider")
)
