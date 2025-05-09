package oauth

import (
	"fmt"

	"github.com/zalando/go-keyring"
	"go.acuvity.ai/elemental"
)

func TokenToKeyring(server string, t Credentials) error {

	data, err := elemental.Encode(elemental.EncodingTypeJSON, t)
	if err != nil {
		return fmt.Errorf("unable to encode token data: %w", err)
	}

	if err := keyring.Set("minibridge", server, string(data)); err != nil {
		return fmt.Errorf("unable to store token into keyring: %w", err)
	}

	return nil
}

func TokenFromKeyring(server string) (Credentials, error) {

	data, err := keyring.Get("minibridge", server)
	if err != nil {
		return Credentials{}, fmt.Errorf("unable to retrieve token from keyring: %w", err)
	}

	t := Credentials{}
	if err := elemental.Decode(elemental.EncodingTypeJSON, []byte(data), &t); err != nil {
		return Credentials{}, fmt.Errorf("unable to decode token data from keychain: %w", err)
	}

	return t, nil
}
