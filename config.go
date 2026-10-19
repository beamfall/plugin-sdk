package sdk

// ConfigFieldSpec declares one operator-editable configuration field of a
// plugin (or one input field of an ActionSpec). Its Type reuses the small
// FieldTypes vocabulary so clients already know how to render and validate the
// value shape.
type ConfigFieldSpec struct {
	Name     string
	Type     string
	Label    LocaleString
	Required bool
	Default  string
	Options  []ConfigOption
}

// ConfigOption constrains a config field to one selectable value.
type ConfigOption struct {
	Value string
	Label LocaleString
}

var permissionKinds = []string{
	"declared-outbound",
	"read-catalog",
	"event-subscribe",
	"plugin-secret",
	"status-write",
	"plugin-job",
	"profile-link",
	"request-ingest",
	"request-file-mutation",
}

var permissionPolicies = map[string]PermissionPolicy{
	"declared-outbound": {
		Label:       L("Declared outbound access"),
		Description: L("Allows the plugin to send data to operator-approved network destinations."),
	},
	"read-catalog": {
		Label:       L("Read catalog"),
		Description: L("Allows the plugin to read Reveal-filtered catalog metadata. Catalog reads require explicit approval and are audited."),
		HighRisk:    true,
	},
	"event-subscribe": {
		Label:       L("Subscribe to events"),
		Description: L("Allows the plugin to receive filtered server events."),
	},
	"plugin-secret": {
		Label:       L("Access plugin secrets"),
		Description: L("Allows the plugin to read and replace secrets stored only for this plugin."),
	},
	"status-write": {
		Label:       L("Write plugin status"),
		Description: L("Allows the plugin to write its own status and dead-letter records."),
	},
	"plugin-job": {
		Label:       L("Run plugin jobs"),
		Description: L("Allows the plugin to enqueue and run its own background jobs."),
	},
	"profile-link": {
		Label:       L("Link profiles"),
		Description: L("Allows the plugin to link external accounts to Beamfall profiles. Profile linking requires explicit approval and is audited."),
		HighRisk:    true,
	},
	"request-ingest": {
		Label:       L("Request ingest"),
		Description: L("Allows the plugin to request that Beamfall ingest content. Ingest requests require explicit approval and are audited."),
		HighRisk:    true,
	},
	"request-file-mutation": {
		Label:       L("Request file changes"),
		Description: L("Allows the plugin to move files, move files to Trash, or permanently delete files. File changes require explicit approval and are audited."),
		HighRisk:    true,
	},
}

// PermissionKinds returns the Core-owned Integration Plugin permission
// vocabulary. These are Host/API grants, not public GraphQL auth
// scopes.
func PermissionKinds() []string { return copySlice(permissionKinds) }

// PermissionPolicy is the host-owned minimum presentation and risk policy for
// one permission kind. Plugins cannot replace its Label or Description, or
// lower its HighRisk classification.
type PermissionPolicy struct {
	Label       LocaleString
	Description LocaleString
	HighRisk    bool
}

// PermissionPolicyFor returns the host-owned minimum policy for kind. Unknown
// kinds fail closed. Returned locale maps are copies.
func PermissionPolicyFor(kind string) (PermissionPolicy, bool) {
	policy, ok := permissionPolicies[kind]
	if !ok {
		return PermissionPolicy{}, false
	}
	policy.Label = copyLocaleString(policy.Label)
	policy.Description = copyLocaleString(policy.Description)
	return policy, true
}

func copyLocaleString(value LocaleString) LocaleString {
	if value == nil {
		return nil
	}
	copied := make(LocaleString, len(value))
	for locale, text := range value {
		copied[locale] = text
	}
	return copied
}

// PermissionSpec declares one permission a plugin requests or needs for an
// action. Label and Description must equal PermissionPolicyFor(Kind);
// Context is the plugin's optional additive explanation. HighRisk cannot be
// lower than the host-owned classification.
type PermissionSpec struct {
	Kind        string
	Label       LocaleString
	Description LocaleString
	Context     LocaleString
	HighRisk    bool
}

var actionRiskLevels = []string{"info", "low", "medium", "high"}

// ActionRiskLevels returns the fixed risk vocabulary for Plugin Action
// Descriptors.
func ActionRiskLevels() []string { return copySlice(actionRiskLevels) }

var controlPolicies = []string{"manager-enabled", "console-approval-required", "console-only"}

// ControlPolicies returns the fixed Manager access posture vocabulary.
func ControlPolicies() []string { return copySlice(controlPolicies) }

// ActionSpec is one server-performed Plugin Action Descriptor.
type ActionSpec struct {
	ID    string
	Label LocaleString
	// Icon is an optional client-known symbol name for the action button.
	// Empty lets clients choose their default action glyph.
	Icon                string
	Risk                string
	RequiredPermissions []string
	InputFields         []ConfigFieldSpec
	Confirmation        LocaleString
}
