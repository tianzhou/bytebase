package v1

import (
	"context"

	"connectrpc.com/connect"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/bytebase/bytebase/backend/common"
	"github.com/bytebase/bytebase/backend/component/iam"
	"github.com/bytebase/bytebase/backend/enterprise"
	"github.com/bytebase/bytebase/backend/store"
	storepb "github.com/bytebase/bytebase/proto/generated-go/store"
	v1pb "github.com/bytebase/bytebase/proto/generated-go/v1"
	"github.com/bytebase/bytebase/proto/generated-go/v1/v1connect"
)

// GroupService implements the group service.
type GroupService struct {
	v1connect.UnimplementedGroupServiceHandler
	store          *store.Store
	iamManager     *iam.Manager
	licenseService *enterprise.LicenseService
}

// NewGroupService creates a new GroupService.
func NewGroupService(store *store.Store, iamManager *iam.Manager, licenseService *enterprise.LicenseService) *GroupService {
	return &GroupService{
		store:          store,
		iamManager:     iamManager,
		licenseService: licenseService,
	}
}

// GetGroup gets a group.
func (s *GroupService) GetGroup(ctx context.Context, req *connect.Request[v1pb.GetGroupRequest]) (*connect.Response[v1pb.Group], error) {
	email, err := common.GetGroupEmail(req.Msg.Name)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	group, err := s.store.GetGroup(ctx, email)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	result, err := s.convertToV1Group(ctx, group)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(result), nil
}

// ListGroups lists all groups.
func (s *GroupService) ListGroups(ctx context.Context, _ *connect.Request[v1pb.ListGroupsRequest]) (*connect.Response[v1pb.ListGroupsResponse], error) {
	groups, err := s.store.ListGroups(ctx, &store.FindGroupMessage{})
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	response := &v1pb.ListGroupsResponse{}
	for _, groupMessage := range groups {
		group, err := s.convertToV1Group(ctx, groupMessage)
		if err != nil {
			return nil, err
		}
		response.Groups = append(response.Groups, group)
	}
	return connect.NewResponse(response), nil
}

// CreateGroup creates a group.
func (s *GroupService) CreateGroup(ctx context.Context, req *connect.Request[v1pb.CreateGroupRequest]) (*connect.Response[v1pb.Group], error) {
	if err := s.licenseService.IsFeatureEnabled(v1pb.PlanFeature_FEATURE_USER_GROUPS); err != nil {
		return nil, status.Error(codes.PermissionDenied, err.Error())
	}
	groupMessage, err := s.convertToGroupMessage(ctx, req.Msg)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	if err := validateEmailWithDomains(ctx, s.licenseService, s.store, groupMessage.Email, false /* isServiceAccount */, true); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid email %q, error: %v", groupMessage.Email, err)
	}

	group, err := s.store.CreateGroup(ctx, groupMessage)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	if err := s.iamManager.ReloadCache(ctx); err != nil {
		return nil, err
	}

	result, err := s.convertToV1Group(ctx, group)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(result), nil
}

// UpdateGroup updates a group.
func (s *GroupService) UpdateGroup(ctx context.Context, req *connect.Request[v1pb.UpdateGroupRequest]) (*connect.Response[v1pb.Group], error) {
	if err := s.licenseService.IsFeatureEnabled(v1pb.PlanFeature_FEATURE_USER_GROUPS); err != nil {
		return nil, status.Error(codes.PermissionDenied, err.Error())
	}
	groupEmail, err := common.GetGroupEmail(req.Msg.Group.Name)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	user, ok := ctx.Value(common.UserContextKey).(*store.UserMessage)
	if !ok {
		return nil, status.Errorf(codes.Internal, "user not found")
	}

	group, err := s.store.GetGroup(ctx, groupEmail)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get group %q with error: %v", groupEmail, err)
	}
	if group == nil {
		if req.Msg.AllowMissing {
			ok, err := s.iamManager.CheckPermission(ctx, iam.PermissionGroupsCreate, user)
			if err != nil {
				return nil, status.Errorf(codes.Internal, "failed to check permission with error: %v", err.Error())
			}
			if !ok {
				return nil, status.Errorf(codes.PermissionDenied, "user does not have permission %q", iam.PermissionGroupsCreate)
			}
			return s.CreateGroup(ctx, connect.NewRequest(&v1pb.CreateGroupRequest{
				Group:      req.Msg.Group,
				GroupEmail: groupEmail,
			}))
		}
		return nil, status.Errorf(codes.NotFound, "group %q not found", groupEmail)
	}

	if err := s.checkPermission(ctx, group, user, iam.PermissionGroupsUpdate); err != nil {
		return nil, err
	}

	patch := &store.UpdateGroupMessage{}
	for _, path := range req.Msg.UpdateMask.Paths {
		switch path {
		case "title":
			patch.Title = &req.Msg.Group.Title
		case "description":
			patch.Description = &req.Msg.Group.Description
		case "members":
			if group.Payload.Source != "" {
				return nil, status.Errorf(codes.InvalidArgument, "cannot change members for external group")
			}
			payload, err := s.convertToGroupPayload(ctx, req.Msg.Group)
			if err != nil {
				return nil, status.Error(codes.InvalidArgument, err.Error())
			}
			patch.Payload = payload
		default:
			return nil, status.Errorf(codes.InvalidArgument, `unsupported update_mask "%s"`, path)
		}
	}

	groupMessage, err := s.store.UpdateGroup(ctx, groupEmail, patch)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	if err := s.iamManager.ReloadCache(ctx); err != nil {
		return nil, err
	}

	result, err := s.convertToV1Group(ctx, groupMessage)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(result), nil
}

// DeleteGroup deletes a group.
func (s *GroupService) DeleteGroup(ctx context.Context, req *connect.Request[v1pb.DeleteGroupRequest]) (*connect.Response[emptypb.Empty], error) {
	email, err := common.GetGroupEmail(req.Msg.Name)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	user, ok := ctx.Value(common.UserContextKey).(*store.UserMessage)
	if !ok {
		return nil, status.Errorf(codes.Internal, "user not found")
	}

	group, err := s.store.GetGroup(ctx, email)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get group %q with error: %v", email, err)
	}
	if group == nil {
		return nil, status.Errorf(codes.NotFound, "cannot found the group %v", req.Msg.Name)
	}

	if err := s.checkPermission(ctx, group, user, iam.PermissionGroupsDelete); err != nil {
		return nil, err
	}

	if err := s.store.DeleteGroup(ctx, email); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	if err := s.iamManager.ReloadCache(ctx); err != nil {
		return nil, err
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *GroupService) checkPermission(ctx context.Context, group *store.GroupMessage, user *store.UserMessage, permission string) error {
	userName := common.FormatUserUID(user.ID)

	ok, err := func() (bool, error) {
		for _, member := range group.Payload.GetMembers() {
			if member.Role == storepb.GroupMember_OWNER && member.Member == userName {
				return true, nil
			}
		}
		return s.iamManager.CheckPermission(ctx, permission, user)
	}()
	if err != nil {
		return status.Errorf(codes.Internal, "failed to check permission, error: %v", err)
	}
	if !ok {
		return status.Errorf(codes.PermissionDenied, "user does not have permission %q", permission)
	}
	return nil
}

func (s *GroupService) convertToGroupPayload(ctx context.Context, group *v1pb.Group) (*storepb.GroupPayload, error) {
	payload := &storepb.GroupPayload{}
	for _, member := range group.Members {
		email, err := common.GetUserEmail(member.Member)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to get member email, error %v", err)
		}
		user, err := s.store.GetUserByEmail(ctx, email)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to get member %s, error %v", member.Member, err)
		}
		if user == nil {
			return nil, status.Errorf(codes.InvalidArgument, "cannot found member %s", member.Member)
		}
		if user.Type != storepb.PrincipalType_END_USER {
			return nil, status.Errorf(codes.InvalidArgument, "only allow add end users to the group")
		}

		m := &storepb.GroupMember{
			Member: common.FormatUserUID(user.ID),
		}
		switch member.Role {
		case v1pb.GroupMember_MEMBER:
			m.Role = storepb.GroupMember_MEMBER
		case v1pb.GroupMember_OWNER:
			m.Role = storepb.GroupMember_OWNER
		default:
			return nil, status.Errorf(codes.InvalidArgument, "unsupport group member role %v", member.Role)
		}
		payload.Members = append(payload.Members, m)
	}
	return payload, nil
}

func (s *GroupService) convertToGroupMessage(ctx context.Context, request *v1pb.CreateGroupRequest) (*store.GroupMessage, error) {
	if request.GroupEmail == "" {
		return nil, status.Error(codes.InvalidArgument, "missing group_email in the request")
	}

	groupMessage := &store.GroupMessage{
		Email:       request.GroupEmail,
		Title:       request.Group.Title,
		Description: request.Group.Description,
		Payload:     &storepb.GroupPayload{},
	}

	payload, err := s.convertToGroupPayload(ctx, request.Group)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	groupMessage.Payload = payload
	return groupMessage, nil
}

func (s *GroupService) convertToV1Group(ctx context.Context, groupMessage *store.GroupMessage) (*v1pb.Group, error) {
	if groupMessage == nil {
		return nil, status.Errorf(codes.NotFound, "cannot found group")
	}

	group := &v1pb.Group{
		Name:        common.FormatGroupEmail(groupMessage.Email),
		Title:       groupMessage.Title,
		Description: groupMessage.Description,
		Source:      groupMessage.Payload.Source,
	}

	for _, member := range groupMessage.Payload.Members {
		uid, err := common.GetUserID(member.Member)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to get member id, error %v", err)
		}
		user, err := s.store.GetUserByID(ctx, uid)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to get member, error %v", err)
		}
		if user == nil {
			continue
		}

		m := &v1pb.GroupMember{
			Member: common.FormatUserEmail(user.Email),
			Role:   v1pb.GroupMember_ROLE_UNSPECIFIED,
		}
		switch member.Role {
		case storepb.GroupMember_MEMBER:
			m.Role = v1pb.GroupMember_MEMBER
		case storepb.GroupMember_OWNER:
			m.Role = v1pb.GroupMember_OWNER
		}
		group.Members = append(group.Members, m)
	}

	return group, nil
}
