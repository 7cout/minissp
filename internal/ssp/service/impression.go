package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/7cout/minissp/internal/ssp/domain"
)

// Impression обрабатывает подтверждение показа.
//
// Идемпотентно: параллельные или повторные вызовы с тем же auction_id
// возвращают nil, повторного списания не будет.
//
// ВАЖНО: этот метод выполняет две мутации в разных сервисах — Commit
// в DSP и AddBalance у publisher'а. Между ними нет транзакции, поэтому
// используется предварительная проверка publisher'а. Если AddBalance
// всё-таки упал — пишем CRITICAL, метку processed не снимаем (повторный
// Impression не поможет, только вызовет второй Commit в DSP).
//
// При переходе на PostgreSQL весь блок от Commit до AddBalance станет
// одной транзакцией: BEGIN; UPDATE publishers; ...; COMMIT — и предварительная
// проверка станет не нужна.
func (s *Service) Impression(ctx context.Context, auctionID string) error {
	record, alreadyProcessed, err := s.takeForImpression(auctionID)
	if err != nil {
		return err
	}
	if alreadyProcessed {
		return nil
	}

	// Предварительная проверка: publisher существует. Отсекает ошибку
	// AddBalance до того, как мы дёрнули DSP. Не закрывает race,
	// но в сочетании с CRITICAL-логом ниже даёт приемлемую гарантию.
	if _, err := s.publishers.Get(ctx, record.PublisherID); err != nil {
		s.unmarkProcessed(auctionID)
		s.storeAuction(record)
		return fmt.Errorf("get publisher %s: %w", record.PublisherID, err)
	}

	if err := s.commitToBidder(ctx, record); err != nil {
		// Снимаем метку и возвращаем запись, чтобы можно было повторить.
		s.unmarkProcessed(auctionID)
		s.storeAuction(record)
		return fmt.Errorf("commit to bidder %s: %w", record.BidderID, err)
	}

	// Начисляем publisher'у долю.
	publisherShare := record.Price * (100 - CommissionPercent) / 100
	if err := s.publishers.AddBalance(ctx, record.PublisherID, publisherShare); err != nil {
		// Коммит в DSP уже прошёл — деньги списаны, обратной операции
		// у нас нет. Логируем как CRITICAL: это состояние требует
		// ручного вмешательства или reconciliation-джобы.
		//
		// Метку processed НЕ снимаем: повторный Impression должен
		// вернуть nil, иначе мы попробуем ещё раз списать с DSP.
		slog.ErrorContext(ctx, "CRITICAL: commit succeeded but publisher balance not credited",
			"auction_id", auctionID,
			"publisher_id", record.PublisherID,
			"campaign_id", record.CampaignID,
			"price", record.Price,
			"publisher_share", publisherShare,
			"error", err,
		)
		return fmt.Errorf("add publisher balance: %w", err)
	}

	return nil
}

// commitToBidder отправляет Commit конкретному биддеру.
func (s *Service) commitToBidder(ctx context.Context, record domain.AuctionRecord) error {
	bidder, ok := s.biddersBy[record.BidderID]
	if !ok {
		return fmt.Errorf("bidder %q not found", record.BidderID)
	}
	return bidder.Commit(ctx, record.CampaignID, record.Price)
}

// GetPublisherBalance возвращает баланс издателя.
func (s *Service) GetPublisherBalance(ctx context.Context, publisherID string) (int64, error) {
	p, err := s.publishers.Get(ctx, publisherID)
	if err != nil {
		return 0, err
	}
	return p.Balance, nil
}
