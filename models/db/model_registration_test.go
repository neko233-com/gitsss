// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package db

import "testing"

func TestRegisterModelInitializer(t *testing.T) {
	models, callbacks := registeredModels, registeredInitFuncs
	registeredModels, registeredInitFuncs = nil, nil
	t.Cleanup(func() { registeredModels, registeredInitFuncs = models, callbacks })
	called := false
	RegisterModel(new(struct{ ID int64 }), func() error { called = true; return nil })
	RegisterModel(new(struct{ ID int64 }))
	RegisterModel(new(struct{ ID int64 }), nil)
	if len(registeredModels) != 3 || len(registeredInitFuncs) != 1 || called {
		t.Fatal("model initialization must be deferred and independent of earlier callbacks")
	}
	if err := registeredInitFuncs[0](); err != nil || !called {
		t.Fatal("the first registered initializer was lost")
	}
}
