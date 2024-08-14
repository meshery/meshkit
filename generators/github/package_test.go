package github

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/meshery/schemas/models/v1beta1/model"
)

func TestGenerateCompFromGitHub(t *testing.T) {
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
		ghPackageManager GitHubPackageManager
		want             int
	}{
		// { // Source pointing to a directory
		// 	ghPackageManager: GitHubPackageManager{
		// 		PackageName: "k8s-config-connector",
		// 		SourceURL:   "git://github.com/GoogleCloudPlatform/k8s-config-connector/master/crds/",
		// 	},
		// 	want: 337,
		// },
		// { // Source pointing to a file in a repo
		// ghPackageManager: GitHubPackageManager{
		// 		PackageName: "k8s-config-connector",
		// 		SourceURL:   "git://github.com/GoogleCloudPlatform/k8s-config-connector/master/crds/accesscontextmanager_v1alpha1_accesscontextmanageraccesslevelcondition.yaml",
		// 	},
		// 	want: 1,
		// },

		// { // Source pointing to a directly downloadable file (not a repo per se)
		// 	ghPackageManager: GitHubPackageManager{
		// 		PackageName: "k8s-config-connector",
		// 		SourceURL:   "git://github.com/GoogleCloudPlatform/k8s-config-connector/master/crds/accesscontextmanager_v1alpha1_accesscontextmanageraccesslevelcondition.yaml",
		// 	},
		// 	want: 1,
		// },

		{ // Source pointing to a directly downloadable file (not a repo per se)
			ghPackageManager: GitHubPackageManager{
				PackageName: "k8s-config-connector",
				SourceURL:   "https://raw.githubusercontent.com/GoogleCloudPlatform/k8s-config-connector/master/crds/alloydb_v1beta1_alloydbbackup.yaml/1.113.0",
			},
			want: 1,
		},

		// { // Source pointing to a directory containing helm chart
		// 	ghPackageManager: GitHubPackageManager{
		// 		PackageName: "acm-controller",
		// 		SourceURL:   "https://meshery.github.io/meshery.io/charts/meshery-v0.7.12.tgz/v0.7.12",
		// 	},
		// 	want: 2,
		// },
		{ // Source pointing to a zip containing manifests but no CRDs
			ghPackageManager: GitHubPackageManager{
				PackageName: "acm-controller",
				SourceURL:   "https://github.com/MUzairS15/WASM-filters/raw/main/test.tar.gz/v0.7.12",
			},
			want: 0,
		},
		{ // Source pointing to a zip containing CRDs
			ghPackageManager: GitHubPackageManager{
				PackageName: "acm-controller",
				SourceURL:   "git://github.com/MUzairS15/WASM-filters/main/chart.tar.gz",
			},
			want: 14,
		},
		// { // Source pointing to a dir containing CRDs
		// 	ghPackageManager: GitHubPackageManager{
		// 		PackageName: "acm-controller",
		// 		SourceURL:   "git://github.com/meshery/meshery/master/install/kubernetes/helm/meshery-operator",
		// 	},
		// 	want: 2,
		// },
	}

	for _, test := range tests {
		t.Run("GenerateComponents", func(t *testing.T) {

			pkg, err := test.ghPackageManager.GetPackage()
			if err != nil {
				t.Errorf("error while getting package: %v", err)
				return
			}
			comps, err := pkg.GenerateComponents(model)
			if err != nil {
				fmt.Println(err)
				t.Errorf("error while generating components: %v", err)
				return
			}
			for _, comp := range comps {
				currentDirectory, _ := os.Getwd()
				dirName := filepath.Join(currentDirectory, comp.Model.Name)
				_, err := os.Stat(dirName)
				if errors.Is(err, os.ErrNotExist) {
					err = os.Mkdir(dirName, fs.ModePerm)
					if err != nil {
						t.Errorf("error creating dir at %s: %v", dirName, err)
						return
					}
				}
				byt, _ := json.MarshalIndent(comp, "", "")

				f, err := os.Create(fmt.Sprintf("%s/%s%s", dirName, comp.Component.Kind, ".json"))
				if err != nil {
					t.Errorf("error creating file for %s: %v", comp.Component.Kind, err)
					continue
				}
				_, _ = f.Write(byt)
			}
			t.Log("generated ", len(comps), "want: ", test.want)
			if len(comps) != test.want {
				t.Errorf("generated %d, want %d", len(comps), test.want)
				return
			}
		})
	}
}
