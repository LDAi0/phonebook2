package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/LDAi0/phonebook/internal/domain"
)

type fakeRepo struct {
	items      map[int64]domain.Contact
	next       int64
	lastFilter domain.ListFilter
}

func newFakeRepo() *fakeRepo { return &fakeRepo{items: map[int64]domain.Contact{}} }

func (f *fakeRepo) Create(_ context.Context, c *domain.Contact) error {
	for _, it := range f.items {
		if it.Phone == c.Phone {
			return domain.ErrPhoneTaken
		}
	}
	f.next++
	c.ID = f.next
	c.CreatedAt = time.Now()
	c.UpdatedAt = c.CreatedAt
	f.items[c.ID] = *c
	return nil
}

func (f *fakeRepo) GetByID(_ context.Context, id int64) (*domain.Contact, error) {
	it, ok := f.items[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return &it, nil
}

func (f *fakeRepo) List(_ context.Context, flt domain.ListFilter) ([]domain.Contact, int, error) {
	f.lastFilter = flt
	return []domain.Contact{}, 0, nil
}

func (f *fakeRepo) Update(_ context.Context, c *domain.Contact) error {
	if _, ok := f.items[c.ID]; !ok {
		return domain.ErrNotFound
	}
	f.items[c.ID] = *c
	return nil
}

func (f *fakeRepo) Delete(_ context.Context, id int64) error {
	if _, ok := f.items[id]; !ok {
		return domain.ErrNotFound
	}
	delete(f.items, id)
	return nil
}

type fakePublisher struct {
	events []domain.ContactEvent
	err    error
}

func (p *fakePublisher) Publish(_ context.Context, evt domain.ContactEvent) error {
	if p.err != nil {
		return p.err
	}
	p.events = append(p.events, evt)
	return nil
}

func newSvc() (*ContactService, *fakeRepo, *fakePublisher) {
	repo, pub := newFakeRepo(), &fakePublisher{}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewContactService(repo, pub, log), repo, pub
}

func TestCreate_NormalizesAndPublishes(t *testing.T) {
	svc, _, pub := newSvc()

	c, err := svc.Create(context.Background(), ContactInput{
		FirstName: "  Ada ", LastName: "Lovelace", Phone: "+372 5555-1234", Email: "ada@example.com",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.Phone != "+37255551234" || c.FirstName != "Ada" {
		t.Fatalf("not normalized: %+v", c)
	}
	if len(pub.events) != 1 || pub.events[0].Type != domain.EventContactCreated || pub.events[0].ContactID != c.ID {
		t.Fatalf("unexpected events: %+v", pub.events)
	}
}

func TestCreate_Validation(t *testing.T) {
	tests := []struct {
		name  string
		in    ContactInput
		field string
	}{
		{"empty first name", ContactInput{Phone: "1234567"}, "first_name"},
		{"short phone", ContactInput{FirstName: "A", Phone: "123"}, "phone"},
		{"letters in phone", ContactInput{FirstName: "A", Phone: "12345abc"}, "phone"},
		{"bad email", ContactInput{FirstName: "A", Phone: "1234567", Email: "not-an-email"}, "email"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, _, pub := newSvc()
			_, err := svc.Create(context.Background(), tt.in)
			var verr *domain.ValidationError
			if !errors.As(err, &verr) {
				t.Fatalf("expected ValidationError, got %v", err)
			}
			if _, ok := verr.Fields[tt.field]; !ok {
				t.Fatalf("expected error for field %q, got %v", tt.field, verr.Fields)
			}
			if len(pub.events) != 0 {
				t.Fatal("no event must be published for invalid input")
			}
		})
	}
}

func TestCreate_DuplicatePhone(t *testing.T) {
	svc, _, pub := newSvc()
	in := ContactInput{FirstName: "A", Phone: "1234567"}
	if _, err := svc.Create(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Create(context.Background(), in); !errors.Is(err, domain.ErrPhoneTaken) {
		t.Fatalf("expected ErrPhoneTaken, got %v", err)
	}
	if len(pub.events) != 1 {
		t.Fatalf("expected exactly 1 event, got %d", len(pub.events))
	}
}

func TestCreate_PublishFailureDoesNotFailRequest(t *testing.T) {
	svc, _, pub := newSvc()
	pub.err = errors.New("kafka down")
	if _, err := svc.Create(context.Background(), ContactInput{FirstName: "A", Phone: "1234567"}); err != nil {
		t.Fatalf("publish failure must not fail the request: %v", err)
	}
}

func TestUpdate_NotFound(t *testing.T) {
	svc, _, _ := newSvc()
	_, err := svc.Update(context.Background(), 42, ContactInput{FirstName: "A", Phone: "1234567"})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestDelete_PublishesEvent(t *testing.T) {
	svc, _, pub := newSvc()
	c, _ := svc.Create(context.Background(), ContactInput{FirstName: "A", Phone: "1234567"})
	if err := svc.Delete(context.Background(), c.ID); err != nil {
		t.Fatal(err)
	}
	last := pub.events[len(pub.events)-1]
	if last.Type != domain.EventContactDeleted || last.Contact != nil {
		t.Fatalf("unexpected delete event: %+v", last)
	}
}

func TestList_ClampsPaging(t *testing.T) {
	svc, repo, _ := newSvc()
	if _, _, err := svc.List(context.Background(), domain.ListFilter{Query: "  bob ", Limit: 10000, Offset: -5}); err != nil {
		t.Fatal(err)
	}
	if repo.lastFilter.Limit != MaxLimit || repo.lastFilter.Offset != 0 || repo.lastFilter.Query != "bob" {
		t.Fatalf("unexpected filter: %+v", repo.lastFilter)
	}
}
