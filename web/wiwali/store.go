package main

import (
	"fmt"
	"log/slog"
	"syscall/js"
)

// indexedDBFS is an implementation of pkg/cache.FS that's backed in IndexedDB.
//
// IndexedDB is a very oldschool API (Events!), so it's a bit of a pain to use,
// and thus this code kinda sucks. Sorry.
type indexedDBFS struct {
	db js.Value
}

func newIndexedDBFS() (*indexedDBFS, error) {
	dbC := make(chan js.Value)
	errC := make(chan error)

	req := js.Global().Get("indexedDB").Call("open", "cache", 3)
	req.Set("onerror", js.FuncOf(func(this js.Value, args []js.Value) any {
		errC <- fmt.Errorf("could not open indexedDB")
		return js.Null()
	}))
	req.Set("onsuccess", js.FuncOf(func(this js.Value, args []js.Value) any {
		db := args[0].Get("target").Get("result")
		dbC <- db
		return js.Null()
	}))
	req.Set("onupgradeneeded", js.FuncOf(func(this js.Value, args []js.Value) any {
		slog.Info("Upgrading IndexedDB...")
		db := args[0].Get("target").Get("result")
		db.Call("createObjectStore", "files", map[string]any{
			"keyPath": "path",
		})
		return js.Null()
	}))
	select {
	case err := <-errC:
		slog.Error("IndexedDB for cache could not be initialized.", "err", err)
		return nil, err
	case db := <-dbC:
		slog.Info("IndexedDB for cache ready.")
		return &indexedDBFS{
			db: db,
		}, nil
	}
}

func (l *indexedDBFS) ReadFile(p string) ([]byte, error) {
	objStore := l.db.Call("transaction", "files", "readonly").Call("objectStore", "files")
	req := objStore.Call("get", p)
	okC := make(chan []byte)
	errC := make(chan error)
	req.Set("onsuccess", js.FuncOf(func(this js.Value, args []js.Value) any {
		slog.Info("success", "path", p)
		data, err := fromUint8Array(req.Get("result").Get("data").Get("buffer"))
		if err != nil {
			errC <- err
		} else {
			okC <- data
		}
		return js.Null()
	}))
	req.Set("onerror", js.FuncOf(func(this js.Value, args []js.Value) any {
		slog.Info("error", "path", p)
		errC <- fmt.Errorf("failed to store")
		return js.Null()
	}))
	select {
	case err := <-errC:
		return nil, err
	case data := <-okC:
		return data, nil
	}
}

func (l *indexedDBFS) WriteFile(p string, data []byte) error {
	objStore := l.db.Call("transaction", "files", "readwrite").Call("objectStore", "files")
	req := objStore.Call("put", map[string]any{
		"path": p,
		"data": toUint8Array(data),
	})

	okC := make(chan struct{})
	errC := make(chan error)
	req.Set("onsuccess", js.FuncOf(func(this js.Value, args []js.Value) any {
		slog.Info("success", "path", p)
		okC <- struct{}{}
		return js.Null()
	}))
	req.Set("onerror", js.FuncOf(func(this js.Value, args []js.Value) any {
		slog.Info("error", "path", p)
		errC <- fmt.Errorf("failed to store")
		return js.Null()
	}))
	select {
	case err := <-errC:
		return err
	case _ = <-okC:
		return nil
	}
}

func (l *indexedDBFS) Remove(p string) error {
	return fmt.Errorf("Remove unimplemented")
}

func (l *indexedDBFS) Exists(p string) (bool, error) {
	objStore := l.db.Call("transaction", "files", "readonly").Call("objectStore", "files")
	req := objStore.Call("get", p)
	okC := make(chan []byte)
	errC := make(chan error)
	req.Set("onsuccess", js.FuncOf(func(this js.Value, args []js.Value) any {
		slog.Info("success", "path", p)
		data, err := fromUint8Array(req.Get("result").Get("data").Get("buffer"))
		if err != nil {
			errC <- err
		} else {
			okC <- data
		}
		return js.Null()
	}))
	req.Set("onerror", js.FuncOf(func(this js.Value, args []js.Value) any {
		slog.Info("error", "path", p)
		errC <- fmt.Errorf("failed to store")
		return js.Null()
	}))
	select {
	case <-errC:
		return false, nil
	case <-okC:
		return true, nil
	}
}
