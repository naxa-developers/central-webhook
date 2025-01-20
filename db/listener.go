// Code by @brojonat
// https://github.com/brojonat/notifier
// Idea by @brandur

package db

import (
	"context"
	"errors"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Listener interface connects to the database and allows callers to listen to a
// particular topic by issuing a LISTEN command. WaitForNotification blocks
// until receiving a notification or until the supplied context expires. The
// default implementation is tightly coupled to pgx (following River's
// implementation), but callers may implement their own listeners for any
// backend they'd like.
type Listener interface {
	Close(ctx context.Context) error
	Connect(ctx context.Context) error
	Listen(ctx context.Context, topic string) error
	Ping(ctx context.Context) error
	Unlisten(ctx context.Context, topic string) error
	WaitForNotification(ctx context.Context) (*Notification, error)
}

// NewListener return a Listener that draws a connection from the supplied Pool. This
// is somewhat discouraged
func NewListener(dbPool *pgxpool.Pool) Listener {
	return &listener{
		mu:     sync.Mutex{},
		dbPool: dbPool,
	}
}

type listener struct {
	conn   *pgxpool.Conn
	dbPool *pgxpool.Pool
	mu     sync.Mutex
}

// Close the connection to the database.
func (listener *listener) Close(ctx context.Context) error {
	listener.mu.Lock()
	defer listener.mu.Unlock()

	if listener.conn == nil {
		return nil
	}

	// Release below would take care of cleanup and potentially put the
	// connection back into rotation, but in case a Listen was invoked without a
	// subsequent Unlisten on the same topic, close the connection explicitly to
	// guarantee no other caller will receive a partially tainted connection.
	err := listener.conn.Conn().Close(ctx)

	// Even in the event of an error, make sure conn is set back to nil so that
	// the listener can be reused.
	listener.conn.Release()
	listener.conn = nil

	return err
}

// Connect to the database.
func (listener *listener) Connect(ctx context.Context) error {
	listener.mu.Lock()
	defer listener.mu.Unlock()

	if listener.conn != nil {
		return errors.New("connection already established")
	}

	conn, err := listener.dbPool.Acquire(ctx)
	if err != nil {
		return err
	}

	listener.conn = conn
	return nil
}

// Listen issues a LISTEN command for the supplied topic.
func (listener *listener) Listen(ctx context.Context, topic string) error {
	listener.mu.Lock()
	defer listener.mu.Unlock()

	_, err := listener.conn.Exec(ctx, "LISTEN \""+topic+"\"")
	return err
}

// Ping the database
func (listener *listener) Ping(ctx context.Context) error {
	listener.mu.Lock()
	defer listener.mu.Unlock()

	return listener.conn.Ping(ctx)
}

// Unlisten issues an UNLISTEN from the supplied topic.
func (listener *listener) Unlisten(ctx context.Context, topic string) error {
	listener.mu.Lock()
	defer listener.mu.Unlock()

	_, err := listener.conn.Exec(ctx, "UNLISTEN \""+topic+"\"")
	return err
}

// WaitForNotification blocks until receiving a notification and returns it. The
// pgx driver should maintain a buffer of notifications, so as long as Listen
// has been called, repeatedly calling WaitForNotification should yield all
// notifications.
func (listener *listener) WaitForNotification(ctx context.Context) (*Notification, error) {
	listener.mu.Lock()
	defer listener.mu.Unlock()

	pgn, err := listener.conn.Conn().WaitForNotification(ctx)

	if err != nil {
		return nil, err
	}

	n := Notification{
		Channel: pgn.Channel,
		Payload: []byte(pgn.Payload),
	}

	return &n, nil
}
