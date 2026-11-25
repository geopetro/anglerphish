package dmi

import (
	"context"
	"errors"

	"github.com/gophish/gophish/mailer"
)

// NewDialer validates the config and returns the provider dialer.
func NewDialer(c Config) (mailer.Dialer, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	switch c.Provider {
	case ProviderMicrosoft:
		return newMicrosoftDialer(c)
	case ProviderGoogle:
		return newGoogleDialer(c)
	}
	return nil, errors.New("Provider is not valid")
}

// TestConnection checks access to the mailbox without creating anything.
func TestConnection(ctx context.Context, c Config, mailbox string) error {
	if err := c.Validate(); err != nil {
		return err
	}
	mailbox, err := c.CheckRecipient(mailbox)
	if err != nil {
		return err
	}
	switch c.Provider {
	case ProviderMicrosoft:
		return testMicrosoft(ctx, c, mailbox)
	case ProviderGoogle:
		return testGoogle(ctx, c, mailbox)
	}
	return errors.New("Provider is not valid")
}
