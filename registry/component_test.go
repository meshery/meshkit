package registry

import (
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/meshery/schemas/models/v1beta1/component"
)

func TestUpdateCompDefinitionWithDefaultCapabilities(t *testing.T) {
    tests := []struct {
        name                    string
        csvCapabilities        string
        expectedCapabilitiesLen int
        shouldHaveDefaultCaps   bool
    }{
        {
            name:                    "Empty capabilities should get defaults",
            csvCapabilities:        "",
            expectedCapabilitiesLen: 3,
            shouldHaveDefaultCaps:   true,
        },
        {
            name:                    "Null capabilities should get defaults", 
            csvCapabilities:        "null",
            expectedCapabilitiesLen: 3,
            shouldHaveDefaultCaps:   true,
        },
        {
            name:                    "Existing capabilities should be preserved",
            csvCapabilities:        `[{"displayName":"Custom Cap","kind":"test"}]`,
            expectedCapabilitiesLen: 1,
            shouldHaveDefaultCaps:   false,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            comp := ComponentCSV{
                Component:    "TestComponent",
                Capabilities: tt.csvCapabilities,
                Registrant:   "meshery",
                Model:        "test-model",
            }
            
            compDef := &component.ComponentDefinition{}
            
            err := comp.UpdateCompDefinition(compDef)
            
            assert.NoError(t, err)
            assert.NotNil(t, compDef.Capabilities)
            assert.Len(t, *compDef.Capabilities, tt.expectedCapabilitiesLen)
            
            if tt.shouldHaveDefaultCaps {
                capabilities := *compDef.Capabilities
                
                expectedNames := []string{"Styling", "Change Shape", "Compound Drag And Drop"}
                actualNames := make([]string, len(capabilities))
                for i, cap := range capabilities {
                    actualNames[i] = cap.DisplayName
                }
                
                assert.ElementsMatch(t, expectedNames, actualNames)
                
                assert.Equal(t, "Styling", capabilities[0].DisplayName)
                assert.Equal(t, "mutate", capabilities[0].Kind)
                assert.Equal(t, "style", capabilities[0].Type)
            }
        })
    }
}

func TestGetMinimalUICapabilitiesFromSchema(t *testing.T) {
    capabilities, err := getMinimalUICapabilitiesFromSchema()
    
    assert.NoError(t, err)
    assert.Len(t, capabilities, 3)
    
    expectedNames := []string{"Styling", "Change Shape", "Compound Drag And Drop"}
    actualNames := make([]string, len(capabilities))
    for i, cap := range capabilities {
        actualNames[i] = cap.DisplayName
    }
    
    assert.ElementsMatch(t, expectedNames, actualNames)
}