package sessions

import (
	"context"
	"fmt"

	"github.com/hashicorp/boundary/internal/db"
	"github.com/hashicorp/boundary/internal/sessions/store"
	"google.golang.org/protobuf/proto"
)

const (
	DefaultSessionTableName = "session"
)

type Session struct {
	*store.Session
	tableName string `gorm:"-"`
}

var _ Cloneable = (*Session)(nil)
var _ db.VetForWriter = (*Session)(nil)

// NewSession creates a new in memory session.  No options
// are currently supported.
func NewSession(
	userId,
	hostId,
	serverId,
	serverType,
	targetId,
	hostSetId,
	authTokenId,
	scopeId,
	address,
	port string,
	opt ...Option) (*Session, error) {
	s := Session{
		Session: &store.Session{
			UserId:      userId,
			HostId:      hostId,
			ServerId:    serverId,
			ServerType:  serverType,
			TargetId:    targetId,
			SetId:       hostSetId,
			AuthTokenId: authTokenId,
			ScopeId:     scopeId,
			Address:     address,
			Port:        port,
		},
	}

	if err := validateNewSession(&s, "new session:"); err != nil {
		return nil, err
	}
	return &s, nil
}

// allocSession will allocate a Session
func allocSession() Session {
	return Session{
		Session: &store.Session{},
	}
}

// Clone creates a clone of the Session
func (s *Session) Clone() interface{} {
	cp := proto.Clone(s.Session)
	return &Session{
		Session: cp.(*store.Session),
	}
}

// VetForWrite implements db.VetForWrite() interface and validates the session
// before it's written.
func (s *Session) VetForWrite(ctx context.Context, r db.Reader, opType db.OpType, opt ...db.Option) error {
	if s.PublicId == "" {
		return fmt.Errorf("session vet for write: missing public id: %w", db.ErrInvalidParameter)
	}
	switch opType {
	case db.CreateOp:
		if err := validateNewSession(s, "session vet for write:"); err != nil {
			return err
		}
	case db.UpdateOp:
		panic("not implemented")
	}
	return nil
}

// TableName returns the tablename to override the default gorm table name
func (s *Session) TableName() string {
	if s.tableName != "" {
		return s.tableName
	}
	return DefaultSessionTableName
}

// SetTableName sets the tablename and satisfies the ReplayableMessage
// interface. If the caller attempts to set the name to "" the name will be
// reset to the default name.
func (s *Session) SetTableName(n string) {
	s.tableName = n
}

// validateNewSession checks everything but the session's PublicId
func validateNewSession(s *Session, errorPrefix string) error {
	if s.UserId == "" {
		return fmt.Errorf("%s missing user id: %w", errorPrefix, db.ErrInvalidParameter)
	}
	if s.HostId == "" {
		return fmt.Errorf("%s missing host id: %w", errorPrefix, db.ErrInvalidParameter)
	}
	if s.ServerId == "" {
		return fmt.Errorf("%s missing server id: %w", errorPrefix, db.ErrInvalidParameter)
	}
	if s.ServerType == "" {
		return fmt.Errorf("%s missing server type: %w", errorPrefix, db.ErrInvalidParameter)
	}
	if s.TargetId == "" {
		return fmt.Errorf("%s missing target id: %w", errorPrefix, db.ErrInvalidParameter)
	}
	if s.SetId == "" {
		return fmt.Errorf("%s missing host set id: %w", errorPrefix, db.ErrInvalidParameter)
	}
	if s.AuthTokenId == "" {
		return fmt.Errorf("%s missing auth token id: %w", errorPrefix, db.ErrInvalidParameter)
	}
	if s.ScopeId == "" {
		return fmt.Errorf("%s missing scope id: %w", errorPrefix, db.ErrInvalidParameter)
	}
	if s.Address == "" {
		return fmt.Errorf("%s missing address: %w", errorPrefix, db.ErrInvalidParameter)
	}
	if s.Port == "" {
		return fmt.Errorf("%s missing port: %w", errorPrefix, db.ErrInvalidParameter)
	}
	return nil
}
