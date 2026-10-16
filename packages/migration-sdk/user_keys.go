// Copyright 2015 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// PublicKey publickey is a user key to push code to repository
type PublicKey struct {
	ID          int64     `json:"id"`
	Key         string    `json:"key"`
	URL         string    `json:"url,omitempty"`
	Title       string    `json:"title,omitempty"`
	Fingerprint string    `json:"fingerprint,omitempty"`
	Created     time.Time `json:"created_at,omitempty"`
	Updated     time.Time `json:"last_used_at,omitempty"`
	Owner       *User     `json:"user,omitempty"`
	ReadOnly    bool      `json:"read_only,omitempty"`
	KeyType     string    `json:"key_type,omitempty"`
}

// ListPublicKeysOptions options for listing a user's PublicKeys
type ListPublicKeysOptions struct {
	ListOptions
}

// ListPublicKeys list all the public keys of the user
func (c *UsersService) ListPublicKeys(ctx context.Context, user string, opt ListPublicKeysOptions) ([]*PublicKey, *Response, error) {
	if err := escapeValidatePathSegments(&user); err != nil {
		return nil, nil, err
	}
	opt.setDefaults()
	keys := make([]*PublicKey, 0, opt.PageSize)
	resp, err := c.getParsedResponse(ctx, "GET", fmt.Sprintf("/users/%s/keys?%s", user, opt.getURLQuery().Encode()), nil, nil, &keys)
	return keys, resp, err
}

// ListMyPublicKeys list all the public keys of current user
func (c *UsersService) ListMyPublicKeys(ctx context.Context, opt ListPublicKeysOptions) ([]*PublicKey, *Response, error) {
	opt.setDefaults()
	keys := make([]*PublicKey, 0, opt.PageSize)
	resp, err := c.getParsedResponse(ctx, "GET", fmt.Sprintf("/user/keys?%s", opt.getURLQuery().Encode()), nil, nil, &keys)
	return keys, resp, err
}

// GetPublicKey get current user's public key by key id
func (c *UsersService) GetPublicKey(ctx context.Context, keyID int64) (*PublicKey, *Response, error) {
	key := new(PublicKey)
	resp, err := c.getParsedResponse(ctx, "GET", fmt.Sprintf("/user/keys/%d", keyID), nil, nil, &key)
	return key, resp, err
}

// CreateKeyOption options when creating a key
type CreateKeyOption struct {
	// Title of the key to add
	Title string `json:"title"`
	// An armored SSH key to add
	Key string `json:"key"`
	// Describe if the key has only read access or read/write
	ReadOnly bool `json:"read_only"`
}

// CreatePublicKey create public key with options
func (c *UsersService) CreatePublicKey(ctx context.Context, opt CreateKeyOption) (*PublicKey, *Response, error) {
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, nil, err
	}
	key := new(PublicKey)
	resp, err := c.getParsedResponse(ctx, "POST", "/user/keys", jsonHeader, bytes.NewReader(body), key)
	return key, resp, err
}

// DeletePublicKey delete public key with key id
func (c *UsersService) DeletePublicKey(ctx context.Context, keyID int64) (*Response, error) {
	return c.doRequestWithStatusHandle(ctx, "DELETE", fmt.Sprintf("/user/keys/%d", keyID), nil, nil)
}

// GPGKey a user GPG key to sign commit and tag in repository
type GPGKey struct {
	ID                int64          `json:"id"`
	PrimaryKeyID      string         `json:"primary_key_id"`
	KeyID             string         `json:"key_id"`
	PublicKey         string         `json:"public_key"`
	Emails            []*GPGKeyEmail `json:"emails"`
	SubsKey           []*GPGKey      `json:"subkeys"`
	CanSign           bool           `json:"can_sign"`
	CanEncryptComms   bool           `json:"can_encrypt_comms"`
	CanEncryptStorage bool           `json:"can_encrypt_storage"`
	CanCertify        bool           `json:"can_certify"`
	Verified          bool           `json:"verified"`
	Created           time.Time      `json:"created_at,omitempty"`
	Expires           time.Time      `json:"expires_at,omitempty"`
}

// GPGKeyEmail an email attached to a GPGKey
type GPGKeyEmail struct {
	Email    string `json:"email"`
	Verified bool   `json:"verified"`
}

// ListGPGKeysOptions options for listing a user's GPGKeys
type ListGPGKeysOptions struct {
	ListOptions
}

// ListGPGKeys list all the GPG keys of the user
func (c *UsersService) ListGPGKeys(ctx context.Context, user string, opt ListGPGKeysOptions) ([]*GPGKey, *Response, error) {
	if err := escapeValidatePathSegments(&user); err != nil {
		return nil, nil, err
	}
	opt.setDefaults()
	keys := make([]*GPGKey, 0, opt.PageSize)
	resp, err := c.getParsedResponse(ctx, "GET", fmt.Sprintf("/users/%s/gpg_keys?%s", user, opt.getURLQuery().Encode()), nil, nil, &keys)
	return keys, resp, err
}

// ListMyGPGKeys list all the GPG keys of current user
func (c *UsersService) ListMyGPGKeys(ctx context.Context, opt *ListGPGKeysOptions) ([]*GPGKey, *Response, error) {
	opt.setDefaults()
	keys := make([]*GPGKey, 0, opt.PageSize)
	resp, err := c.getParsedResponse(ctx, "GET", fmt.Sprintf("/user/gpg_keys?%s", opt.getURLQuery().Encode()), nil, nil, &keys)
	return keys, resp, err
}

// GetGPGKey get current user's GPG key by key id
func (c *UsersService) GetGPGKey(ctx context.Context, keyID int64) (*GPGKey, *Response, error) {
	key := new(GPGKey)
	resp, err := c.getParsedResponse(ctx, "GET", fmt.Sprintf("/user/gpg_keys/%d", keyID), nil, nil, &key)
	return key, resp, err
}

// CreateGPGKeyOption options create user GPG key
type CreateGPGKeyOption struct {
	// An armored GPG key to add
	ArmoredKey string `json:"armored_public_key"`
	// An optional armored signature for the GPG key
	Signature string `json:"armored_signature,omitempty"`
}

// CreateGPGKey create GPG key with options
func (c *UsersService) CreateGPGKey(ctx context.Context, opt CreateGPGKeyOption) (*GPGKey, *Response, error) {
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, nil, err
	}
	key := new(GPGKey)
	resp, err := c.getParsedResponse(ctx, "POST", "/user/gpg_keys", jsonHeader, bytes.NewReader(body), key)
	return key, resp, err
}

// DeleteGPGKey delete GPG key with key id
func (c *UsersService) DeleteGPGKey(ctx context.Context, keyID int64) (*Response, error) {
	return c.doRequestWithStatusHandle(ctx, "DELETE", fmt.Sprintf("/user/gpg_keys/%d", keyID), nil, nil)
}

// GetGPGKeyVerificationToken gets a verification token for adding a GPG key.
// Returns the token as a plain string (API returns text/plain, not JSON).
// The user should sign this token with their GPG key and submit via VerifyGPGKey.
func (c *UsersService) GetGPGKeyVerificationToken(ctx context.Context) (string, *Response, error) {
	body, resp, err := c.getResponse(ctx, "GET", "/user/gpg_key_token", nil, nil)
	return string(body), resp, err
}

// VerifyGPGKeyOption options for verifying a GPG key
type VerifyGPGKeyOption struct {
	// KeyID is the GPG key ID to verify
	KeyID string `json:"key_id"`
	// Signature is the ASCII-armored signature of the verification token
	Signature string `json:"armored_signature"`
}

// VerifyGPGKey verifies a GPG key by submitting a signed verification token.
// First call GetGPGKeyVerificationToken to get the token, sign it with the GPG key,
// then call this with the key ID and armored signature.
func (c *UsersService) VerifyGPGKey(ctx context.Context, opt VerifyGPGKeyOption) (*GPGKey, *Response, error) {
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, nil, err
	}
	key := new(GPGKey)
	resp, err := c.getParsedResponse(ctx, "POST", "/user/gpg_key_verify",
		jsonHeader, bytes.NewReader(body), key)
	return key, resp, err
}
