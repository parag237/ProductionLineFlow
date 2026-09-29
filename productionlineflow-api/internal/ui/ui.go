package ui

type Screen struct {
	Name          string      `json:"name"`
	SchemaVersion int         `json:"schema_version"`
	Title        string      `json:"title"`
	Components   []Component `json:"components"`
	Actions      []Action    `json:"actions"`
}

type Component struct {
	Type          string         `json:"type"`
	ID            string         `json:"id,omitempty"`
	Name          string         `json:"name,omitempty"`
	Label         string         `json:"label,omitempty"`
	OptionsSource string         `json:"options_source,omitempty"`
	Fields        []Field        `json:"fields,omitempty"`
	Items         []Component    `json:"items,omitempty"`
	Text          string         `json:"text,omitempty"`
}

type Field struct {
	Type         string `json:"type"`
	Name         string `json:"name,omitempty"`
	Label        string `json:"label,omitempty"`
	Required     bool   `json:"required,omitempty"`
	MinLength    int    `json:"min,omitempty"`
	OptionsSource string `json:"options_source,omitempty"`
}

type Action struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Event   string `json:"event"`
	Style   string `json:"style,omitempty"`
	Submits string `json:"submits,omitempty"`
}
