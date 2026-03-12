// Package schema validates Meshery documents against the embedded schemas
// published by github.com/meshery/schemas.
//
// # Dynamic schema discovery
//
// Schema registrations are discovered at runtime by walking the embedded
// github.com/meshery/schemas module. Every schema asset found under
// schemas/constructs/ is registered automatically — including dynamically
// discovered types such as "selector" and "subcategory".
//
// To enumerate all registered document types at runtime, use the package-level
// [DocumentTypes] function or [Validator.DocumentTypes] on a specific [Validator]
// instance:
//
//	for _, dt := range schema.DocumentTypes() {
//		fmt.Println(dt)
//	}
//
// Callers that need a specific type should construct it directly with
// [DocumentType], for example:
//
//	ref := schema.Ref{
//		SchemaVersion: "relationships.meshery.io/v1alpha3",
//		Type:          schema.DocumentType("relationship"),
//	}
//
// Prefer [DocumentTypes] whenever code needs to enumerate registered types.
package schema
