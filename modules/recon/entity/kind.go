package entity

// KindRole maps a semantic node kind to its cross-language role.
// Used internally by the quota and coverage systems -- never exposed to the model.
// When adding a new kind, assign its role here first.
func KindRole(kind string) string {
	switch kind {
	case "method", "vue-method":
		return "method"
	case "func", "composable":
		return "func"
	case "struct", "class", "interface", "trait", "enum", "type":
		return "type"
	case "component", "vue-setup", "vue-setup-legacy", "store", "plugin", "blade":
		return "component"
	case "vue-template", "vue-style":
		return "view"
	case "const", "var":
		return "data"
	case "imports", "commented-block":
		return "meta"
	default:
		return "config"
	}
}
