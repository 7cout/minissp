package postgres

import (
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/7cout/minissp/internal/auction/domain"
)

// pgCodeInvalidTextRepresentation — код ошибки PostgreSQL
// «invalid input syntax for type X» (например, невалидный UUID).
const pgCodeInvalidTextRepresentation = "22P02"

// mapPgError превращает инфраструктурные ошибки PostgreSQL
// в доменные. Возвращает исходную ошибку, если не распознана.
//
// notFound — доменная ошибка для случая «строка не найдена».
// Для методов, которые не ожидают ErrNoRows, можно передать nil.
func mapPgError(err, notFound error) error {
	if notFound != nil && errors.Is(err, pgx.ErrNoRows) {
		return notFound
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgCodeInvalidTextRepresentation {
		return domain.ErrInvalidID
	}

	return err
}
