package records

import (
	"context"
	"fmt"

	recordsdomain "github.com/sohidul/dns-server/internal/modules/records/domain"
)

// RecordDTO is the application-layer representation of a record, decoupling the
// transport layer from the domain value objects.
type RecordDTO struct {
	Domain string
	IP     string
}

type Service struct {
	repo     recordsdomain.Repository
	notifier recordsdomain.Notifier
}

func NewService(repo recordsdomain.Repository, notifier recordsdomain.Notifier) *Service {
	return &Service{repo: repo, notifier: notifier}
}

func (s *Service) List(ctx context.Context) ([]RecordDTO, error) {
	records, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]RecordDTO, 0, len(records))
	for _, r := range records {
		out = append(out, RecordDTO{Domain: r.Domain().String(), IP: r.IP().String()})
	}
	return out, nil
}

func (s *Service) Add(ctx context.Context, domain, ip string) error {
	record, err := recordsdomain.New(domain, ip)
	if err != nil {
		return err
	}
	if err := s.repo.Save(ctx, record); err != nil {
		return err
	}
	return s.notifier.Notify(
		ctx,
		"success",
		"DNS Record Created",
		fmt.Sprintf("Custom DNS record created for %s pointing to %s.", record.Domain(), record.IP()),
	)
}

func (s *Service) Delete(ctx context.Context, domain string) error {
	d, err := recordsdomain.NewDomain(domain)
	if err != nil {
		return err
	}
	if err := s.repo.Delete(ctx, d); err != nil {
		return err
	}
	return s.notifier.Notify(
		ctx,
		"info",
		"DNS Record Deleted",
		fmt.Sprintf("Custom DNS record for %s has been deleted.", d),
	)
}
