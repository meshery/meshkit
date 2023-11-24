package factory

import (	
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/layer5io/meshkit/models/meshmodel/core/v1alpha1"
	"github.com/layer5io/meshkit/utils/artifacthub"
	"gopkg.in/yaml.v3"
)

const dumpFile = "./dump.csv"
const COLUMNRANGE = "!A:AF3" //Update this on addition of new columns


// Stages have to run sequentially. The steps within each stage can be concurrent.
// pipeline function should return only after completion
func executeInStages(
	pipeline func(
		in chan []artifacthub.AhPackage, 
		csv chan string, 
	modelChan chan v1alpha1.ModelChannel, 
	dp *dedup) error,
	csv chan string,
	modelChan chan v1alpha1.ModelChannel, 
	dp *dedup, 
	pkg ...[]artifacthub.AhPackage) {
	for stageno, p := range pkg {
		input := make(chan []artifacthub.AhPackage)
		go func() {
			for len(p) != 0 {
				x := 50
				if len(p) < x {
					x = len(p)
				}
				input <- p[:x]
				p = p[x:]
			}
			close(input)
		}()
		var wg sync.WaitGroup
		for i := 1; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				pipeline(input, csv, modelChan, dp) //synchronous
				fmt.Println("Pipeline exited for a go routine")
			}()
		}
		wg.Wait()
		fmt.Println("[DEBUG] Completed stage", stageno)
	}
}

type dedup struct {
	m  map[string]bool
	mx sync.Mutex
}

// used in generator.go
func Newdedup() *dedup {
	return &dedup{
		m: make(map[string]bool),
	}
}

func (d *dedup) set(key string) {
	d.mx.Lock()
	defer d.mx.Unlock()
	d.m[key] = true
}
func (d *dedup) check(key string) bool {
	return d.m[key]
}

// used in generator.go
func StartPipeline(in chan []artifacthub.AhPackage, csv chan string, modelChan chan v1alpha1.ModelChannel, dp *dedup) error {
	pkgsChan := make(chan []artifacthub.AhPackage)
	compsChan := make(chan struct {
		comps []v1alpha1.ComponentDefinition
		model string
	})
	compsCSV := make(chan struct {
		comps []v1alpha1.ComponentDefinition
		model string
	})
	// updating pacakge data
	go func() {
		for pkgs := range in {
			ahPkgs := make([]artifacthub.AhPackage, 0)
			for _, ap := range pkgs {
				fmt.Println("[DEBUG] Updating package data for: ", ap.Name)
				err := ap.UpdatePackageData()
				if err != nil {
					fmt.Println(err)
					continue
				}
				ahPkgs = append(ahPkgs, ap)
				pkgsChan <- ahPkgs
			}
		}
		close(pkgsChan)
	}()
	// writer
	go func() {
		for modelcomps := range compsChan {
			err := writeComponents(modelcomps.comps)
			if err != nil {
				fmt.Println(err)
			}
			compsCSV <- struct {
				comps []v1alpha1.ComponentDefinition
				model string
			}{
				comps: modelcomps.comps,
				model: modelcomps.model,
			}
		}
	}()
	if _, err := os.Stat(dumpFile); os.IsExist(err) {
		// If file exists, delete it
		err := os.Remove(dumpFile)
		if err != nil {
			fmt.Printf("Error deleting file: %s\n", err)
		}
	}

	go func() {
		for comps := range compsCSV {
			count := len(comps.comps)
			names := "\""
			for _, cmp := range comps.comps {
				names += fmt.Sprintf("%s,", cmp.Kind)
			}
			names = strings.TrimSuffix(names, ",")
			names += "\""
			if count > 0 {
				model := comps.model
				fmt.Println(fmt.Sprintf("[DEBUG]Adding to CSV: %s", model))
				csv <- fmt.Sprintf("%s,%d,%s\n", model, count, names)
			}
		}
	}()
	for pkgs := range pkgsChan {
		for _, ap := range pkgs {
			fmt.Printf("[DEBUG] Generating components for: %s with verified status %v\n", ap.Name, ap.VerifiedPublisher)
			comps, err := ap.GenerateComponents()
			if err != nil {
				fmt.Println(err)
				continue
			}
			var newcomps []v1alpha1.ComponentDefinition
			for _, comp := range comps {
				key := fmt.Sprintf("%sMESHERY%s", comp.Kind, comp.APIVersion)
				if !dp.check(key) {
					fmt.Println("SETTING FOR: ", key)
					newcomps = append(newcomps, comp)
					dp.set(key)
				}
			}
			compsCSV <- struct {
				comps []v1alpha1.ComponentDefinition
				model string
			}{
				comps: newcomps,
				model: ap.Name,
			}
			compsChan <- struct {
				comps []v1alpha1.ComponentDefinition
				model string
			}{
				comps: newcomps,
				model: ap.Name,
			}
			modelChan <- v1alpha1.ModelChannel{
				Comps:   newcomps,
				Model:   ap.Name,
				HelmURL: ap.ChartUrl,
			}
		}
	}
	return nil
}

type Writer struct {
	file *os.File
	m    sync.Mutex
}

// used in generator.go
func WriteComponentModels(models []artifacthub.AhPackage, writer *Writer) error {
	writer.m.Lock()
	defer writer.m.Unlock()
	val, err := yaml.Marshal(models)
	if err != nil {
		return err
	}
	_, err = writer.file.Write(val)
	if err != nil {
		return err
	}
	return nil
}

func writeComponents(cmps []v1alpha1.ComponentDefinition) error {
	for _, comp := range cmps {
		modelPath := filepath.Join(OutputDirectoryPath, comp.Model.Name)
		if _, err := os.Stat(modelPath); errors.Is(err, os.ErrNotExist) {
			err := os.Mkdir(modelPath, os.ModePerm)
			if err != nil {
				return err
			}
			fmt.Println("created directory ", comp.Model.Name)
		}
		componentPath := filepath.Join(modelPath, comp.Model.Version)
		if _, err := os.Stat(componentPath); errors.Is(err, os.ErrNotExist) {
			err := os.Mkdir(componentPath, os.ModePerm)
			if err != nil {
				return err
			}
			fmt.Println("created versioned directory ", comp.Model.Version)
		}
		relationshipsPath := filepath.Join(modelPath, "relationships")
		policiesPath := filepath.Join(modelPath, "policies")
		if _, err := os.Stat(relationshipsPath); errors.Is(err, os.ErrNotExist) {
			err := os.Mkdir(relationshipsPath, os.ModePerm)
			if err != nil {
				return err
			}
		}
		if _, err := os.Stat(policiesPath); errors.Is(err, os.ErrNotExist) {
			err := os.Mkdir(policiesPath, os.ModePerm)
			if err != nil {
				return err
			}
		}
		f, err := os.Create(filepath.Join(componentPath, comp.Kind+".json"))
		if err != nil {
			return err
		}
		byt, err := json.Marshal(comp)
		if err != nil {
			return err
		}
		_, err = f.Write(byt)
		if err != nil {
			return err
		}
	}
	return nil
}
