package artifacthub

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"testing"

	"github.com/meshery/schemas/models/v1beta1/model"
)

func TestGetChartUrl(t *testing.T) {
	modelData := `{
		"category": {
		 "name": "Provisioning"
		},
		"displayName": "Azure Active Directory (AAD)",
		"id": "00000000-0000-0000-0000-000000000000",
		"metadata": {
		 "capabilities": null,
		 "defaultData": "fasfasfasfsaf",
		 "isAnnotation": false,
		 "primaryColor": "#1988d9",
		 "secondaryColor": "#54aef0",
		 "shape": "rectangle",
		 "shapePolygonPoints": "afnasfj",
		 "styleOverrides": "{ajfs\"shape-polygon-points\":\"afnasfj\",\"data\":fasfasfasfsaf}",
		 "styles": "ajfs",
		 "svgColor": "\u003c?xml version=\"1.0\" encoding=\"UTF-8\"?\u003e\u003c!DOCTYPE svg\u003e\u003csvg xmlns=\"http://www.w3.org/2000/svg\" id=\"bdb56329-4717-4410-aa13-4505ecaa4e46\" width=\"20\" height=\"20\" viewBox=\"0 0 18 18\"\u003e\u003cdefs xmlns=\"http://www.w3.org/2000/svg\"\u003e\u003clinearGradient xmlns=\"http://www.w3.org/2000/svg\" id=\"ba2610c3-a45a-4e7e-a0c0-285cfd7e005d\" x1=\"13.25\" y1=\"13.02\" x2=\"8.62\" y2=\"4.25\" gradientUnits=\"userSpaceOnUse\"\u003e\u003cstop xmlns=\"http://www.w3.org/2000/svg\" offset=\"0\" stop-color=\"#1988d9\"\u003e\u003c/stop\u003e\u003cstop xmlns=\"http://www.w3.org/2000/svg\" offset=\"0.9\" stop-color=\"#54aef0\"\u003e\u003c/stop\u003e\u003c/linearGradient\u003e\u003clinearGradient xmlns=\"http://www.w3.org/2000/svg\" id=\"bd8f618b-4f2f-4cb7-aff0-2fd2d211326d\" x1=\"11.26\" y1=\"10.47\" x2=\"14.46\" y2=\"15.99\" gradientUnits=\"userSpaceOnUse\"\u003e\u003cstop xmlns=\"http://www.w3.org/2000/svg\" offset=\"0.1\" stop-color=\"#54aef0\"\u003e\u003c/stop\u003e\u003cstop xmlns=\"http://www.w3.org/2000/svg\" offset=\"0.29\" stop-color=\"#4fabee\"\u003e\u003c/stop\u003e\u003cstop xmlns=\"http://www.w3.org/2000/svg\" offset=\"0.51\" stop-color=\"#41a2e9\"\u003e\u003c/stop\u003e\u003cstop xmlns=\"http://www.w3.org/2000/svg\" offset=\"0.74\" stop-color=\"#2a93e0\"\u003e\u003c/stop\u003e\u003cstop xmlns=\"http://www.w3.org/2000/svg\" offset=\"0.88\" stop-color=\"#1988d9\"\u003e\u003c/stop\u003e\u003c/linearGradient\u003e\u003c/defs\u003e\u003ctitle xmlns=\"http://www.w3.org/2000/svg\"\u003eIcon-identity-221\u003c/title\u003e\u003cpolygon xmlns=\"http://www.w3.org/2000/svg\" points=\"1.01 10.19 8.93 15.33 16.99 10.17 18 11.35 8.93 17.19 0 11.35 1.01 10.19\" fill=\"#50e6ff\"\u003e\u003c/polygon\u003e\u003cpolygon xmlns=\"http://www.w3.org/2000/svg\" points=\"1.61 9.53 8.93 0.81 16.4 9.54 8.93 14.26 1.61 9.53\" fill=\"#fff\"\u003e\u003c/polygon\u003e\u003cpolygon xmlns=\"http://www.w3.org/2000/svg\" points=\"8.93 0.81 8.93 14.26 1.61 9.53 8.93 0.81\" fill=\"#50e6ff\"\u003e\u003c/polygon\u003e\u003cpolygon xmlns=\"http://www.w3.org/2000/svg\" points=\"8.93 0.81 8.93 14.26 16.4 9.54 8.93 0.81\" fill=\"url(#ba2610c3-a45a-4e7e-a0c0-285cfd7e005d)\"\u003e\u003c/polygon\u003e\u003cpolygon xmlns=\"http://www.w3.org/2000/svg\" points=\"8.93 7.76 16.4 9.54 8.93 14.26 8.93 7.76\" fill=\"#53b1e0\"\u003e\u003c/polygon\u003e\u003cpolygon xmlns=\"http://www.w3.org/2000/svg\" points=\"8.93 14.26 1.61 9.53 8.93 7.76 8.93 14.26\" fill=\"#9cebff\"\u003e\u003c/polygon\u003e\u003cpolygon xmlns=\"http://www.w3.org/2000/svg\" points=\"8.93 17.19 18 11.35 16.99 10.17 8.93 15.33 8.93 17.19\" fill=\"url(#bd8f618b-4f2f-4cb7-aff0-2fd2d211326d)\"\u003e\u003c/polygon\u003e\u003c/svg\u003e",
		 "svgComplete": "",
		 "svgWhite": "\u003c?xml version=\"1.0\" encoding=\"UTF-8\"?\u003e\u003c!DOCTYPE svg\u003e\u003csvg xmlns=\"http://www.w3.org/2000/svg\" id=\"bdb56329-4717-4410-aa13-4505ecaa4e46\" width=\"20\" height=\"20\" viewBox=\"0 0 18 18\"\u003e \u003cdefs xmlns=\"http://www.w3.org/2000/svg\"\u003e \u003clinearGradient xmlns=\"http://www.w3.org/2000/svg\" id=\"ba2610c3-a45a-4e7e-a0c0-285cfd7e005d\" x1=\"13.25\" y1=\"13.02\" x2=\"8.62\" y2=\"4.25\" gradientUnits=\"userSpaceOnUse\"\u003e \u003cstop xmlns=\"http://www.w3.org/2000/svg\" offset=\"0\" stop-color=\"#FFF\"\u003e\u003c/stop\u003e \u003cstop xmlns=\"http://www.w3.org/2000/svg\" offset=\"0.9\" stop-color=\"#FFF\"\u003e\u003c/stop\u003e \u003c/linearGradient\u003e \u003clinearGradient xmlns=\"http://www.w3.org/2000/svg\" id=\"bd8f618b-4f2f-4cb7-aff0-2fd2d211326d\" x1=\"11.26\" y1=\"10.47\" x2=\"14.46\" y2=\"15.99\" gradientUnits=\"userSpaceOnUse\"\u003e \u003cstop xmlns=\"http://www.w3.org/2000/svg\" offset=\"0.1\" stop-color=\"#FFF\"\u003e\u003c/stop\u003e \u003cstop xmlns=\"http://www.w3.org/2000/svg\" offset=\"0.29\" stop-color=\"#FFF\"\u003e\u003c/stop\u003e \u003cstop xmlns=\"http://www.w3.org/2000/svg\" offset=\"0.51\" stop-color=\"#FFF\"\u003e\u003c/stop\u003e \u003cstop xmlns=\"http://www.w3.org/2000/svg\" offset=\"0.74\" stop-color=\"#FFF\"\u003e\u003c/stop\u003e \u003cstop xmlns=\"http://www.w3.org/2000/svg\" offset=\"0.88\" stop-color=\"#FFF\"\u003e\u003c/stop\u003e \u003c/linearGradient\u003e \u003c/defs\u003e \u003ctitle xmlns=\"http://www.w3.org/2000/svg\"\u003eIcon-identity-221\u003c/title\u003e \u003cpolygon xmlns=\"http://www.w3.org/2000/svg\" points=\"1.01 10.19 8.93 15.33 16.99 10.17 18 11.35 8.93 17.19 0 11.35 1.01 10.19\" fill=\"#FFF\"\u003e\u003c/polygon\u003e \u003cpolygon xmlns=\"http://www.w3.org/2000/svg\" points=\"1.61 9.53 8.93 0.81 16.4 9.54 8.93 14.26 1.61 9.53\" fill=\"#fff\"\u003e\u003c/polygon\u003e \u003cpolygon xmlns=\"http://www.w3.org/2000/svg\" points=\"8.93 0.81 8.93 14.26 1.61 9.53 8.93 0.81\" fill=\"#FFF\"\u003e\u003c/polygon\u003e \u003cpolygon xmlns=\"http://www.w3.org/2000/svg\" points=\"8.93 0.81 8.93 14.26 16.4 9.54 8.93 0.81\" fill=\"url(#ba2610c3-a45a-4e7e-a0c0-285cfd7e005d)\"\u003e\u003c/polygon\u003e \u003cpolygon xmlns=\"http://www.w3.org/2000/svg\" points=\"8.93 7.76 16.4 9.54 8.93 14.26 8.93 7.76\" fill=\"#FFF\"\u003e\u003c/polygon\u003e \u003cpolygon xmlns=\"http://www.w3.org/2000/svg\" points=\"8.93 14.26 1.61 9.53 8.93 7.76 8.93 14.26\" fill=\"#FFF\"\u003e\u003c/polygon\u003e \u003cpolygon xmlns=\"http://www.w3.org/2000/svg\" points=\"8.93 17.19 18 11.35 16.99 10.17 8.93 15.33 8.93 17.19\" fill=\"url(#bd8f618b-4f2f-4cb7-aff0-2fd2d211326d)\"\u003e\u003c/polygon\u003e \u003c/svg\u003e"
		},
		"model": {
		 "version": "4.1.18"
		},
		"name": "aad-pod-identity",
		"registrant": {
		 "created_at": "0001-01-01T00:00:00Z",
		 "credential_id": "00000000-0000-0000-0000-000000000000",
		 "deleted_at": "0001-01-01T00:00:00Z",
		 "id": "00000000-0000-0000-0000-000000000000",
		 "kind": "artifacthub",
		 "name": "Artifact Hub",
		 "status": "registered",
		 "sub_type": "",
		 "type": "registry",
		 "updated_at": "0001-01-01T00:00:00Z",
		 "user_id": "00000000-0000-0000-0000-000000000000"
		},
		"connection_id": "00000000-0000-0000-0000-000000000000",
		"schemaVersion": "models.meshery.io/v1beta1",
		"status": "enabled",
		"subCategory": "Security \u0026 Compliance",
		"version": "v1.0.0",
		"components": null,
		"relationships": null
	   }`
	var model model.ModelDefinition
	err := json.Unmarshal([]byte(modelData), &model)
	if err != nil {
		panic(err)
	}
	var tests = []struct {
		ahpkg AhPackage
		want  string
	}{
		// these might change in the future, so the tests have to be changed as well when the urls change
		// because the urls will change with every new version update to the package
		{AhPackage{Name: "consul", Repository: "bitnami", Organization: "", RepoUrl: "https://charts.bitnami.com/bitnami"}, "https://charts.bitnami.com/bitnami/consul"},
		{AhPackage{Name: "crossplane-types", Repository: "crossplane", Organization: "", RepoUrl: "https://charts.crossplane.io/master"}, "https://charts.crossplane.io/master/crossplane-types-0.13.0-rc.191.g3a18fb7.tgz"},
	}
	for _, tt := range tests {
		t.Run("UpdatePackageData", func(t *testing.T) {
			err := tt.ahpkg.UpdatePackageData()
			if err != nil {
				t.Errorf("error while updating package data = %v", err)
				return
			}
			if !strings.HasPrefix(tt.ahpkg.ChartUrl, tt.want) {
				t.Errorf("got %v, want %v", tt.ahpkg.ChartUrl, tt.want)
			}

			comps, err := tt.ahpkg.GenerateComponents(model)
			if err != nil {
				fmt.Println(err)
				t.Errorf("error while generating components: %v", err)
				return
			}
			for _, comp := range comps {
				dirName := "./" + comp.Model.Name
				_, err := os.Stat(dirName)
				if errors.Is(err, os.ErrNotExist) {
					err = os.Mkdir(dirName, fs.ModePerm)
					if err != nil {
						t.Errorf("err creating dir : %v", err)
					}
				}
				byt, _ := json.MarshalIndent(comp, "", "")

				f, err := os.Create(dirName + "/" + comp.Component.Kind + ".json")
				if err != nil {
					t.Errorf("error create file : %v", err)
					continue
				}
				_, _ = f.Write(byt)
			}
		})
	}
}
