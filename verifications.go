package calisero

import (
	"context"
	"time"
)

// VerificationStatus is the status of a verification; other values may come with
// future API versions.
type VerificationStatus string

// The statuses of a verification.
const (
	VerificationStatusUnverified VerificationStatus = "unverified"
	VerificationStatusVerified   VerificationStatus = "verified"
)

// Verification is a phone verification: a one-time code sent by SMS, to be
// validated.
type Verification struct {
	ID string `json:"id"`

	Phone string `json:"phone"`

	// Brand is the brand named in the default message; empty when a template was used.
	Brand string `json:"brand"`

	Status VerificationStatus `json:"status"`

	// Template is the custom message, holding {code}; empty for the default message.
	Template string `json:"template"`

	CreatedAt time.Time `json:"created_at"`

	ExpiresAt time.Time `json:"expires_at"`

	// VerifiedAt is when the code was validated; nil until it is.
	VerifiedAt *time.Time `json:"verified_at"`

	// Attempts is how many times a code was checked against this verification.
	Attempts int `json:"attempts"`

	Expired bool `json:"expired"`
}

// UnmarshalJSON reads a verification as the API sends it, the way the client
// does: a required field missing or malformed is an error naming it.
func (v *Verification) UnmarshalJSON(data []byte) error {
	return unmarshalResource(data, v, "verification", readVerification)
}

func readVerification(value any, context string) (Verification, error) {
	r := newReader(value, context)
	verification := Verification{
		ID:         r.str("id"),
		Phone:      r.str("phone"),
		Brand:      r.optStr("brand"),
		Status:     VerificationStatus(r.str("status")),
		Template:   r.optStr("template"),
		CreatedAt:  r.timestamp("created_at"),
		ExpiresAt:  r.timestamp("expires_at"),
		VerifiedAt: r.optTimestamp("verified_at"),
		Attempts:   r.integer("attempts"),
		Expired:    r.boolean("expired"),
	}

	if r.err != nil {
		return Verification{}, r.err
	}

	return verification, nil
}

// CreateVerificationRequest is a verification to start: Calisero generates a code
// and sends it by SMS. The optional fields left at their zero value are not sent.
type CreateVerificationRequest struct {
	// Phone is the phone number to verify, in international E.164 format. Required.
	Phone string

	// Brand is the brand named in the default message, at most 120 characters:
	// required unless a Template is given.
	Brand string

	// Template is a message of your own, at most 600 characters, that must hold
	// {code}.
	Template string

	// ExpiresIn is how many minutes the code stays valid, 1 to 10: 5 when 0.
	ExpiresIn int
}

// VerificationCheckRequest is a code to check against the latest verification of
// a phone number.
type VerificationCheckRequest struct {
	// Phone is the phone number the code was sent to, in international E.164
	// format. Required.
	Phone string

	// Code is the 6-character code received, in any case. Required.
	Code string
}

// VerificationListOptions are the options of VerificationService.List.
type VerificationListOptions struct {
	// Page is the page to read, from 1, each holding 50 items; 0 reads the first one.
	Page int

	// Status keeps only the verifications with this status; all of them when empty.
	Status VerificationStatus
}

// createVerificationPayload is the body of POST /verifications.
type createVerificationPayload struct {
	Phone     string `json:"phone"`
	Brand     string `json:"brand,omitempty"`
	Template  string `json:"template,omitempty"`
	ExpiresIn int    `json:"expires_in,omitempty"`
}

// verificationCheckPayload is the body of PUT /verifications/validate.
type verificationCheckPayload struct {
	Phone string `json:"phone"`
	Code  string `json:"code"`
}

// VerificationService verifies phone numbers with one-time codes (OTP, 2FA):
// reach it as Client.Verifications.
type VerificationService struct {
	client *Client
}

// List lists the verifications, newest first, 50 a page; options may be nil.
func (s *VerificationService) List(ctx context.Context, options *VerificationListOptions) (*Page[Verification], error) {
	if options == nil {
		options = &VerificationListOptions{}
	}

	query, err := pageQuery(options.Page)
	if err != nil {
		return nil, err
	}

	if options.Status != "" {
		query.Set("status", string(options.Status))
	}

	res, err := s.client.get(ctx, "/verifications", query)
	if err != nil {
		return nil, err
	}

	return page(res, readVerification)
}

// Create starts a verification: Calisero generates a code and sends it by SMS.
//
// The code's SMS counts towards the account's daily sending limit: its
// ResponseMeta holds what is left of it, and once it is reached Create returns a
// *DailyLimitExceededError.
func (s *VerificationService) Create(ctx context.Context, request *CreateVerificationRequest) (*Response[Verification], error) {
	if request == nil {
		return nil, errNilRequest("Verifications.Create")
	}

	payload := createVerificationPayload{
		Phone:     request.Phone,
		Brand:     request.Brand,
		Template:  request.Template,
		ExpiresIn: request.ExpiresIn,
	}

	res, err := s.client.post(ctx, "/verifications", payload)
	if err != nil {
		return nil, err
	}

	return single(res, readVerification, "verification")
}

// Get retrieves a verification by its ID.
func (s *VerificationService) Get(ctx context.Context, verificationID string) (*Response[Verification], error) {
	id, err := pathID(verificationID, "verification ID")
	if err != nil {
		return nil, err
	}

	res, err := s.client.get(ctx, "/verifications/"+id, nil)
	if err != nil {
		return nil, err
	}

	return single(res, readVerification, "verification")
}

// Validate checks a code against the latest verification of a phone number.
//
// A wrong, expired or exhausted code (5 attempts) returns a *ValidationError
// whose ValidationErrors["code"] says which; a phone number without a
// verification a *NotFoundError. Checking a verified code again succeeds.
func (s *VerificationService) Validate(ctx context.Context, request *VerificationCheckRequest) (*Response[Verification], error) {
	if request == nil {
		return nil, errNilRequest("Verifications.Validate")
	}

	payload := verificationCheckPayload{Phone: request.Phone, Code: request.Code}

	res, err := s.client.put(ctx, "/verifications/validate", payload)
	if err != nil {
		return nil, err
	}

	return single(res, readVerification, "verification")
}
