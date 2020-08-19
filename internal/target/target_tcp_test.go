package target

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hashicorp/boundary/internal/db"
	dbassert "github.com/hashicorp/boundary/internal/db/assert"
	"github.com/hashicorp/boundary/internal/iam"
	"github.com/hashicorp/boundary/internal/kms"
	"github.com/hashicorp/boundary/internal/oplog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

// NOTE: there are no update tests since all the RootKey attributes are
// immutable and those tests are covered by TestRootKey_ImmutableFields

func TestTcpTarget_Create(t *testing.T) {
	t.Parallel()
	conn, _ := db.TestSetup(t, "postgres")
	wrapper := db.TestWrapper(t)
	org, prj := iam.TestScopes(t, iam.TestRepo(t, conn, wrapper))
	type args struct {
		scopeId string
		name    string
		opt     []Option
	}
	tests := []struct {
		name          string
		args          args
		want          *TcpTarget
		wantErr       bool
		wantIsErr     error
		create        bool
		wantCreateErr bool
	}{
		{
			name:      "empty-scopeId",
			args:      args{},
			wantErr:   true,
			wantIsErr: db.ErrInvalidParameter,
		},
		{
			name: "valid-org-config",
			args: args{
				scopeId: org.PublicId,
				name:    "valid-org-config",
				opt:     []Option{WithDescription("valid-org-config-description"), WithDefaultPort(uint32(22))},
			},
			want: func() *TcpTarget {
				t := allocTcpTarget()
				t.ScopeId = org.PublicId
				t.Name = "valid-org-config"
				t.Description = "valid-org-config-description"
				t.DefaultPort = uint32(22)
				return &t
			}(),
			create: true,
		},
		{
			name: "valid-proj-scope",
			args: args{
				scopeId: prj.PublicId,
				name:    "valid-proj-scope",
			},
			want: func() *TcpTarget {
				t := allocTcpTarget()
				t.ScopeId = prj.PublicId
				t.Name = "valid-proj-scope"
				return &t
			}(),
			create: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert, require := assert.New(t), require.New(t)
			require.NoError(conn.Where("1=1").Delete(kms.AllocRootKey()).Error)
			got, err := NewTcpTarget(tt.args.scopeId, tt.args.name, tt.args.opt...)
			if tt.wantErr {
				require.Error(err)
				assert.True(errors.Is(err, tt.wantIsErr))
				return
			}
			require.NoError(err)
			assert.Equal(tt.want, got)
			if tt.create {
				id, err := newTcpId()
				require.NoError(err)
				got.PublicId = id
				err = db.New(conn).Create(context.Background(), got)
				if tt.wantCreateErr {
					assert.Error(err)
					return
				} else {
					assert.NoError(err)
				}
			}
		})
	}
}

func TestTcpTarget_Delete(t *testing.T) {
	t.Parallel()
	conn, _ := db.TestSetup(t, "postgres")
	rw := db.New(conn)
	wrapper := db.TestWrapper(t)
	org, _ := iam.TestScopes(t, iam.TestRepo(t, conn, wrapper))

	tests := []struct {
		name            string
		target          *TcpTarget
		wantRowsDeleted int
		wantErr         bool
		wantErrMsg      string
	}{
		{
			name:            "valid",
			target:          TestTcpTarget(t, conn, org.PublicId, testTargetName(t, org.PublicId)),
			wantErr:         false,
			wantRowsDeleted: 1,
		},
		{
			name: "bad-id",
			target: func() *TcpTarget {
				target := allocTcpTarget()
				id, err := newTcpId()
				require.NoError(t, err)
				target.PublicId = id
				target.ScopeId = org.PublicId
				target.Name = testTargetName(t, org.PublicId)
				return &target
			}(),
			wantErr:         false,
			wantRowsDeleted: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert, require := assert.New(t), require.New(t)
			deleteTarget := allocTcpTarget()
			deleteTarget.PublicId = tt.target.PublicId
			deletedRows, err := rw.Delete(context.Background(), &deleteTarget)
			if tt.wantErr {
				require.Error(err)
				return
			}
			require.NoError(err)
			if tt.wantRowsDeleted == 0 {
				assert.Equal(tt.wantRowsDeleted, deletedRows)
				return
			}
			assert.Equal(tt.wantRowsDeleted, deletedRows)
			foundTarget := allocTcpTarget()
			foundTarget.PublicId = tt.target.PublicId
			err = rw.LookupById(context.Background(), &foundTarget)
			require.Error(err)
			assert.True(errors.Is(db.ErrRecordNotFound, err))
		})
	}
}

func TestTcpTarget_Update(t *testing.T) {
	t.Parallel()
	id := testId(t)
	conn, _ := db.TestSetup(t, "postgres")
	rw := db.New(conn)
	wrapper := db.TestWrapper(t)
	org, proj := iam.TestScopes(t, iam.TestRepo(t, conn, wrapper))

	type args struct {
		name           string
		description    string
		fieldMaskPaths []string
		nullPaths      []string
		ScopeId        string
	}
	tests := []struct {
		name           string
		args           args
		wantRowsUpdate int
		wantErr        bool
		wantErrMsg     string
		wantDup        bool
	}{
		{
			name: "valid",
			args: args{
				name:           "valid" + id,
				fieldMaskPaths: []string{"Name"},
				ScopeId:        org.PublicId,
			},
			wantErr:        false,
			wantRowsUpdate: 1,
		},
		{
			name: "proj-scope-id",
			args: args{
				name:           "proj-scope-id" + id,
				fieldMaskPaths: []string{"ScopeId"},
				ScopeId:        proj.PublicId,
			},
			wantErr:    true,
			wantErrMsg: "update: failed: pq: immutable column: target_tcp.scope_id",
		},
		{
			name: "proj-scope-id-not-in-mask",
			args: args{
				name:           "proj-scope-id" + id,
				fieldMaskPaths: []string{"Name"},
				ScopeId:        proj.PublicId,
			},
			wantErr:        false,
			wantRowsUpdate: 1,
		},
		{
			name: "empty-scope-id",
			args: args{
				name:           "empty-scope-id" + id,
				fieldMaskPaths: []string{"Name"},
				ScopeId:        "",
			},
			wantErr:        false,
			wantRowsUpdate: 1,
		},
		{
			name: "dup-name",
			args: args{
				name:           "dup-name" + id,
				fieldMaskPaths: []string{"Name"},
				ScopeId:        org.PublicId,
			},
			wantErr:    true,
			wantDup:    true,
			wantErrMsg: `update: failed: pq: duplicate key value violates unique constraint "target_tcp_scope_id_name_key"`,
		},
		{
			name: "set description null",
			args: args{
				name:           "set description null" + id,
				fieldMaskPaths: []string{"Name"},
				nullPaths:      []string{"Description"},
				ScopeId:        org.PublicId,
			},
			wantErr:        false,
			wantRowsUpdate: 1,
		},
		{
			name: "set name null",
			args: args{
				description:    "set description null" + id,
				fieldMaskPaths: []string{"Description"},
				nullPaths:      []string{"Name"},
				ScopeId:        org.PublicId,
			},
			wantErr:    true,
			wantErrMsg: `update: failed: pq: null value in column "name" violates not-null constraint`,
		},
		{
			name: "set description null",
			args: args{
				name:           "set name null" + id,
				fieldMaskPaths: []string{"Name"},
				nullPaths:      []string{"Description"},
				ScopeId:        org.PublicId,
			},
			wantErr:        false,
			wantRowsUpdate: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert, require := assert.New(t), require.New(t)
			if tt.wantDup {
				target := TestTcpTarget(t, conn, org.PublicId, testTargetName(t, org.PublicId))
				target.Name = tt.args.name
				_, err := rw.Update(context.Background(), target, tt.args.fieldMaskPaths, tt.args.nullPaths)
				require.NoError(err)
			}

			id := testId(t)
			target := TestTcpTarget(t, conn, org.PublicId, id, WithDescription(id))

			updateTarget := allocTcpTarget()
			updateTarget.PublicId = target.PublicId
			updateTarget.ScopeId = tt.args.ScopeId
			updateTarget.Name = tt.args.name
			updateTarget.Description = tt.args.description

			updatedRows, err := rw.Update(context.Background(), &updateTarget, tt.args.fieldMaskPaths, tt.args.nullPaths)
			if tt.wantErr {
				require.Error(err)
				assert.Equal(0, updatedRows)
				assert.Equal(tt.wantErrMsg, err.Error())
				err = db.TestVerifyOplog(t, rw, target.PublicId, db.WithOperation(oplog.OpType_OP_TYPE_UPDATE), db.WithCreateNotBefore(10*time.Second))
				require.Error(err)
				assert.Equal("record not found", err.Error())
				return
			}
			require.NoError(err)
			assert.Equal(tt.wantRowsUpdate, updatedRows)
			assert.NotEqual(target.UpdateTime, updateTarget.UpdateTime)
			foundTarget := allocTcpTarget()
			foundTarget.PublicId = target.GetPublicId()
			err = rw.LookupByPublicId(context.Background(), &foundTarget)
			require.NoError(err)
			assert.True(proto.Equal(updateTarget, foundTarget))
			if len(tt.args.nullPaths) != 0 {
				dbassert := dbassert.New(t, rw)
				for _, f := range tt.args.nullPaths {
					dbassert.IsNull(&foundTarget, f)
				}
			}
		})
	}
	t.Run("update dup names in diff scopes", func(t *testing.T) {
		assert, require := assert.New(t), require.New(t)
		id := testId(t)
		_ = TestTcpTarget(t, conn, org.PublicId, id, WithDescription(id))
		projTarget := TestTcpTarget(t, conn, proj.PublicId, id)
		projTarget.Name = id
		updatedRows, err := rw.Update(context.Background(), projTarget, []string{"Name"}, nil)
		require.NoError(err)
		assert.Equal(1, updatedRows)

		foundTarget := allocTcpTarget()
		foundTarget.PublicId = projTarget.GetPublicId()
		err = rw.LookupByPublicId(context.Background(), &foundTarget)
		require.NoError(err)
		assert.Equal(id, projTarget.Name)
	})
}

func TestTcpTarget_Clone(t *testing.T) {
	t.Parallel()
	conn, _ := db.TestSetup(t, "postgres")
	wrapper := db.TestWrapper(t)
	t.Run("valid", func(t *testing.T) {
		assert := assert.New(t)
		org, _ := iam.TestScopes(t, iam.TestRepo(t, conn, wrapper))
		target := TestTcpTarget(t, conn, org.PublicId, testTargetName(t, org.PublicId))
		cp := target.Clone()
		assert.True(proto.Equal(cp.(*TcpTarget).TcpTarget, target.TcpTarget))
	})
	t.Run("not-equal", func(t *testing.T) {
		assert := assert.New(t)
		org, _ := iam.TestScopes(t, iam.TestRepo(t, conn, wrapper))
		org2, _ := iam.TestScopes(t, iam.TestRepo(t, conn, wrapper))
		target := TestTcpTarget(t, conn, org.PublicId, testTargetName(t, org.PublicId))
		target2 := TestTcpTarget(t, conn, org2.PublicId, testTargetName(t, org2.PublicId))

		cp := target.Clone()
		assert.True(!proto.Equal(cp.(*TcpTarget).TcpTarget, target2.TcpTarget))
	})
}

func TestTcpTable_SetTableName(t *testing.T) {
	t.Parallel()
	defaultTableName := DefaultTcpTableName
	tests := []struct {
		name      string
		setNameTo string
		want      string
	}{
		{
			name:      "new-name",
			setNameTo: "new-name",
			want:      "new-name",
		},
		{
			name:      "reset to default",
			setNameTo: "",
			want:      defaultTableName,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert, require := assert.New(t), require.New(t)
			def := allocTcpTarget()
			require.Equal(defaultTableName, def.TableName())
			s := allocTcpTarget()
			s.SetTableName(tt.setNameTo)
			assert.Equal(tt.want, s.TableName())
		})
	}
}