// Code generated by "make api"; DO NOT EDIT.
package groups

import (
	"context"
	"fmt"

	"github.com/hashicorp/watchtower/api"
)

func (s Group) ReadGroup(ctx context.Context, id string) (*Group, *api.Error, error) {
	if id == "" {
		return nil, nil, fmt.Errorf("empty ID value passed into ReadGroup request")
	}

	if s.Client == nil {
		return nil, nil, fmt.Errorf("nil client in ReadGroup request")
	}

	var opts []api.Option
	if s.Scope.Id != "" {
		// If it's explicitly set here, override anything that might be in the
		// client
		opts = append(opts, api.WithScopeId(s.Scope.Id))
	}

	req, err := s.Client.NewRequest(ctx, "GET", fmt.Sprintf("%s/%s", "groups", id), nil, opts...)
	if err != nil {
		return nil, nil, fmt.Errorf("error creating ReadGroup request: %w", err)
	}

	resp, err := s.Client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("error performing client request during ReadGroup call: %w", err)
	}

	target := new(Group)
	apiErr, err := resp.Decode(target)
	if err != nil {
		return nil, nil, fmt.Errorf("error decoding ReadGroup repsonse: %w", err)
	}

	target.Client = s.Client

	return target, apiErr, nil
}