package repository

import (
	"context"
	"database/sql"
	"log"
	"log/slog"
	"os"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
	"github.com/valkey-io/valkey-go"

	"backend/common"
	"backend/onlineconversation/internal/dto"
	"backend/onlineconversation/internal/projection"

	_ "github.com/joho/godotenv/autoload"
)

// Session runs queries, both *sql.DB and *sql.Tx satisfy it
type Session interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// Tx is a transaction started by BeginTx, *sql.Tx satisfies it
type Tx interface {
	Session
	Commit() error
	Rollback() error
}

type Repository interface {
	FindParticipantIds(ctx context.Context, conversationId string) ([]string, error)
	AddParticipantId(ctx context.Context, conversationId string, memberId uuid.UUID) error
	RemoveParticipantId(ctx context.Context, conversationId string, memberId uuid.UUID) error
	SetServerIP(ctx context.Context, memberId, ip string) error
	RemoveServerIP(ctx context.Context, memberId string) error
	InsertConversation(ctx context.Context, session Session, conversationId uuid.UUID, req dto.CreateConversationRequest) error
	UpdateConversationIfModerator(ctx context.Context, session Session, memberId uuid.UUID, req dto.UpdateConversationRequest) (bool, error)
	DeleteOnlineConversationIfModerator(ctx context.Context, session Session, conversationId, memberId uuid.UUID) (bool, error)
	AddBanIdIfModerator(ctx context.Context, session Session, conversationId, modId, banId uuid.UUID) (bool, error)
	InsertModerator(ctx context.Context, session Session, conversationId, memberId uuid.UUID) error
	InsertRegistrant(ctx context.Context, session Session, conversationId, memberId uuid.UUID) error
	AddNotificationId(ctx context.Context, session Session, conversationId, memberId uuid.UUID) error
	FindConversationDetail(ctx context.Context, session Session, conversationId, memberId uuid.UUID) (projection.Detail, error)
	TryIncrementRegistrants(ctx context.Context, session Session, conversationId uuid.UUID) (bool, error)
	DecrementRegistrants(ctx context.Context, session Session, conversationId uuid.UUID) error
	RemoveRegistrantId(ctx context.Context, session Session, conversationId, memberId uuid.UUID) error
	RemoveNotificationId(ctx context.Context, session Session, conversationId, memberId uuid.UUID) error
	FindNotificationIds(ctx context.Context, session Session, conversationId uuid.UUID) ([]uuid.UUID, error)
	InsertOutbox(ctx context.Context, session Session, id, conversationId uuid.UUID, topic, taskType string, payload []byte) error
	DeleteOutbox(ctx context.Context, session Session, id, conversationId uuid.UUID) error
	BeginTx(ctx context.Context) (Tx, error)
	Tx() Session
	FindConversationContents(ctx context.Context, tx Tx, id uuid.UUID) (projection.Contents, error)
}

type repository struct {
	db           *sql.DB
	valkeyClient valkey.Client
}

func NewRepository() Repository {
	mysqlConfig := mysql.Config{
		User:                 os.Getenv("MYSQL_USERNAME"),
		Passwd:               os.Getenv("MYSQL_PASSWORD"),
		Net:                  "tcp",
		Addr:                 os.Getenv("MYSQL_URL"),
		DBName:               "conversation",
		ParseTime:            true,
		AllowNativePasswords: true,
	}
	db, err := sql.Open("mysql", mysqlConfig.FormatDSN())
	if err != nil {
		log.Panicf("fail to open mysql connection: %v", err)
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)

	pingCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		log.Panicf("fail to ping mysql: %v", err)
	}
	slog.Info("success to connect mysql")

	clientOption := valkey.ClientOption{
		InitAddress: []string{os.Getenv("VALKEY_ADDRESS")},
	}

	tlsConfig, err := common.CreateTlSConfig("", "", os.Getenv("VALKEY_CA_CERT_PATH"))
	if err != nil {
		log.Panicf("fail to create tls config for valkey")
	}
	tlsConfig.ServerName = os.Getenv("VALKEY_HOST")
	clientOption.TLSConfig = tlsConfig

	clientOption.Username = os.Getenv("VALKEY_USERNAME")
	clientOption.Password = os.Getenv("VALKEY_PASSWORD")
	v, err := valkey.NewClient(clientOption)
	if err != nil {
		log.Panicf("fail to connect to redis: %v", err)
	}

	return &repository{
		db:           db,
		valkeyClient: v,
	}
}

func (r *repository) BeginTx(ctx context.Context) (Tx, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		slog.Error("fail to start transaction",
			"err", err)
		return nil, err
	}
	return tx, nil
}

func (r *repository) Tx() Session {
	return r.db
}
