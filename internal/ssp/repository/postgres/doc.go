// Package postgres содержит реализации репозиториев SSP
// поверх PostgreSQL через pgx/v5.
//
// Использует raw SQL: атомарные UPDATE ... WHERE ... RETURNING
// и INSERT ... ON CONFLICT. ORM не используем.
package postgres
