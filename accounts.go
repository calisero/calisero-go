package calisero

import (
	"context"
	"time"
)

// AccountStatus is the status of an account; other values may come with future
// API versions.
type AccountStatus string

// The statuses of an account.
const (
	AccountStatusActive   AccountStatus = "active"
	AccountStatusPending  AccountStatus = "pending"
	AccountStatusDisabled AccountStatus = "disabled"
)

// Account is a Calisero account. Its optional details are empty, or nil, when the
// account has none.
type Account struct {
	ID string `json:"id"`

	Code string `json:"code"`

	Name string `json:"name"`

	Description string `json:"description"`

	FiscalCode string `json:"fiscal_code"`

	RegistryNumber string `json:"registry_number"`

	IBAN string `json:"iban"`

	City string `json:"city"`

	State string `json:"state"`

	Country string `json:"country"`

	Address string `json:"address"`

	PostalCode *int `json:"postal_code"`

	Email string `json:"email"`

	Phone string `json:"phone"`

	ContactPerson string `json:"contact_person"`

	// Credit is the account's credit balance.
	Credit float64 `json:"credit"`

	Status AccountStatus `json:"status"`

	// Sandbox reports whether this is a sandbox account: its messages are test
	// messages, marked delivered at once, never sent to a phone nor billed.
	Sandbox bool `json:"sandbox"`

	// DailyLimit is how many messages the account can send in a day, midnight to
	// midnight, Romania time; nil when no daily limit applies.
	DailyLimit *int `json:"daily_limit"`

	// DailyRemaining is how many messages the account can still send today; nil
	// when no daily limit applies.
	DailyRemaining *int `json:"daily_remaining"`

	// SentToday is the number of real messages the account created today, Romania
	// time: OTP codes included, test messages left out, one per message whatever
	// its parts.
	SentToday int `json:"sent_today"`

	CreatedAt time.Time `json:"created_at"`
}

// UnmarshalJSON reads an account as the API sends it, the way the client does: a
// required field missing or malformed is an error naming it.
func (a *Account) UnmarshalJSON(data []byte) error {
	return unmarshalResource(data, a, "account", readAccount)
}

func readAccount(value any, context string) (Account, error) {
	r := newReader(value, context)
	account := Account{
		ID:             r.str("id"),
		Code:           r.str("code"),
		Name:           r.str("name"),
		Description:    r.optStr("description"),
		FiscalCode:     r.optStr("fiscal_code"),
		RegistryNumber: r.optStr("registry_number"),
		IBAN:           r.optStr("iban"),
		City:           r.optStr("city"),
		State:          r.optStr("state"),
		Country:        r.optStr("country"),
		Address:        r.optStr("address"),
		PostalCode:     r.optInt("postal_code"),
		Email:          r.optStr("email"),
		Phone:          r.optStr("phone"),
		ContactPerson:  r.optStr("contact_person"),
		Credit:         r.number("credit"),
		Status:         AccountStatus(r.str("status")),
		Sandbox:        r.boolean("sandbox"),
		DailyLimit:     r.optInt("daily_limit"),
		DailyRemaining: r.optInt("daily_remaining"),
		CreatedAt:      r.timestamp("created_at"),
	}

	if sentToday := r.optInt("sent_today"); sentToday != nil {
		account.SentToday = *sentToday
	}

	if r.err != nil {
		return Account{}, r.err
	}

	return account, nil
}

// AccountService reads the account: credit, status and daily sending limit. Reach
// it as Client.Accounts.
type AccountService struct {
	client *Client
}

// Get retrieves an account by its ID, shown in the dashboard.
func (s *AccountService) Get(ctx context.Context, accountID string) (*Response[Account], error) {
	id, err := pathID(accountID, "account ID")
	if err != nil {
		return nil, err
	}

	res, err := s.client.get(ctx, "/accounts/"+id, nil)
	if err != nil {
		return nil, err
	}

	return single(res, readAccount, "account")
}
