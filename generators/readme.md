# Generators

Generators are responsible for creating Meshery components from various sources like ArtifactHub and GitHub.

## ArtifactHub Generator

The ArtifactHub generator creates Meshery components by discovering Helm charts from [ArtifactHub](https://artifacthub.io/).

### How It Works

#### First Generation (New Model)

When a model is generated for the first time with `registrant: artifacthub`:

1. **CSV Input**: The model's `SourceURL` field contains a search query (e.g., `"consul"`)
2. **Search ArtifactHub**: The generator searches ArtifactHub for packages matching the name
3. **Package Selection**: The best package is selected based on ranking (verified publisher, CNCF, official, etc.)
4. **Package Resolution**: The selected package's actual chart URL is obtained (e.g., `"https://charts.bitnami.com/bitnami/consul-1.0.0.tgz"`)
5. **Component Generation**: Components are generated from the Helm chart's CRDs
6. **Persistence**: 
   - Model metadata `source_uri` is set to the actual package URL
   - CSV model's `SourceURL` is updated to the actual package URL
   - Model definition is written to filesystem
   - Updated CSV is sent to spreadsheet with the resolved URL

#### Subsequent Updates (Existing Model)

When the model is updated in subsequent runs:

1. **CSV Input**: The model's `SourceURL` field contains the actual package URL (e.g., `"https://charts.bitnami.com/bitnami/consul-1.0.0.tgz"`)
2. **Direct URL Usage**: The generator detects the URL and uses it directly (no search needed)
3. **Package Fetch**: The package is fetched from the URL
4. **Component Generation**: Components are generated from the Helm chart's CRDs

### Benefits

- **Consistency**: The actual package URL is stored and used everywhere
- **Efficiency**: Subsequent updates don't need to search ArtifactHub again
- **Reliability**: Using direct URLs ensures we get the same package every time
- **Backward Compatible**: Still supports search queries for new models

### URL Detection

The generator automatically detects if `SourceURL` is:
- **A search query**: Any string that doesn't start with `http://`, `https://`, or `oci://`
- **A direct URL**: Any string starting with `http://`, `https://`, or `oci://`

### Code Structure

```
generators/
├── artifacthub/
│   ├── package_manager.go  # Main entry point, detects URL vs search query
│   ├── scanner.go          # Searches ArtifactHub API
│   ├── ranker.go           # Ranks packages by quality metrics
│   └── package.go          # Handles package data and component generation
└── generator.go            # Factory for creating generators
```

## GitHub Generator

The GitHub generator creates Meshery components from GitHub repositories containing Kubernetes manifests or Helm charts.

### Usage

Set `registrant: github` in the model CSV and provide a GitHub URL in the `SourceURL` field.
