package registry

import (
    "os"
    "path/filepath" 
    "testing"
    "github.com/stretchr/testify/assert"
)

func TestEndToEndComponentGeneration(t *testing.T) {
    tempDir, err := os.MkdirTemp("", "meshkit-test")
    assert.NoError(t, err)
    defer os.RemoveAll(tempDir)
    
    csvContent := `registrant,model,component,capabilities,primaryColor,shape
meshery,test-model,TestComponent,,#00B39F,circle
meshery,test-model,TestComponent2,null,#FF6B6B,rectangle`
    
    csvPath := filepath.Join(tempDir, "components.csv")
    err = os.WriteFile(csvPath, []byte(csvContent), 0644)
    assert.NoError(t, err)
    
    helper, err := NewComponentCSVHelper("", "test", 0, csvPath)
    assert.NoError(t, err)
    
    err = helper.ParseComponentsSheet("")
    assert.NoError(t, err)
    
    assert.Contains(t, helper.Components, "meshery")
    assert.Contains(t, helper.Components["meshery"], "test-model")
    assert.Len(t, helper.Components["meshery"]["test-model"], 2)
    
    for _, comp := range helper.Components["meshery"]["test-model"] {
        compDef, err := comp.CreateComponentDefinition(true, "v1.0.0")
        assert.NoError(t, err)
        
\        assert.NotNil(t, compDef.Capabilities)
        assert.Len(t, *compDef.Capabilities, 3)
        
        capabilities := *compDef.Capabilities
        assert.Equal(t, "Styling", capabilities[0].DisplayName)
        assert.Equal(t, "Change Shape", capabilities[1].DisplayName)
        assert.Equal(t, "Compound Drag And Drop", capabilities[2].DisplayName)
    }
}