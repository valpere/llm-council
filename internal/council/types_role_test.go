package council

import "testing"

func TestRoleBasedStrategyConstants(t *testing.T) {
	if RoleBased == PeerReview {
		t.Fatal("RoleBased must differ from PeerReview")
	}
	if RoleBasedReview == PeerReview {
		t.Fatal("RoleBasedReview must differ from PeerReview")
	}
	if RoleBased == RoleBasedReview {
		t.Fatal("RoleBased must differ from RoleBasedReview")
	}
}

func TestCouncilTypeHasRoles(t *testing.T) {
	ct := CouncilType{
		Name:     "test",
		Strategy: RoleBased,
		Roles: []Role{
			{Name: "critic", Instruction: "Find bugs."},
		},
	}
	if len(ct.Roles) != 1 {
		t.Fatalf("expected 1 role, got %d", len(ct.Roles))
	}
	if ct.Roles[0].Name != "critic" {
		t.Fatalf("unexpected role name %q", ct.Roles[0].Name)
	}
}
