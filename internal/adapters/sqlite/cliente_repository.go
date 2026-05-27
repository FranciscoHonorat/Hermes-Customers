package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/FranciscoHonorat/mundo-invest/internal/core/domain"
	_ "modernc.org/sqlite"
)

// DB é o handle compartilhado do banco SQLite.
type DB struct {
	db *sql.DB
}

// NewDB abre (ou cria) o arquivo SQLite e cria as tabelas se necessário.
func NewDB(dsn string) (*DB, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("sqlite: abrir banco: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("sqlite: ping: %w", err)
	}
	repo := &DB{db: db}
	if err := repo.migrate(); err != nil {
		return nil, err
	}
	return repo, nil
}

func (r *DB) migrate() error {
	_, err := r.db.Exec(`
		CREATE TABLE IF NOT EXISTS clientes (
			id               INTEGER PRIMARY KEY AUTOINCREMENT,
			nome             TEXT    NOT NULL,
			email            TEXT    NOT NULL UNIQUE,
			tipo_solicitacao TEXT    NOT NULL,
			valor_patrimonio REAL    NOT NULL,
			status           TEXT    NOT NULL DEFAULT 'Aguardando Análise',
			prioridade       TEXT    NOT NULL DEFAULT ''
		);

		CREATE TABLE IF NOT EXISTS webhook_events (
			id            INTEGER PRIMARY KEY AUTOINCREMENT,
			event_id      TEXT    NOT NULL UNIQUE,
			card_id       TEXT    NOT NULL,
			cliente_email TEXT    NOT NULL,
			timestamp     TEXT    NOT NULL,
			processado    INTEGER NOT NULL DEFAULT 1
		);
	`)
	return err
}

// ---------------------------------------------------------------------------
// ClienteRepository
// ---------------------------------------------------------------------------

// Salvar insere um novo cliente no banco.
func (r *DB) Salvar(ctx context.Context, c *domain.Cliente) (*domain.Cliente, error) {
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO clientes (nome, email, tipo_solicitacao, valor_patrimonio, status, prioridade)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		c.Nome, c.Email, c.TipoSolicitacao, c.ValorPatrimonio, string(c.Status), string(c.Prioridade),
	)
	if err != nil {
		return nil, fmt.Errorf("sqlite: salvar cliente: %w", err)
	}
	id, _ := res.LastInsertId()
	c.ID = id
	return c, nil
}

// BuscarPorEmail retorna o cliente com o e-mail informado.
func (r *DB) BuscarPorEmail(ctx context.Context, email string) (*domain.Cliente, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, nome, email, tipo_solicitacao, valor_patrimonio, status, prioridade
		 FROM clientes WHERE email = ?`, email)

	var c domain.Cliente
	var status, prioridade string
	err := row.Scan(&c.ID, &c.Nome, &c.Email, &c.TipoSolicitacao, &c.ValorPatrimonio, &status, &prioridade)
	if err == sql.ErrNoRows {
		return nil, domain.ErrClienteNaoEncontrado
	}
	if err != nil {
		return nil, fmt.Errorf("sqlite: buscar cliente: %w", err)
	}
	c.Status = domain.StatusCliente(status)
	c.Prioridade = domain.Prioridade(prioridade)
	return &c, nil
}

// Atualizar persiste status e prioridade do cliente.
func (r *DB) Atualizar(ctx context.Context, c *domain.Cliente) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE clientes SET status = ?, prioridade = ? WHERE id = ?`,
		string(c.Status), string(c.Prioridade), c.ID,
	)
	if err != nil {
		return fmt.Errorf("sqlite: atualizar cliente: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// WebhookEventRepository
// ---------------------------------------------------------------------------

// EventoJaProcessado verifica se o event_id já existe no banco.
func (r *DB) EventoJaProcessado(ctx context.Context, eventID string) (bool, error) {
	var count int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(1) FROM webhook_events WHERE event_id = ?`, eventID,
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("sqlite: verificar evento: %w", err)
	}
	return count > 0, nil
}

// RegistrarEvento persiste o evento processado.
func (r *DB) RegistrarEvento(ctx context.Context, event *domain.WebhookEvent) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO webhook_events (event_id, card_id, cliente_email, timestamp, processado)
		 VALUES (?, ?, ?, ?, 1)`,
		event.EventID, event.CardID, event.ClienteEmail, event.Timestamp.Format("2006-01-02T15:04:05Z"),
	)
	if err != nil {
		return fmt.Errorf("sqlite: registrar evento: %w", err)
	}
	return nil
}
