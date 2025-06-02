// Copyright 2021 Meshery Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package config

import (
	"github.com/meshery/meshkit/errors"
)

var (
	ErrEmptyConfigCode = "meshkit-11123"
	ErrViperCode       = "meshkit-11124"
	ErrInMemCode       = "meshkit-11125"

	// ErrEmptyConfig is returned when the config has not been initialized.
	ErrEmptyConfig = errors.New(ErrEmptyConfigCode, errors.Alert, []string{"Config not initialized"}, []string{}, []string{"Viper is crashing"}, []string{"Make sure viper is configured properly"})
)

// ErrViper returns a MeshKit error indicating an (initialization) error in the Viper provider.
func ErrViper(err error) error {
	return errors.New(ErrViperCode, errors.Fatal, []string{"Viper configuration initialization failed"}, []string{err.Error()}, []string{"Viper is crashing"}, []string{"Make sure viper is configured properly"})
}

// ErrViper returns a MeshKit error indicating an (initialization) error in the in-memory provider.
func ErrInMem(err error) error {
	return errors.New(ErrInMemCode, errors.Fatal, []string{"InMem configuration initialization failed"}, []string{err.Error()}, []string{"In memory map is crashing"}, []string{"Make sure map is configured properly"})
}
