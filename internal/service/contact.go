// Package service is the business layer: validation, normalisation and
// orchestration. It depends only on interfaces, never on pgx/Kafka/HTTP.
package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/mail"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/LDAi0/phonebook/internal/domain"
)

const (
	DefaultLimit   = 20
	MaxLimit       = 100
	maxNameLen     = 100
	publishTimeout = 3 * time.Second
)

var (
	phoneRe      = regexp.MustCompile(`^\+?[0-9]{7,15}$`)
	phoneCleaner = strings.NewReplacer(" ", "", "-", "", "(", "", ")", "", ".", "")
)

// ContactRepository is implemented by the data layer.
type ContactRepository interface {
	Create(ctx context.Context, c *domain.Contact) error
	GetByID(ctx context.Context, id int64) (*domain.Contact, error)
	List(ctx context.Context, f domain.ListFilter) ([]domain.Contact, int, error)
	Update(ctx context.Context, c *domain.Contact) error
	Delete(ctx context.Context, id int64) error
}

// EventPublisher is implemented by the Kafka adapter.
type EventPublisher interface {
	Publish(ctx context.Context, evt domain.ContactEvent) error
}

type ContactInput struct {
	FirstName string
	LastName  string
	Phone     string
	Email     string
}

type ContactService struct {
	repo ContactRepository
	pub  EventPublisher
	log  *slog.Logger
}

func NewContactService(repo ContactRepository, pub EventPublisher, log *slog.Logger) *ContactService {
	return &ContactService{repo: repo, pub: pub, log: log}
}

func (s *ContactService) Create(ctx context.Context, in ContactInput) (*domain.Contact, error) {
	in, err := normalize(in)
	if err != nil {
		return nil, err
	}
	c := &domain.Contact{FirstName: in.FirstName, LastName: in.LastName, Phone: in.Phone, Email: in.Email}
	if err := s.repo.Create(ctx, c); err != nil {
		return nil, err
	}
	s.publish(ctx, domain.EventContactCreated, c.ID, c)
	return c, nil
}

func (s *ContactService) Get(ctx context.Context, id int64) (*domain.Contact, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *ContactService) List(ctx context.Context, f domain.ListFilter) ([]domain.Contact, int, error) {
	f.Query = strings.TrimSpace(f.Query)
	if f.Limit <= 0 {
		f.Limit = DefaultLimit
	}
	if f.Limit > MaxLimit {
		f.Limit = MaxLimit
	}
	if f.Offset < 0 {
		f.Offset = 0
	}
	return s.repo.List(ctx, f)
}

func (s *ContactService) Update(ctx context.Context, id int64, in ContactInput) (*domain.Contact, error) {
	in, err := normalize(in)
	if err != nil {
		return nil, err
	}
	c := &domain.Contact{ID: id, FirstName: in.FirstName, LastName: in.LastName, Phone: in.Phone, Email: in.Email}
	if err := s.repo.Update(ctx, c); err != nil {
		return nil, err
	}
	s.publish(ctx, domain.EventContactUpdated, c.ID, c)
	return c, nil
}

func (s *ContactService) Delete(ctx context.Context, id int64) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	s.publish(ctx, domain.EventContactDeleted, id, nil)
	return nil
}

// publish is best-effort: the database is the source of truth, so a Kafka
// outage is logged but does not fail the request. (For guaranteed delivery
// use the transactional outbox pattern, see README.)
func (s *ContactService) publish(ctx context.Context, t domain.EventType, id int64, c *domain.Contact) {
	evt := domain.ContactEvent{
		ID:         newEventID(),
		Type:       t,
		ContactID:  id,
		Contact:    c,
		OccurredAt: time.Now().UTC(),
	}
	// Detach from the request context: the client may disconnect right after the commit.
	pctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), publishTimeout)
	defer cancel()
	if err := s.pub.Publish(pctx, evt); err != nil {
		s.log.Error("publish event failed", "event_type", t, "contact_id", id, "error", err)
	}
}

func normalize(in ContactInput) (ContactInput, error) {
	in.FirstName = strings.TrimSpace(in.FirstName)
	in.LastName = strings.TrimSpace(in.LastName)
	in.Email = strings.TrimSpace(in.Email)
	in.Phone = phoneCleaner.Replace(strings.TrimSpace(in.Phone))

	fields := map[string]string{}
	switch {
	case in.FirstName == "":
		fields["first_name"] = "is required"
	case utf8.RuneCountInString(in.FirstName) > maxNameLen:
		fields["first_name"] = "must be at most 100 characters"
	}
	if utf8.RuneCountInString(in.LastName) > maxNameLen {
		fields["last_name"] = "must be at most 100 characters"
	}
	if !phoneRe.MatchString(in.Phone) {
		fields["phone"] = "must contain 7-15 digits, optionally starting with +"
	}
	if in.Email != "" {
		addr, err := mail.ParseAddress(in.Email)
		if err != nil || addr.Address != in.Email {
			fields["email"] = "is not a valid email address"
		}
	}
	if len(fields) > 0 {
		return in, &domain.ValidationError{Fields: fields}
	}
	return in, nil
}

func newEventID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
