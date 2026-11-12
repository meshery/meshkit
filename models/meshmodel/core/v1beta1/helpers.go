package v1beta1

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/meshery/meshkit/utils"
	"github.com/meshery/schemas/models/v1beta3/component"
	"k8s.io/apimachinery/pkg/util/validation"
)

// resolveSourceURI extracts source_uri from component metadata first, falling back
// to model metadata if component metadata is missing or does not contain source_uri.
// Returns an empty string and nil error if neither contains source_uri.
func resolveSourceURI(comp component.ComponentDefinition) (string, error) {
	// 1. Check Component Metadata
	if comp.Metadata.AdditionalProperties != nil {
		if raw, ok := comp.Metadata.AdditionalProperties["source_uri"]; ok && raw != nil {
			str, err := utils.Cast[string](raw)
			if err != nil {
				return "", fmt.Errorf("invalid component source_uri type: %w", err)
			}
			trimmed := strings.TrimSpace(str)
			if trimmed != "" {
				return trimmed, nil
			}
		}
	}

	// 2. Fall back to Model Metadata
	if comp.Model != nil && comp.Model.Metadata != nil && comp.Model.Metadata.AdditionalProperties != nil {
		if raw, ok := comp.Model.Metadata.AdditionalProperties["source_uri"]; ok && raw != nil {
			str, err := utils.Cast[string](raw)
			if err != nil {
				return "", fmt.Errorf("invalid model source_uri type: %w", err)
			}
			trimmed := strings.TrimSpace(str)
			if trimmed != "" {
				return trimmed, nil
			}
		}
	}

	return "", nil
}

// isValidDNS1123Label checks whether a string conforms to Kubernetes DNS-1123 label requirements.
func isValidDNS1123Label(name string) bool {
	if name == "" {
		return false
	}
	errs := validation.IsDNS1123Label(name)
	return len(errs) == 0
}

// resolveNamespace safely extracts the namespace from comp.Configuration["namespace"].
// If the configured namespace is valid under DNS-1123 label rules, it is returned.
// Otherwise, it validates and returns the fallback namespace, or "default" if the fallback is invalid or empty.
func resolveNamespace(comp component.ComponentDefinition, fallback string) string {
	if comp.Configuration != nil {
		if val, ok := comp.Configuration["namespace"]; ok && val != nil {
			if str, ok := val.(string); ok {
				trimmed := strings.TrimSpace(str)
				if isValidDNS1123Label(trimmed) {
					return trimmed
				}
			}
		}
	}
	trimmedFallback := strings.TrimSpace(fallback)
	if isValidDNS1123Label(trimmedFallback) {
		return trimmedFallback
	}
	return "default"
}

// classifyGitHubChartURL inspects a source URI and determines if it represents a deployable
// Helm chart archive (.tgz or .tar.gz). It normalizes git:// URLs to raw HTTPS URLs.
// Plain HTTP URLs return an error; non-archive HTTPS and git sources return shouldInstall=false and nil error.
func classifyGitHubChartURL(rawURI string) (normalizedURL string, shouldInstall bool, err error) {
	trimmed := strings.TrimSpace(rawURI)
	if trimmed == "" {
		return "", false, nil
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", false, fmt.Errorf("invalid source URI %q: %w", trimmed, err)
	}

	scheme := strings.ToLower(parsed.Scheme)
	pathLower := strings.ToLower(parsed.Path)

	isArchive := strings.HasSuffix(pathLower, ".tgz") || strings.HasSuffix(pathLower, ".tar.gz")

	switch scheme {
	case "https":
		if !isArchive {
			// Plain YAML manifest, directory, or non-chart URL -> skip cleanly
			return "", false, nil
		}
		// Direct HTTPS archive URL (e.g. GitHub release asset or hosted chart)
		return trimmed, true, nil

	case "http":
		return "", false, fmt.Errorf("insecure HTTP scheme %q in source URI %q: only HTTPS is permitted", parsed.Scheme, trimmed)

	case "git":
		hostLower := strings.ToLower(parsed.Host)
		if hostLower != "github.com" {
			return "", false, fmt.Errorf("unsupported git host %q: only git://github.com/ is supported", parsed.Host)
		}

		parts := strings.SplitN(strings.TrimPrefix(parsed.Path, "/"), "/", 4)
		if len(parts) < 4 {
			return "", false, fmt.Errorf("invalid git URI %q: expected 4 segments: git://github.com/<owner>/<repo>/<ref>/<path>", trimmed)
		}

		owner, repo, ref, path := parts[0], parts[1], parts[2], parts[3]
		filePathLower := strings.ToLower(path)
		if !strings.HasSuffix(filePathLower, ".tgz") && !strings.HasSuffix(filePathLower, ".tar.gz") {
			// Directory or non-archive path in repo -> skip cleanly
			return "", false, nil
		}

		// Convert to raw HTTPS URL. Note: multi-segment refs containing / are unsupported
		// as strings.SplitN with 4 parts treats the 3rd slash-delimited token as ref.
		rawURL := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s", owner, repo, ref, path)
		return rawURL, true, nil

	default:
		return "", false, fmt.Errorf("unsupported scheme %q in source URI %q", parsed.Scheme, trimmed)
	}
}
