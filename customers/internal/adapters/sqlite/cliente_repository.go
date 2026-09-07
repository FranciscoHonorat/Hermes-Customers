package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/FranciscoHonorat/mundo-invest/customers/internal/core/domain"
	_ "modernc.org/sqlite"
)

// busyTimeout é quanto uma conexão espera por um lock antes de desistir com
// SQLITE_BUSY. Sem isso, qualquer escrita concorrente falha imediatamente —
// medido na prática: >98% de erro 500 a partir de ~5 escritas simultâneas
// (ver docs/testing/resultados-2026-09-07.md).
const busyTimeoutMS = 5000

// DB é o handle compartilhado do banco SQLite.
type DB struct {
	db *sql.DB
}

// NewDB abre (ou cria) o arquivo SQLite e cria as tabelas se necessário.
//
// Configura WAL (permite leitores concorrentes durante uma escrita) e
// busy_timeout (uma segunda escrita espera até 5s pelo lock em vez de
// falhar na hora com "database is locked"). SQLite só admite um escritor
// por vez de qualquer forma — por isso o pool fica limitado a 1 conexão:
// sem isso, múltiplas conexões do database/sql tentando escrever ao mesmo
// tempo ainda disputam o mesmo lock de arquivo entre si.
func NewDB(dsn string) (*DB, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("sqlite: abrir banco: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("sqlite: ping: %w", err)
	}

	db.SetMaxOpenConns(1)

	if _, err := db.Exec("PRAGMA journal_mode=WAL;"); err != nil {
		return nil, fmt.Errorf("sqlite: configurar journal_mode=WAL: %w", err)
	}
	if _, err := db.Exec(fmt.Sprintf("PRAGMA busy_timeout=%d;", busyTimeoutMS)); err != nil {
		return nil, fmt.Errorf("sqlite: configurar busy_timeout: %w", err)
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

// isUniqueConstraintError detecta violação de constraint UNIQUE do SQLite,
// independente da mensagem exata do driver (modernc.org/sqlite reporta como
// "UNIQUE constraint failed: <tabela>.<coluna>").
func isUniqueConstraintError(err error) bool {
	return strings.Contains(err.Error(), "UNIQUE constraint failed")
}

// isBusyError detecta SQLITE_BUSY (lock não liberado dentro do busy_timeout
// configurado em NewDB) — com MaxOpenConns(1) isso deveria ser raro (as
// chamadas já ficam na fila do pool do database/sql antes de chegar no
// driver), mas pode acontecer se outro processo tiver o mesmo arquivo aberto.
func isBusyError(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "database is locked") || strings.Contains(msg, "SQLITE_BUSY")
}

// mapWriteError traduz um erro cru do driver para um erro de domínio quando
// reconhecido, preservando o erro original (com %w) nos demais casos para
// que o chamador possa logar a causa raiz antes de responder genericamente.
func mapWriteError(op string, err error) error {
	switch {
	case isUniqueConstraintError(err):
		return domain.ErrEmailDuplicado
	case isBusyError(err):
		return domain.ErrBancoIndisponivel
	default:
		return fmt.Errorf("sqlite: %s: %w", op, err)
	}
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
		return nil, mapWriteError("salvar cliente", err)
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
		if isBusyError(err) {
			return nil, domain.ErrBancoIndisponivel
		}
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
		return mapWriteError("atualizar cliente", err)
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
		if isBusyError(err) {
			return false, domain.ErrBancoIndisponivel
		}
		return false, fmt.Errorf("sqlite: verificar evento: %w", err)
	}
	return count > 0, nil
}

// RegistrarEvento persiste o evento processado. A constraint UNIQUE aqui é
// em event_id (idempotência), não email — por isso não usa mapWriteError
// (que mapeia UNIQUE para ErrEmailDuplicado, que seria enganoso neste
// contexto). O caminho normal de idempotência é EventoJaProcessado, checado
// antes desta chamada pelo service; uma violação UNIQUE aqui indicaria uma
// corrida entre duas requisições concorrentes com o mesmo event_id.
func (r *DB) RegistrarEvento(ctx context.Context, event *domain.WebhookEvent) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO webhook_events (event_id, card_id, cliente_email, timestamp, processado)
		 VALUES (?, ?, ?, ?, 1)`,
		event.EventID, event.CardID, event.ClienteEmail, event.Timestamp.Format("2006-01-02T15:04:05Z"),
	)
	if err != nil {
		if isUniqueConstraintError(err) {
			return domain.ErrEventoDuplicado
		}
		if isBusyError(err) {
			return domain.ErrBancoIndisponivel
		}
		return fmt.Errorf("sqlite: registrar evento: %w", err)
	}
	return nil
}
