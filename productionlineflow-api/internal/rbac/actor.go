package rbac

import (
	"fmt"
	"productionlineflow-api/internal/constants"
)

func DefaultPlatformRolePermissions(role string) []string {
	switch role {
	case constants.PlatformRoleProductOwner:
		return []string{constants.PermissionCompaniesCreate}
	default:
		return nil
	}
}

type Assignment struct {
	Permission  string `json:"permission"`
	WarehouseID *int64 `json:"warehouse_id,omitempty"`
}

type Actor struct {
	UserID      int64        `json:"user_id"`
	CompanyID   int64        `json:"company_id"`
	Assignments []Assignment `json:"assignments"`
}

type PlatformActor struct {
	UserID      int64    `json:"user_id"`
	Role        string   `json:"role"`
	Permissions []string `json:"permissions"`
}

func (a PlatformActor) Can(permission string) bool {
	for _, granted := range a.Permissions {
		if granted == permission {
			return true
		}
	}
	return false
}

func (a PlatformActor) MustHave(permission string) error {
	if !a.Can(permission) {
		return fmt.Errorf("forbidden: missing platform permission %q", permission)
	}
	return nil
}

func (a Actor) Can(permission string, warehouseID *int64) bool {
	for _, as := range a.Assignments {
		if as.Permission != permission {
			continue
		}
		if as.WarehouseID == nil {
			return true
		}
		if warehouseID != nil && *as.WarehouseID == *warehouseID {
			return true
		}
	}
	return false
}

func (a Actor) MustHave(permission string, warehouseID *int64) error {
	if !a.Can(permission, warehouseID) {
		return fmt.Errorf("forbidden: missing permission %q", permission)
	}
	return nil
}
