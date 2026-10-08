package events

import (
	"encoding/json"
	"time"
)

// TopicImpression — топик Kafka для событий Impression.
const TopicImpression = "impression"

// ImpressionEvent — факт успешного Impression.
//
// Публикуется в Kafka после того, как:
//   - Commit в DSP прошёл (деньги списаны с advertiser'а),
//   - publisher.balance увеличен на PublisherShare,
//   - платформа удержала PlatformFee.
//
// Два поля времени: CreatedAt — когда состоялся аукцион (из
// AuctionRecord), OccurredAt — когда произошёл Impression. Разница
// полезна для аналитики: сколько живёт аукцион до подтверждения.
type ImpressionEvent struct {
	AuctionID      string    `json:"auction_id"`
	ImpID          string    `json:"imp_id"`
	CampaignID     string    `json:"campaign_id"`
	CreativeID     string    `json:"creative_id"`
	PublisherID    string    `json:"publisher_id"`
	SlotID         string    `json:"slot_id"`
	BidderID       string    `json:"bidder_id"`
	Price          int64     `json:"price"`
	PublisherShare int64     `json:"publisher_share"`
	PlatformFee    int64     `json:"platform_fee"`
	CreatedAt      time.Time `json:"created_at"`
	OccurredAt     time.Time `json:"occurred_at"`
}

// Topic возвращает имя топика Kafka для этого события.
func (e ImpressionEvent) Topic() string { return TopicImpression }

// Key возвращает ключ партиционирования.
//
// По auction_id: все события одного аукциона попадут в одну партицию.
// Сейчас это избыточно (у нас один тип события), но когда появятся
// click и auction_won — это обеспечит правильный порядок между ними.
func (e ImpressionEvent) Key() string { return e.AuctionID }

// Marshal сериализует событие в JSON.
func (e ImpressionEvent) Marshal() ([]byte, error) {
	return json.Marshal(e)
}
