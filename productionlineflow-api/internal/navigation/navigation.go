package navigation

type MenuItem struct {
	Label string `json:"label"`
	Route string `json:"route"`
}

func BuildMenu(permissions []string) []MenuItem {
	items := make([]MenuItem, 0, len(permissions))
	for _, permission := range permissions {
		items = append(items, MenuItem{Label: permission, Route: "/" + permission})
	}
	if len(items) == 0 {
		return []MenuItem{{Label: "dashboard", Route: "/dashboard"}}
	}
	return items
}
