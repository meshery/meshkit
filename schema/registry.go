package schema

import (
	stderrors "errors"
	"io/fs"
	"path"
	"sort"
	"strings"

	meshkitencoding "github.com/meshery/meshkit/encoding"
	meshschemas "github.com/meshery/schemas"
)

const constructsRoot = "schemas/constructs"

func builtinRegistrations() ([]Registration, error) {
	registrations := []Registration{}
	seen := map[string]struct{}{}

	err := fs.WalkDir(meshschemas.Schemas, constructsRoot, func(assetPath string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		name := entry.Name()
		if entry.IsDir() {
			if strings.HasPrefix(name, ".") || name == "templates" {
				return fs.SkipDir
			}
			return nil
		}

		if strings.HasPrefix(name, ".") || !isDiscoverableSchemaAsset(name) || isAPIDescription(name) {
			return nil
		}

		registration, ok, err := discoverRegistration(meshschemas.Schemas, assetPath)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}

		if _, exists := seen[registration.Location]; exists {
			return nil
		}
		seen[registration.Location] = struct{}{}
		registrations = append(registrations, registration)
		return nil
	})
	if err != nil {
		return nil, ErrInvalidRegistration(err)
	}

	sort.Slice(registrations, func(i, j int) bool {
		return registrations[i].Location < registrations[j].Location
	})

	return registrations, nil
}

func (v *Validator) Register(registration Registration) error {
	if registration.Ref.IsZero() {
		return ErrInvalidRegistration(stderrors.New("schema ref is empty"))
	}

	registration.Location = strings.TrimSpace(registration.Location)
	if registration.Location == "" {
		return ErrInvalidRegistration(stderrors.New("schema location is empty"))
	}

	if registration.AssetVersion == "" {
		registration.AssetVersion = assetVersionFromLocation(registration.Location)
	}

	v.mu.Lock()
	defer v.mu.Unlock()

	if registration.Ref.SchemaVersion != "" {
		v.registrations[schemaVersionKey(registration.Ref.SchemaVersion)] = registration
	}

	if registration.Ref.Type != "" {
		if registration.AssetVersion != "" {
			if v.typeVersions[registration.Ref.Type] == nil {
				v.typeVersions[registration.Ref.Type] = map[string]Registration{}
			}
			v.typeVersions[registration.Ref.Type][registration.AssetVersion] = registration
		}

		// Type key uses "latest wins" semantics: if multiple schema versions are registered
		// for the same type, the most recently registered one becomes the default for
		// type-only lookups (e.g. ValidateAs). This remains intentional for current callers.
		v.registrations[typeKey(registration.Ref.Type)] = registration
	}

	return nil
}

func (v *Validator) resolve(ref Ref) (Registration, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()

	if ref.SchemaVersion != "" {
		if registration, ok := v.registrations[schemaVersionKey(ref.SchemaVersion)]; ok {
			return registration, nil
		}

		if registration, ok := v.resolveByDerivedSchemaVersion(ref.SchemaVersion, ref.Type); ok {
			return registration, nil
		}

		// SchemaVersion was explicitly specified but has no registered schema.
		// Do NOT fall back to the Type key – that would silently validate the document
		// against a different schema than the caller requested.
		return Registration{}, ErrResolveSchema(ref)
	}

	if ref.Type != "" {
		if registration, ok := v.registrations[typeKey(ref.Type)]; ok {
			return registration, nil
		}
	}

	return Registration{}, ErrResolveSchema(ref)
}

func (v *Validator) resolveByDerivedSchemaVersion(schemaVersion string, explicitType DocumentType) (Registration, bool) {
	documentType, assetVersion, ok := parseSchemaVersion(schemaVersion)
	if !ok {
		return Registration{}, false
	}

	if explicitType != "" && explicitType != documentType {
		return Registration{}, false
	}

	versionedRegistrations, ok := v.typeVersions[documentType]
	if !ok {
		return Registration{}, false
	}

	registration, ok := versionedRegistrations[assetVersion]
	return registration, ok
}

func discoverRegistration(fsys fs.FS, assetPath string) (Registration, bool, error) {
	asset, err := fs.ReadFile(fsys, assetPath)
	if err != nil {
		return Registration{}, false, err
	}

	var document any
	if err := meshkitencoding.Unmarshal(asset, &document); err != nil {
		return Registration{}, false, nil
	}

	root, ok := document.(map[string]any)
	if !ok {
		return Registration{}, false, nil
	}

	location, schemaNode, ok := discoverLocation(assetPath, root)
	if !ok {
		return Registration{}, false, nil
	}

	documentType := documentTypeFromAssetPath(assetPath)
	if documentType == "" {
		return Registration{}, false, nil
	}

	return Registration{
		Ref: Ref{
			SchemaVersion: extractSchemaVersion(fsys, assetPath, schemaNode),
			Type:          documentType,
		},
		Location:     location,
		AssetVersion: assetVersionFromLocation(location),
	}, true, nil
}

func discoverLocation(assetPath string, root map[string]any) (string, map[string]any, bool) {
	if isRootSchema(root) {
		return assetPath, root, true
	}

	componentSchemas, ok := nestedMap(root, "components", "schemas")
	if !ok {
		return "", nil, false
	}

	candidates := map[string]map[string]any{}
	for componentName, rawSchema := range componentSchemas {
		schema, ok := rawSchema.(map[string]any)
		if !ok || !hasSchemaVersionProperty(schema) {
			continue
		}
		candidates[componentName] = schema
	}

	if len(candidates) != 1 {
		return "", nil, false
	}

	for componentName, schema := range candidates {
		return assetPath + "#/components/schemas/" + escapeJSONPointerToken(componentName), schema, true
	}

	return "", nil, false
}

func isRootSchema(root map[string]any) bool {
	if looksLikeOpenAPIDocument(root) {
		return false
	}

	return looksLikeJSONSchema(root)
}

func looksLikeOpenAPIDocument(root map[string]any) bool {
	_, hasOpenAPI := root["openapi"]
	_, hasInfo := root["info"]
	_, hasPaths := root["paths"]
	return hasOpenAPI || (hasInfo && hasPaths)
}

func looksLikeJSONSchema(root map[string]any) bool {
	for _, key := range []string{
		"$schema",
		"$id",
		"$ref",
		"type",
		"properties",
		"definitions",
		"$defs",
		"items",
		"enum",
		"const",
		"allOf",
		"anyOf",
		"oneOf",
		"required",
		"additionalProperties",
	} {
		if _, ok := root[key]; ok {
			return true
		}
	}

	return false
}

func hasSchemaVersionProperty(schema map[string]any) bool {
	properties, ok := nestedMap(schema, "properties")
	if !ok {
		return false
	}

	_, ok = properties["schemaVersion"]
	return ok
}

func extractSchemaVersion(fsys fs.FS, assetPath string, schemaNode map[string]any) string {
	if schemaVersion, ok := schemaVersionFromProperty(schemaNode); ok {
		return schemaVersion
	}

	if schemaVersion, ok := uniqueSchemaVersionLiteral(schemaNode); ok {
		return schemaVersion
	}

	if schemaVersion, ok := schemaVersionFromTemplates(fsys, assetPath); ok {
		return schemaVersion
	}

	return ""
}

func schemaVersionFromProperty(schemaNode map[string]any) (string, bool) {
	properties, ok := nestedMap(schemaNode, "properties")
	if !ok {
		return "", false
	}

	property, ok := properties["schemaVersion"].(map[string]any)
	if !ok {
		return "", false
	}

	if schemaVersion, ok := stringField(property, "const"); ok {
		return schemaVersion, true
	}

	if schemaVersion, ok := stringField(property, "default"); ok {
		return schemaVersion, true
	}

	values, ok := stringSliceField(property, "enum")
	if ok && len(values) == 1 {
		return values[0], true
	}

	values, ok = stringSliceField(property, "examples")
	if ok && len(values) == 1 {
		return values[0], true
	}

	if schemaVersion, ok := stringField(property, "example"); ok {
		return schemaVersion, true
	}

	return "", false
}

func uniqueSchemaVersionLiteral(schemaNode map[string]any) (string, bool) {
	values := map[string]struct{}{}
	for _, key := range []string{"default", "example", "examples"} {
		value, ok := schemaNode[key]
		if !ok {
			continue
		}
		collectSchemaVersionFields(value, values)
	}

	return uniqueString(values)
}

func schemaVersionFromTemplates(fsys fs.FS, assetPath string) (string, bool) {
	templatePath := path.Join(path.Dir(assetPath), "templates")
	entries, err := fs.ReadDir(fsys, templatePath)
	if err != nil {
		return "", false
	}

	values := map[string]struct{}{}
	for _, entry := range entries {
		if entry.IsDir() || !isDiscoverableSchemaAsset(entry.Name()) {
			continue
		}

		templateAsset, err := fs.ReadFile(fsys, path.Join(templatePath, entry.Name()))
		if err != nil {
			return "", false
		}

		var document any
		if err := meshkitencoding.Unmarshal(templateAsset, &document); err != nil {
			continue
		}

		object, ok := document.(map[string]any)
		if !ok {
			continue
		}

		schemaVersion, ok := object["schemaVersion"].(string)
		if !ok || strings.TrimSpace(schemaVersion) == "" {
			continue
		}

		values[schemaVersion] = struct{}{}
	}

	return uniqueString(values)
}

func collectSchemaVersionFields(value any, values map[string]struct{}) {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if key == "schemaVersion" {
				schemaVersion, ok := child.(string)
				if ok && strings.TrimSpace(schemaVersion) != "" {
					values[schemaVersion] = struct{}{}
				}
			}
			collectSchemaVersionFields(child, values)
		}
	case []any:
		for _, child := range typed {
			collectSchemaVersionFields(child, values)
		}
	}
}

func documentTypeFromAssetPath(assetPath string) DocumentType {
	base := path.Base(assetPath)
	extension := path.Ext(base)
	if extension == "" {
		return ""
	}

	return DocumentType(strings.TrimSuffix(base, extension))
}

func assetVersionFromLocation(location string) string {
	assetPath, _, _ := strings.Cut(location, "#")
	segments := strings.Split(assetPath, "/")
	for index := 0; index+1 < len(segments); index++ {
		if segments[index] == "constructs" {
			return segments[index+1]
		}
	}

	return ""
}

func parseSchemaVersion(schemaVersion string) (DocumentType, string, bool) {
	subject, assetVersion, ok := strings.Cut(strings.TrimSpace(schemaVersion), "/")
	if !ok || subject == "" || assetVersion == "" {
		return "", "", false
	}

	if index := strings.Index(subject, "."); index >= 0 {
		subject = subject[:index]
	}

	documentType := singularDocumentType(subject)
	if documentType == "" {
		return "", "", false
	}

	return documentType, assetVersion, true
}

func singularDocumentType(subject string) DocumentType {
	subject = strings.TrimSpace(strings.ToLower(subject))
	if subject == "" {
		return ""
	}

	switch {
	case strings.HasSuffix(subject, "ies") && len(subject) > len("ies"):
		subject = subject[:len(subject)-len("ies")] + "y"
	case strings.HasSuffix(subject, "s") && !strings.HasSuffix(subject, "ss"):
		subject = subject[:len(subject)-1]
	}

	return DocumentType(subject)
}

func nestedMap(root map[string]any, keys ...string) (map[string]any, bool) {
	current := root
	for _, key := range keys {
		value, ok := current[key]
		if !ok {
			return nil, false
		}

		next, ok := value.(map[string]any)
		if !ok {
			return nil, false
		}

		current = next
	}

	return current, true
}

func stringField(root map[string]any, key string) (string, bool) {
	value, ok := root[key].(string)
	if !ok {
		return "", false
	}

	value = strings.TrimSpace(value)
	if value == "" {
		return "", false
	}

	return value, true
}

func stringSliceField(root map[string]any, key string) ([]string, bool) {
	rawValues, ok := root[key].([]any)
	if !ok {
		return nil, false
	}

	values := make([]string, 0, len(rawValues))
	for _, rawValue := range rawValues {
		value, ok := rawValue.(string)
		if !ok || strings.TrimSpace(value) == "" {
			return nil, false
		}
		values = append(values, strings.TrimSpace(value))
	}

	return values, true
}

func uniqueString(values map[string]struct{}) (string, bool) {
	if len(values) != 1 {
		return "", false
	}

	for value := range values {
		return value, true
	}

	return "", false
}

func isDiscoverableSchemaAsset(name string) bool {
	switch path.Ext(name) {
	case ".yaml", ".yml", ".json":
		return true
	default:
		return false
	}
}

func isAPIDescription(name string) bool {
	base := strings.TrimSuffix(name, path.Ext(name))
	return base == "api"
}

func schemaVersionKey(schemaVersion string) string {
	return "schemaVersion:" + schemaVersion
}

func typeKey(documentType DocumentType) string {
	return "type:" + string(documentType)
}
