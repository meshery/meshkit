package kompose

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestFormatComposeFile_RemovesEnvFile(t *testing.T) {
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

	var result DockerComposeFile = composeWithEnvFileString
	formatComposeFile(&result)

	// Unmarshal to verify env_file was removed
	var parsed map[string]interface{}
	err := yaml.Unmarshal(result, &parsed)
	if err != nil {
		t.Fatalf("Failed to unmarshal formatted compose file: %v", err)
	}

	// Check services exist
	services, ok := parsed["services"].(map[string]interface{})
	if !ok {
		t.Fatal("Services not found in formatted compose file")
	}

	// Check env_file was removed from web service
	webService, ok := services["web"].(map[string]interface{})
	if !ok {
		t.Fatal("Web service not found")
	}
	if _, exists := webService["env_file"]; exists {
		t.Error("env_file should be removed from web service")
	}
	// Verify environment is still present
	if _, exists := webService["environment"]; !exists {
		t.Error("environment should still be present in web service")
	}

	// Check env_file was removed from db service
	dbService, ok := services["db"].(map[string]interface{})
	if !ok {
		t.Fatal("DB service not found")
	}
	if _, exists := dbService["env_file"]; exists {
		t.Error("env_file should be removed from db service")
	}
}

func TestFormatComposeFile_RemovesEnvFileArray(t *testing.T) {
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

	var result DockerComposeFile = composeWithEnvFileArray
	formatComposeFile(&result)

	// Unmarshal to verify env_file was removed
	var parsed map[string]interface{}
	err := yaml.Unmarshal(result, &parsed)
	if err != nil {
		t.Fatalf("Failed to unmarshal formatted compose file: %v", err)
	}

	services, ok := parsed["services"].(map[string]interface{})
	if !ok {
		t.Fatal("Services not found in formatted compose file")
	}

	appService, ok := services["app"].(map[string]interface{})
	if !ok {
		t.Fatal("App service not found")
	}
	if _, exists := appService["env_file"]; exists {
		t.Error("env_file array should be removed from app service")
	}
	// Verify environment is still present
	if _, exists := appService["environment"]; !exists {
		t.Error("environment should still be present in app service")
	}
}

func TestFormatComposeFile_PreservesOtherFields(t *testing.T) {
	// Test case 3: Ensure other fields are preserved
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

	// Verify env_file is removed
	if _, exists := webService["env_file"]; exists {
		t.Error("env_file should be removed")
	}

	// Verify other fields are preserved
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

func TestFormatComposeFile_NoEnvFile(t *testing.T) {
	// Test case 4: Compose file without env_file (should work normally)
	composeWithoutEnvFile := []byte(`
version: "3.8"
services:
  web:
    image: nginx:latest
    environment:
      APP_ENV: production
`)

	var result DockerComposeFile = composeWithoutEnvFile
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

	// Verify environment is preserved
	if _, exists := webService["environment"]; !exists {
		t.Error("environment should be preserved")
	}
}

func TestFormatComposeFile_EmptyServices(t *testing.T) {
	// Test case 5: Compose file with no services
	composeEmptyServices := []byte(`
version: "3.8"
services:
`)

	var result DockerComposeFile = composeEmptyServices
	formatComposeFile(&result)

	// Should not panic and should complete successfully
	var parsed map[string]interface{}
	err := yaml.Unmarshal(result, &parsed)
	if err != nil {
		t.Fatalf("Failed to unmarshal formatted compose file: %v", err)
	}
}
