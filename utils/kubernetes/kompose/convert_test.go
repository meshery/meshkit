package kompose

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestExtractEnvFileReferences_String(t *testing.T) {
	// Test case 1: Docker compose with env_file as string
	composeWithEnvFileString := []byte(`
version: "3.8"
services:
  web:
    image: nginx:latest
    env_file: .env
    environment:
      APP_ENV: production
  db:
    image: postgres:13
    env_file: ./config/.env
    environment:
      POSTGRES_USER: admin
`)

	envFiles := extractEnvFileReferences(composeWithEnvFileString)
	if len(envFiles) != 2 {
		t.Errorf("Expected 2 env files, got %d", len(envFiles))
	}
	
	expectedFiles := map[string]bool{
		".env":          false,
		"./config/.env": false,
	}
	
	for _, file := range envFiles {
		if _, exists := expectedFiles[file]; exists {
			expectedFiles[file] = true
		}
	}
	
	for file, found := range expectedFiles {
		if !found {
			t.Errorf("Expected env file %s not found", file)
		}
	}
}

func TestExtractEnvFileReferences_Array(t *testing.T) {
	// Test case 2: Docker compose with env_file as array
	composeWithEnvFileArray := []byte(`
version: "3.8"
services:
  app:
    image: myapp:latest
    env_file:
      - .env
      - .env.local
      - ./config/.env.prod
    environment:
      NODE_ENV: production
`)

	envFiles := extractEnvFileReferences(composeWithEnvFileArray)
	if len(envFiles) != 3 {
		t.Errorf("Expected 3 env files, got %d", len(envFiles))
	}
	
	expectedFiles := map[string]bool{
		".env":               false,
		".env.local":         false,
		"./config/.env.prod": false,
	}
	
	for _, file := range envFiles {
		if _, exists := expectedFiles[file]; exists {
			expectedFiles[file] = true
		}
	}
	
	for file, found := range expectedFiles {
		if !found {
			t.Errorf("Expected env file %s not found", file)
		}
	}
}

func TestExtractEnvFileReferences_NoEnvFile(t *testing.T) {
	// Test case 3: Compose file without env_file
	composeWithoutEnvFile := []byte(`
version: "3.8"
services:
  web:
    image: nginx:latest
    environment:
      APP_ENV: production
`)

	envFiles := extractEnvFileReferences(composeWithoutEnvFile)
	if len(envFiles) != 0 {
		t.Errorf("Expected 0 env files, got %d", len(envFiles))
	}
}

func TestCreateEmptyEnvFiles(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "test-meshery-")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)
	
	envFiles := []string{
		".env",
		"config/.env.prod",
		"nested/dir/.env.local",
	}
	
	createdFiles, err := createEmptyEnvFiles(tempDir, envFiles)
	if err != nil {
		t.Fatalf("Failed to create empty env files: %v", err)
	}
	
	if len(createdFiles) != 3 {
		t.Errorf("Expected 3 created files, got %d", len(createdFiles))
	}
	
	// Verify files were created
	for _, envFile := range envFiles {
		fullPath := filepath.Join(tempDir, envFile)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			t.Errorf("Expected file %s to exist", fullPath)
		}
	}
}

func TestCreateEmptyEnvFiles_ExistingFiles(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "test-meshery-")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)
	
	// Pre-create one env file
	existingFile := filepath.Join(tempDir, ".env")
	if err := os.WriteFile(existingFile, []byte("EXISTING=true"), 0644); err != nil {
		t.Fatalf("Failed to create existing file: %v", err)
	}
	
	envFiles := []string{
		".env",
		"config/.env.prod",
	}
	
	createdFiles, err := createEmptyEnvFiles(tempDir, envFiles)
	if err != nil {
		t.Fatalf("Failed to create empty env files: %v", err)
	}
	
	// Should only create the one that didn't exist
	if len(createdFiles) != 1 {
		t.Errorf("Expected 1 created file (skipping existing), got %d", len(createdFiles))
	}
	
	// Verify existing file content wasn't overwritten
	content, err := os.ReadFile(existingFile)
	if err != nil {
		t.Fatalf("Failed to read existing file: %v", err)
	}
	if string(content) != "EXISTING=true" {
		t.Error("Existing file content was overwritten")
	}
}

func TestFormatComposeFile_PreservesEnvFile(t *testing.T) {
	// Test that env_file references are preserved
	composeWithEnvFile := []byte(`
version: "3.8"
services:
  web:
    image: nginx:latest
    env_file: .env
    environment:
      APP_ENV: production
`)

	var result DockerComposeFile = composeWithEnvFile
	formatComposeFile(&result)

	// Unmarshal to verify env_file is preserved
	var parsed map[string]interface{}
	err := yaml.Unmarshal(result, &parsed)
	if err != nil {
		t.Fatalf("Failed to unmarshal formatted compose file: %v", err)
	}

	services, ok := parsed["services"].(map[string]interface{})
	if !ok {
		t.Fatal("Services not found in formatted compose file")
	}

	webService, ok := services["web"].(map[string]interface{})
	if !ok {
		t.Fatal("Web service not found")
	}
	
	// Verify env_file is still present
	if _, exists := webService["env_file"]; !exists {
		t.Error("env_file should be preserved in web service")
	}
	
	// Verify environment is still present
	if _, exists := webService["environment"]; !exists {
		t.Error("environment should still be present in web service")
	}
}

func TestFormatComposeFile_PreservesOtherFields(t *testing.T) {
	// Ensure other fields are preserved
	composeWithVariousFields := []byte(`
version: "3.8"
services:
  web:
    image: nginx:latest
    env_file: .env
    ports:
      - "80:80"
    volumes:
      - ./html:/usr/share/nginx/html
    networks:
      - mynetwork
    depends_on:
      - db
  db:
    image: postgres:13
    environment:
      POSTGRES_USER: admin
networks:
  mynetwork:
volumes:
  dbdata:
`)

	var result DockerComposeFile = composeWithVariousFields
	formatComposeFile(&result)

	var parsed map[string]interface{}
	err := yaml.Unmarshal(result, &parsed)
	if err != nil {
		t.Fatalf("Failed to unmarshal formatted compose file: %v", err)
	}

	services, ok := parsed["services"].(map[string]interface{})
	if !ok {
		t.Fatal("Services not found")
	}

	webService, ok := services["web"].(map[string]interface{})
	if !ok {
		t.Fatal("Web service not found")
	}

	// Verify all fields are preserved
	if _, exists := webService["env_file"]; !exists {
		t.Error("env_file should be preserved")
	}
	if _, exists := webService["ports"]; !exists {
		t.Error("ports should be preserved")
	}
	if _, exists := webService["volumes"]; !exists {
		t.Error("volumes should be preserved")
	}
	if _, exists := webService["networks"]; !exists {
		t.Error("networks should be preserved")
	}
	if _, exists := webService["depends_on"]; !exists {
		t.Error("depends_on should be preserved")
	}

	// Verify networks and volumes at top level are preserved
	if _, exists := parsed["networks"]; !exists {
		t.Error("top-level networks should be preserved")
	}
	if _, exists := parsed["volumes"]; !exists {
		t.Error("top-level volumes should be preserved")
	}
}
