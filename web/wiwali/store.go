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
	dbC := make(chan js.Value, 1)
	errC := make(chan error, 1)

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
	slog.Info("ReadFile", "path", p)
	objStore := l.db.Call("transaction", "files", "readonly").Call("objectStore", "files")
	okC := make(chan []byte, 1)
	errC := make(chan error, 1)

	req := objStore.Call("get", p)
	req.Set("onsuccess", js.FuncOf(func(this js.Value, args []js.Value) any {
		slog.Info("ReadFile success", "path", p)
		result := req.Get("result")
		data_ := result.Get("data")
		buffer := data_.Get("buffer")
		array := js.Global().Get("Uint8Array").New(buffer)
		data, err := fromUint8Array(array)
		slog.Info("ReadFile decoded", "path", p)
		if err != nil {
			errC <- err
		} else {
			okC <- data
		}
		return js.Null()
	}))
	req.Set("onerror", js.FuncOf(func(this js.Value, args []js.Value) any {
		slog.Info("ReadFile error", "path", p)
		errC <- fmt.Errorf("failed to store")
		return js.Null()
	}))

	slog.Info("ReadFile wait on select...", "path", p)
	select {
	case err := <-errC:
		slog.Info("ReadFile unblocked on error", "path", p)
		return nil, err
	case data := <-okC:
		slog.Info("ReadFile unblocked on success", "path", p)
		return data, nil
	}
}

func (l *indexedDBFS) WriteFile(p string, data []byte) error {
	slog.Info("WriteFile", "path", p)
	objStore := l.db.Call("transaction", "files", "readwrite").Call("objectStore", "files")
	req := objStore.Call("put", map[string]any{
		"path": p,
		"data": toUint8Array(data),
	})

	okC := make(chan struct{}, 1)
	errC := make(chan error, 1)
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
	slog.Info("Exists", "path", p)
	objStore := l.db.Call("transaction", "files", "readonly").Call("objectStore", "files")
	req := objStore.Call("get", p)
	okC := make(chan []byte, 1)
	errC := make(chan error, 1)
	req.Set("onsuccess", js.FuncOf(func(this js.Value, args []js.Value) any {
		slog.Info("Exists success", "path", p)
		result := req.Get("result")
		if result.IsUndefined() {
			errC <- fmt.Errorf("no file")
			return js.Null()
		}
		data_ := result.Get("data")
		buffer := data_.Get("buffer")
		array := js.Global().Get("Uint8Array").New(buffer)
		data, err := fromUint8Array(array)
		if err != nil {
			errC <- err
		} else {
			okC <- data
		}
		return js.Null()
	}))
	req.Set("onerror", js.FuncOf(func(this js.Value, args []js.Value) any {
		slog.Info("Exists error", "path", p)
		slog.Info("error", "path", p)
		errC <- fmt.Errorf("failed to store")
		return js.Null()
	}))
	select {
	case <-errC:
		slog.Info("Exists unblocked on error", "path", p)
		return false, nil
	case <-okC:
		slog.Info("Exists unblocked on success", "path", p)
		return true, nil
	}
}
