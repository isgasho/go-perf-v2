// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package benchfmt

import "testing"

func checkNameExtractor(t *testing.T, x Extractor, fullName string, want string) {
	t.Helper()
	res := &Result{FullName: []byte(fullName)}
	got := x(res)
	if got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestExtractName(t *testing.T) {
	check := checkNameExtractor

	x, err := NewExtractor(".name")
	if err != nil {
		t.Fatal(err)
	}
	check(t, x, "Test", "Test")
	check(t, x, "Test/a", "Test")
	check(t, x, "Test-4", "Test")
	check(t, x, "Test/a-4", "Test")
}

func TestExtractFull(t *testing.T) {
	check := checkNameExtractor

	t.Run("basic", func(t *testing.T) {
		x, err := NewExtractor(".full")
		if err != nil {
			t.Fatal(err)
		}
		check(t, x, "Test", "Test")
		check(t, x, "Test/a=123", "Test/a=123")
		check(t, x, "Test-2", "Test-2")
	})

	t.Run("excludeA", func(t *testing.T) {
		x := NewExtractorFullName([]string{"/a"})
		check(t, x, "Test", "Test")
		check(t, x, "Test/a=123", "Test/a=*")
		check(t, x, "Test/b=123/a=123", "Test/b=123/a=*")
		check(t, x, "Test/a=123/b=123", "Test/a=*/b=123")
		check(t, x, "Test/a=123/a=123", "Test/a=*/a=*")
		check(t, x, "Test/a=123-2", "Test/a=*-2")
	})

	t.Run("excludeGomaxprocs", func(t *testing.T) {
		x := NewExtractorFullName([]string{"/gomaxprocs"})
		check(t, x, "Test", "Test")
		check(t, x, "Test/a=123", "Test/a=123")
		check(t, x, "Test/a=123-2", "Test/a=123-*")
		check(t, x, "Test/gomaxprocs=123", "Test/gomaxprocs=*")
	})
}

func TestExtractNameKey(t *testing.T) {
	check := checkNameExtractor

	t.Run("basic", func(t *testing.T) {
		x, err := NewExtractor("/a")
		if err != nil {
			t.Fatal(err)
		}
		check(t, x, "Test", "")
		check(t, x, "Test/a=1", "1")
		check(t, x, "Test/aa=1", "")
		check(t, x, "Test/a=1/b=2", "1")
		check(t, x, "Test/b=1/a=2", "2")
		check(t, x, "Test/b=1/a=2-4", "2")
	})

	t.Run("gomaxprocs", func(t *testing.T) {
		x, err := NewExtractor("/gomaxprocs")
		if err != nil {
			t.Fatal(err)
		}
		check(t, x, "Test", "")
		check(t, x, "Test/gomaxprocs=4", "4")
		check(t, x, "Test-4", "4")
		check(t, x, "Test/a-4", "4")
	})
}

func TestExtractFileKey(t *testing.T) {
	x, err := NewExtractor("file-key")
	if err != nil {
		t.Fatal(err)
	}

	res := r([]Config{{"file-key", "123"}, {"other-key", "456"}}, "Name", 1, nil)
	got := x(res)
	want := "123"
	if got != want {
		t.Errorf("got %s, want %s", got, want)
	}

	res = r([]Config{{"other-key", "456"}}, "Name", 1, nil)
	got = x(res)
	want = ""
	if got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestExtractBadKey(t *testing.T) {
	check := func(t *testing.T, got error, want string) {
		t.Helper()
		if got == nil || got.Error() != want {
			t.Errorf("got error %s, want error %s", got, want)
		}
	}
	t.Run("special", func(t *testing.T) {
		_, err := NewExtractor(".bad")
		check(t, err, "unknown special key: .bad")
	})
	t.Run("other", func(t *testing.T) {
		_, err := NewExtractor("Foo")
		check(t, err, "expected .name, .full, /key, or file key: Foo")
		_, err = NewExtractor("foo bar")
		check(t, err, "expected .name, .full, /key, or file key: foo bar")
	})
}
