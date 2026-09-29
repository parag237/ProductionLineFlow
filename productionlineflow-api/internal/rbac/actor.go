package rbac

import "fmt"

const (
	PlatformRoleProductOwner  = "product_owner"
	PermissionCompaniesCreate = "companies.create"
)

type Assignment struct {
	Permission  string  `json:"permission"`
	WarehouseID *string `json:"warehouse_id,omitempty"`
}

type Actor struct {
	UserID      string       `json:"user_id"`
	CompanyID   string       `json:"company_id"`
	Assignments []Assignment `json:"assignments"`
}

type PlatformActor struct {
	UserID      string   `json:"user_id"`
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

func (a Actor) Can(permission string, warehouseID *string) bool {
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

func (a Actor) MustHave(permission string, warehouseID *string) error {
	if !a.Can(permission, warehouseID) {
		return fmt.Errorf("forbidden: missing permission %q", permission)
	}
	return nil
}
