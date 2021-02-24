// Copyright 2020 Layer5, Inc.
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

// Package config provides the interface Handler and errors related to the configuration of adapters.
package config

// Interface Handler is the interface to be implemented by config providers used by adapters.
//
// Provided implementations can be found in the package config/provider.
type Handler interface {
	// SetKey is used to set a string value for a given key.
	SetKey(key string, value string)

	// GetKey is used to retrieve a string value for a given key.
	GetKey(key string) string

	// GetObject is used to retrieve an object for a given key and a given interface representing that object in result.
	// An example of such an object is map[string]string. These objects can e.g. be set in the factory function for a specific
	// config provider implementation.
	GetObject(key string, result interface{}) error

	SetObject(key string, value interface{}) error
}
