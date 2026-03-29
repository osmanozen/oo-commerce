package domain

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
	bbdomain "github.com/osmanozen/oo-commerce/src/pkg/buildingblocks/domain"
	"github.com/osmanozen/oo-commerce/src/pkg/buildingblocks/types"
)

// ─── Strongly-Typed IDs ──────────────────────────────────────────────────────

type profileTag struct{}
type addressTag struct{}

type ProfileID = types.TypedID[profileTag]
type AddressID = types.TypedID[addressTag]

func NewProfileID() ProfileID                         { return types.NewTypedID[profileTag]() }
func NewAddressID() AddressID                         { return types.NewTypedID[addressTag]() }
func ProfileIDFromString(s string) (ProfileID, error) { return types.TypedIDFromString[profileTag](s) }
func AddressIDFromString(s string) (AddressID, error) { return types.TypedIDFromString[addressTag](s) }

// ─── Address (Owned Entity) ─────────────────────────────────────────────────

type Address struct {
	ID        AddressID `json:"id" db:"id"`
	ProfileID ProfileID `json:"profileId" db:"profile_id"`
	Label     string    `json:"label" db:"label"`
	FirstName string    `json:"firstName" db:"first_name"`
	LastName  string    `json:"lastName" db:"last_name"`
	Street    string    `json:"street" db:"street"`
	City      string    `json:"city" db:"city"`
	State     string    `json:"state" db:"state"`
	ZipCode   string    `json:"zipCode" db:"zip_code"`
	Country   string    `json:"country" db:"country"`
	Phone     string    `json:"phone" db:"phone"`
	IsDefault bool      `json:"isDefault" db:"is_default"`
}

// ─── User Profile Aggregate Root ─────────────────────────────────────────────

type UserProfile struct {
	bbdomain.BaseAggregateRoot
	bbdomain.Auditable
	bbdomain.Versionable

	ID        ProfileID          `json:"id" db:"id"`
	UserID    string             `json:"userId" db:"user_id"`
	Email     types.Email        `json:"email"`
	FirstName string             `json:"firstName" db:"first_name"`
	LastName  string             `json:"lastName" db:"last_name"`
	Phone     *types.PhoneNumber `json:"phone,omitempty"`
	AvatarURL *string            `json:"avatarUrl,omitempty" db:"avatar_url"`
	Addresses []Address          `json:"addresses,omitempty"`
}

var zipCodeRegex = regexp.MustCompile(`^[A-Za-z0-9\- ]{3,20}$`)

// NewUserProfile creates a new UserProfile aggregate.
func NewUserProfile(userID, email, firstName, lastName string) (*UserProfile, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, errors.New("user id is required")
	}

	emailVO, err := types.NewEmail(email)
	if err != nil {
		return nil, err
	}

	p := &UserProfile{
		ID:        NewProfileID(),
		UserID:    userID,
		Email:     emailVO,
		FirstName: strings.TrimSpace(firstName),
		LastName:  strings.TrimSpace(lastName),
		Addresses: []Address{},
	}
	p.SetCreated()

	p.AddDomainEvent(&ProfileCreatedEvent{
		BaseDomainEvent: bbdomain.NewBaseDomainEvent(),
		ProfileID:       p.ID.Value(),
		UserID:          userID,
	})

	return p, nil
}

func (p *UserProfile) DisplayName() string {
	full := strings.TrimSpace(strings.TrimSpace(p.FirstName) + " " + strings.TrimSpace(p.LastName))
	if full == "" {
		return "User"
	}
	return full
}

// UpdateDisplayName validates and updates display name (2-50 chars).
func (p *UserProfile) UpdateDisplayName(name string) error {
	trimmed := strings.TrimSpace(name)
	if len(trimmed) < 2 || len(trimmed) > 50 {
		return errors.New("display name must be 2-50 characters")
	}

	parts := strings.Fields(trimmed)
	p.FirstName = parts[0]
	if len(parts) > 1 {
		p.LastName = strings.Join(parts[1:], " ")
	} else {
		p.LastName = ""
	}

	p.Touch()
	return nil
}

func (p *UserProfile) SetAvatar(url string) error {
	trimmed := strings.TrimSpace(url)
	if trimmed == "" {
		return errors.New("avatar url is required")
	}
	p.AvatarURL = &trimmed
	p.Touch()
	return nil
}

func (p *UserProfile) RemoveAvatar() {
	p.AvatarURL = nil
	p.Touch()
}

func newAddress(label, street, city, state, zipCode, country string) (Address, error) {
	if strings.TrimSpace(label) == "" || strings.TrimSpace(street) == "" || strings.TrimSpace(city) == "" || strings.TrimSpace(state) == "" || strings.TrimSpace(country) == "" {
		return Address{}, errors.New("required address fields missing")
	}
	zip := strings.TrimSpace(zipCode)
	if zip == "" || !zipCodeRegex.MatchString(zip) {
		return Address{}, errors.New("invalid zip code")
	}

	return Address{
		ID:        NewAddressID(),
		Label:     strings.TrimSpace(label),
		Street:    strings.TrimSpace(street),
		City:      strings.TrimSpace(city),
		State:     strings.TrimSpace(state),
		ZipCode:   zip,
		Country:   strings.ToUpper(strings.TrimSpace(country)),
		IsDefault: false,
	}, nil
}

// AddAddress adds a new address to the profile.
func (p *UserProfile) AddAddress(name, street, city, state, zipCode, country string, setAsDefault bool) (*Address, error) {
	addr, err := newAddress(name, street, city, state, zipCode, country)
	if err != nil {
		return nil, err
	}
	addr.ProfileID = p.ID

	// If this is the first address or marked as default, update default.
	if len(p.Addresses) == 0 || setAsDefault {
		for i := range p.Addresses {
			p.Addresses[i].IsDefault = false
		}
		addr.IsDefault = true
	}

	p.Addresses = append(p.Addresses, addr)
	p.Touch()
	return &addr, nil
}

func (p *UserProfile) UpdateAddress(addressID AddressID, name, street, city, state, zipCode, country string) error {
	for i := range p.Addresses {
		if p.Addresses[i].ID != addressID {
			continue
		}

		updated, err := newAddress(name, street, city, state, zipCode, country)
		if err != nil {
			return err
		}

		p.Addresses[i].Label = updated.Label
		p.Addresses[i].Street = updated.Street
		p.Addresses[i].City = updated.City
		p.Addresses[i].State = updated.State
		p.Addresses[i].ZipCode = updated.ZipCode
		p.Addresses[i].Country = updated.Country
		// Preserve default flag.
		p.Touch()
		return nil
	}
	return fmt.Errorf("address not found")
}

func (p *UserProfile) DeleteAddress(addressID AddressID) error {
	idx := -1
	wasDefault := false
	for i := range p.Addresses {
		if p.Addresses[i].ID == addressID {
			idx = i
			wasDefault = p.Addresses[i].IsDefault
			break
		}
	}
	if idx == -1 {
		return fmt.Errorf("address not found")
	}

	p.Addresses = append(p.Addresses[:idx], p.Addresses[idx+1:]...)
	if wasDefault && len(p.Addresses) > 0 {
		for i := range p.Addresses {
			p.Addresses[i].IsDefault = false
		}
		p.Addresses[0].IsDefault = true
	}
	p.Touch()
	return nil
}

// SetDefaultAddress marks an address as the default.
func (p *UserProfile) SetDefaultAddress(addressID AddressID) error {
	found := false
	for i := range p.Addresses {
		if p.Addresses[i].ID == addressID {
			p.Addresses[i].IsDefault = true
			found = true
		} else {
			p.Addresses[i].IsDefault = false
		}
	}
	if !found {
		return errors.New("address not found")
	}
	p.Touch()
	return nil
}

func (p *UserProfile) Touch() {
	p.SetUpdated()
	p.IncrementVersion()
	p.AddDomainEvent(&ProfileUpdatedEvent{
		BaseDomainEvent: bbdomain.NewBaseDomainEvent(),
		ProfileID:       p.ID.Value(),
		UserID:          p.UserID,
	})
}

// ─── Domain Events ───────────────────────────────────────────────────────────

type ProfileCreatedEvent struct {
	bbdomain.BaseDomainEvent
	ProfileID uuid.UUID `json:"profileId"`
	UserID    string    `json:"userId"`
}

func (e *ProfileCreatedEvent) EventType() string { return "profiles.profile.created" }

type ProfileUpdatedEvent struct {
	bbdomain.BaseDomainEvent
	ProfileID uuid.UUID `json:"profileId"`
	UserID    string    `json:"userId"`
}

func (e *ProfileUpdatedEvent) EventType() string { return "profiles.profile.updated" }
