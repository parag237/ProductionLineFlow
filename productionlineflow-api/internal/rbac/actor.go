package rbac

import "fmt"

type Assignment struct {
	Permission string  `json:"permission"`
	WarehouseID *string `json:"warehouse_id,omitempty"`
}

type Actor struct {
	UserID      string       `json:"user_id"`
	CompanyID   string       `json:"company_id"`
	Assignments []Assignment `json:"assignments"`
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
