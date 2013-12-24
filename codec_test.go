// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package bencode

import (
	"bytes"
	"io/ioutil"
	"testing"
	"reflect"
)

type file struct {
	Length uint64   `ben:"length"`
	Path   []string `ben:"path"`
	Md5sum string   `ben:"md5sum"`
}

type info struct {
	PieceLength uint64  `ben:"piece length"`
	Name        string  `ben:"name"`
	Length      uint64  `ben:"length"` // Single
	Md5sum      string  `ben:"md5sum"` // Single
	Files       []*file `ben:"files"`  // Multiple
}

type InfoHash struct {
	Hash [20]byte
}

type MetaInfo struct {
	Info         *info      `ben:"info"`
	Announce     string     `ben:"announce"`
	AnnounceList [][]string `ben:"announce-list"`
	CreationDate int64      `ben:"creation date"`
	Comment      string     `ben:"comment"`
}

func TestCodec(t *testing.T) {
	f, err := ioutil.ReadFile("./testdata/single_file.torrent")
	if err != nil {
		t.Fatal(err)
	}

	rm, err := Decode(bytes.NewReader(f))
	if err != nil {
		t.Fatal(err)
	}

	buf := &bytes.Buffer{}
	Encode(rm, buf)
	bebuf := buf.String()
	if (string(f) != bebuf) {
		t.Fatalf("want: %s got: %s", string(f), bebuf)
	}
}

func TestMarshalUnmarshal(t *testing.T) {
	f, err := ioutil.ReadFile("./testdata/single_file.torrent")
	if err != nil {
		t.Fatal(err)
	}

	mi := &MetaInfo{}
	err = Unmarshal(bytes.NewReader(f), mi)
	if err != nil {
		t.Fatal(err)
	}

	buf := &bytes.Buffer{}
	err = Marshal(mi, buf)
	if err != nil {
		t.Fatal(err)
	}

	got := &MetaInfo{}
	err = Unmarshal(buf, got)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(*mi, *got) {
		t.Fatalf("got: %v, want: %v", got, mi)
	}
}
