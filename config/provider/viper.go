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
	"fmt"
	"sync"

	"github.com/layer5io/meshkit/config"
	"github.com/spf13/viper"
)

const (
	FilePath = "filepath"
	FileType = "filetype"
	FileName = "filename"
)

// Type Viper implements the config interface Handler for a Viper configuration registry.
type Viper struct {
	instance *viper.Viper
	mutex    sync.Mutex
}

// NewViper returns a new instance of a Viper configuration provider using the provided Options opts.
func NewViper(opts Options) (config.Handler, error) {
	v := viper.New()
	v.AddConfigPath(opts.FilePath)
	v.SetConfigType(opts.FileType)
	v.SetConfigName(opts.FileName)
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Config file not found; ignore error
			// Hack until viper issue #433 is fixed
			er := v.WriteConfigAs(fmt.Sprintf("%s/%s.%s", opts.FilePath, opts.FileName, opts.FileType))
			if er != nil {
				return nil, config.ErrViper(err)
			}
			_ = v.WriteConfig()
		} else {
			// Config file was found but another error was produced
			return nil, config.ErrViper(err)
		}
	}

	return &Viper{
		instance: v,
	}, nil
}

func (v *Viper) SetKey(key string, value string) {
	v.mutex.Lock()
	v.instance.Set(key, value)
	_ = v.instance.WriteConfig()
	v.mutex.Unlock()
}

func (v *Viper) GetKey(key string) string {
	v.mutex.Lock()
	_ = v.instance.ReadInConfig()
	defer v.mutex.Unlock()
	return v.instance.Get(key).(string)
}

func (v *Viper) GetObject(key string, result interface{}) error {
	v.mutex.Lock()
	_ = v.instance.ReadInConfig()
	err := v.instance.UnmarshalKey(key, &result)
	defer v.mutex.Unlock()
	if err != nil {
		return config.ErrViper(err)
	}
	return err
}

func (v *Viper) SetObject(key string, value interface{}) error {
	v.mutex.Lock()
	v.instance.Set(key, value)
	err := v.instance.WriteConfig()
	defer v.mutex.Unlock()
	if err != nil {
		return config.ErrViper(err)
	}

	return nil
}
