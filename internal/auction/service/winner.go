package service

import "github.com/7cout/minissp/internal/auction/domain"

// selectWinner выбирает победителя аукциона по модели second-price
//
// Победитель - ставка с максимальной ценой. Цена, которую он платит -
// ставка второго по величине участника. Если ставка одна - платит
// bidfloor слота
//
// При равенстве ставок побеждает первая в слайсе (first-arrived wins)
//
// Если ставок нет - возвращает domain.ErrNoBids.
func selectWinner(bids []domain.Bid, bidFloor int64) (domain.Bid, int64, error) {
	if len(bids) == 0 {
		return domain.Bid{}, 0, domain.ErrNoBids
	}

	// Находим победителя и второго по величине
	var (
		winner     = bids[0]
		secondBest int64
	)

	for _, b := range bids[1:] {
		switch {
		case b.Price > winner.Price:
			// Новый лидер. Прежний лидер становится вторым
			secondBest = winner.Price
			winner = b
		case b.Price > secondBest:
			// Не лидер, но лучше второго.
			secondBest = b.Price
		}
	}

	// Цена: ставка второго. Если второго нет - bidfloor
	price := secondBest
	if price == 0 {
		price = bidFloor
	}

	// Цена не может быть ниже bidfloor
	if price < bidFloor {
		price = bidFloor
	}

	return winner, price, nil
}
