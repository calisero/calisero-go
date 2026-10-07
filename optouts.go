package calisero

import (
	"context"
	"time"
)

// OptOut is a phone number that opted out of receiving messages.
type OptOut struct {
	ID string `json:"id"`

	Phone string `json:"phone"`

	// Reason is why the number opted out; empty when none was given.
	Reason string `json:"reason"`

	CreatedAt time.Time `json:"created_at"`

	UpdatedAt time.Time `json:"updated_at"`
}

// UnmarshalJSON reads an opt-out as the API sends it, the way the client does: a
// required field missing or malformed is an error naming it.
func (o *OptOut) UnmarshalJSON(data []byte) error {
	return unmarshalResource(data, o, "opt-out", readOptOut)
}

func readOptOut(value any, context string) (OptOut, error) {
	r := newReader(value, context)
	optOut := OptOut{
		ID:        r.str("id"),
		Phone:     r.str("phone"),
		Reason:    r.optStr("reason"),
		CreatedAt: r.timestamp("created_at"),
		UpdatedAt: r.timestamp("updated_at"),
	}

	if r.err != nil {
		return OptOut{}, r.err
	}

	return optOut, nil
}

// CreateOptOutRequest is a phone number to opt out of receiving messages.
type CreateOptOutRequest struct {
	// Phone is the phone number in international E.164 format. Required.
	Phone string

	// Reason is why the number opted out, at most 600 characters.
	Reason string
}

// UpdateOptOutRequest is the new data of an opt-out.
type UpdateOptOutRequest struct {
	// Phone is the phone number in international E.164 format. Required.
	Phone string

	// Reason is why the number opted out, at most 600 characters.
	Reason string
}

// optOutPayload is the body of POST /opt-outs and PUT /opt-outs/{id}.
type optOutPayload struct {
	Phone  string `json:"phone"`
	Reason string `json:"reason,omitempty"`
}

// OptOutService manages the phone numbers that opted out of receiving messages:
// reach it as Client.OptOuts. The API refuses to send to an opted-out number.
type OptOutService struct {
	client *Client
}

// Create opts a phone number out.
func (s *OptOutService) Create(ctx context.Context, request *CreateOptOutRequest) (*Response[OptOut], error) {
	if request == nil {
		return nil, errNilRequest("OptOuts.Create")
	}

	res, err := s.client.post(ctx, "/opt-outs", optOutPayload{Phone: request.Phone, Reason: request.Reason})
	if err != nil {
		return nil, err
	}

	return single(res, readOptOut, "opt-out")
}

// Get retrieves an opt-out by its ID.
func (s *OptOutService) Get(ctx context.Context, optOutID string) (*Response[OptOut], error) {
	id, err := pathID(optOutID, "opt-out ID")
	if err != nil {
		return nil, err
	}

	res, err := s.client.get(ctx, "/opt-outs/"+id, nil)
	if err != nil {
		return nil, err
	}

	return single(res, readOptOut, "opt-out")
}

// List lists the opt-outs, 50 a page; options may be nil.
func (s *OptOutService) List(ctx context.Context, options *ListOptions) (*Page[OptOut], error) {
	var pageNumber int

	if options != nil {
		pageNumber = options.Page
	}

	query, err := pageQuery(pageNumber)
	if err != nil {
		return nil, err
	}

	res, err := s.client.get(ctx, "/opt-outs", query)
	if err != nil {
		return nil, err
	}

	return page(res, readOptOut)
}

// Update updates an opt-out.
func (s *OptOutService) Update(ctx context.Context, optOutID string, request *UpdateOptOutRequest) (*Response[OptOut], error) {
	id, err := pathID(optOutID, "opt-out ID")
	if err != nil {
		return nil, err
	}

	if request == nil {
		return nil, errNilRequest("OptOuts.Update")
	}

	res, err := s.client.put(ctx, "/opt-outs/"+id, optOutPayload{Phone: request.Phone, Reason: request.Reason})
	if err != nil {
		return nil, err
	}

	return single(res, readOptOut, "opt-out")
}

// Delete deletes an opt-out: the number can receive messages again.
func (s *OptOutService) Delete(ctx context.Context, optOutID string) error {
	id, err := pathID(optOutID, "opt-out ID")
	if err != nil {
		return err
	}

	_, err = s.client.delete(ctx, "/opt-outs/"+id)

	return err
}
