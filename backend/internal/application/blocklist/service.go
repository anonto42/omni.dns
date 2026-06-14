package blocklist

import (
	"context"
	"fmt"
	"time"

	blocklistdomain "github.com/sohidul/dns-server/internal/domain/blocklist"
)

// EntryDTO is the application-layer representation of a blocklist entry,
// decoupling the transport layer from the domain value objects.
type EntryDTO struct {
	Domain   string
	Wildcard bool
	AddedAt  time.Time
}

type Service struct {
	repo     blocklistdomain.Repository
	notifier blocklistdomain.Notifier
}

func NewService(repo blocklistdomain.Repository, notifier blocklistdomain.Notifier) *Service {
	return &Service{repo: repo, notifier: notifier}
}

func (s *Service) List(ctx context.Context) ([]EntryDTO, error) {
	entries, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]EntryDTO, 0, len(entries))
	for _, e := range entries {
		out = append(out, EntryDTO{
			Domain:   e.Domain().String(),
			Wildcard: e.Wildcard(),
			AddedAt:  e.AddedAt(),
		})
	}
	return out, nil
}

func (s *Service) Add(ctx context.Context, domain string, wildcard bool) error {
	entry, err := blocklistdomain.New(domain, wildcard)
	if err != nil {
		return err
	}
	if err := s.repo.Save(ctx, entry); err != nil {
		return err
	}
	return s.notifier.Notify(
		ctx,
		"warning",
		"Domain Blocked",
		fmt.Sprintf("Domain %s added to local blocklist (Wildcard: %t).", entry.Domain(), entry.Wildcard()),
	)
}

func (s *Service) Remove(ctx context.Context, domain string) error {
	d, err := blocklistdomain.NewDomain(domain)
	if err != nil {
		return err
	}
	if err := s.repo.Delete(ctx, d); err != nil {
		return err
	}
	return s.notifier.Notify(
		ctx,
		"success",
		"Domain Unblocked",
		fmt.Sprintf("Domain %s removed from local blocklist.", d),
	)
}
