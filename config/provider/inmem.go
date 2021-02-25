// Copyright 2021 Layer5, Inc.
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

package provider

import (
	"sync"

	"github.com/layer5io/meshkit/config"
	"github.com/layer5io/meshkit/utils"
)

// Type InMem implements the config interface Handler for an in-memory configuration registry.
type InMem struct {
	store map[string]string
	mutex sync.Mutex
}

// NewInMem returns a new instance of an in-memory configuration provider using the provided Options opts.
func NewInMem(opts Options) (config.Handler, error) {
	return &InMem{
		store: make(map[string]string),
	}, nil
}

// -------------------------------------------Application config methods----------------------------------------------------------------

// SetKey sets a key value in local store
func (l *InMem) SetKey(key string, value string) {
	l.mutex.Lock()
	l.store[key] = value
	l.mutex.Unlock()
}

// GetKey gets a key value from local store
func (l *InMem) GetKey(key string) string {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	return l.store[key]
}

// GetObject gets an object value for the key
func (l *InMem) GetObject(key string, result interface{}) error {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	return utils.Unmarshal(l.store[key], result)
}

// SetObject sets an object value for the key
func (l *InMem) SetObject(key string, value interface{}) error {
	l.mutex.Lock()
	val, err := utils.Marshal(value)
	defer l.mutex.Unlock()
	if err != nil {
		return config.ErrInMem(err)
	}
	l.store[key] = val
	return nil
}
