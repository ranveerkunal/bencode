// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package bencode

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strconv"
)

type KV struct {
	K string      `json:"k"`
	V *RawMessage `json:"v"`
}

type RawMessage struct {
	POD interface{}   `json:"pod,omitempty"`
	L   []*RawMessage `json:"l,omitempty"`
	D   []*KV         `json:"d,omitempty"`
}

func (rm *RawMessage) String() string {
	str, _ := json.Marshal(rm)
	return string(str)
}

func (rm *RawMessage) Marshal(w io.Writer) {
	Encode(rm, w)
}

func (rm *RawMessage) Unmarshal(v interface{}) error {
	switch t := reflect.TypeOf(v); t.Kind() {
	case reflect.Ptr:
		kind := t.Elem().Kind()
		if kind == reflect.Struct {
			err := unmarshalDict(rm.D, reflect.ValueOf(v))
			if err != nil {
				return err
			}
		} else {
			err := unmarshalPOD(rm.POD, reflect.ValueOf(v))
			if err != nil {
				return err
			}
		}
	case reflect.Slice:
		err := unmarshalList(rm.L, reflect.ValueOf(v))
		if err != nil {
			return err
		}
	}
	return nil
}

func isDelim(r *bufio.Reader, delim byte) (bool, error) {
	c, err := r.ReadByte()
	if err != nil {
		return false, err
	}
	if c == delim {
		return true, nil
	}

	err = r.UnreadByte()
	if err != nil {
		return false, err
	}
	return false, nil
}

func intBuf(r *bufio.Reader, delim byte) (string, error) {
	buf := []byte{}
	for {
		if ok, err := isDelim(r, delim); err != nil {
			return "", err
		} else if ok {
			break
		}

		c, err := r.ReadByte()
		if err != nil {
			return "", err
		}

		if !(c == '-' || (c >= '0' && c <= '9')) {
			return "", fmt.Errorf("not a digit")
		}
		buf = append(buf, c)
	}
	return string(buf), nil
}

func strBuf(r *bufio.Reader) (string, error) {
	intbuf, err := intBuf(r, ':')
	if err != nil {
		return "", err
	}

	l, err := strconv.ParseUint(intbuf, 0, 64)
	if err != nil {
		return "", err
	}

	strbuf := make([]byte, l)
	_, err = io.ReadFull(r, strbuf)
	if err != nil {
		return "", err
	}
	return string(strbuf), nil
}

func decodeRawMessage(r *bufio.Reader, n *RawMessage) error {
	c, err := r.ReadByte()
	if err != nil {
		return err
	}

	switch {
	case c >= '0' && c <= '9':
		err := r.UnreadByte()
		if err != nil {
			return err
		}

		strbuf, err := strBuf(r)
		if err != nil {
			return err
		}
		n.POD = strbuf

	case c == 'i':
		intbuf, err := intBuf(r, 'e')
		if err != nil {
			return err
		}

		if intbuf[0] == '-' {
			i, err := strconv.ParseInt(intbuf, 0, 64)
			if err != nil {
				return err
			}
			n.POD = i
		} else {
			u, err := strconv.ParseUint(intbuf, 0, 64)
			if err != nil {
				return err
			}
			n.POD = u
		}

	case c == 'l':
		for {
			if ok, err := isDelim(r, 'e'); err != nil {
				return err
			} else if ok {
				break
			}

			ln := &RawMessage{}
			err = decodeRawMessage(r, ln)
			if err != nil {
				return err
			}
			n.L = append(n.L, ln)
		}

	case c == 'd':
		for {
			if ok, err := isDelim(r, 'e'); err != nil {
				return err
			} else if ok {
				break
			}

			k := &RawMessage{}
			err = decodeRawMessage(r, k)
			if err != nil {
				return err
			}

			v := &RawMessage{}
			err = decodeRawMessage(r, v)
			if err != nil {
				return err
			}

			kv := &KV{k.POD.(string), v}
			n.D = append(n.D, kv)
		}
	default:
		return fmt.Errorf("unexpected character: '%v'", c)
	}
	return nil
}

type Decoder struct {
	r *bufio.Reader
}

func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{bufio.NewReader(r)}
}

func (d *Decoder) Decode() (*RawMessage, error) {
	root := &RawMessage{}
	err := decodeRawMessage(d.r, root)
	if err != nil {
		return nil, err
	}

	return root, nil
}

func Decode(r io.Reader) (*RawMessage, error) {
	d := NewDecoder(r)
	return d.Decode()
}

func Unmarshal(r io.Reader, v interface{}) error {
	rm, err := Decode(r)
	if err != nil {
		return err
	}
	return rm.Unmarshal(v)
}

func encodePOD(pod interface{}) string {
	switch pod.(type) {
	case int64:
		return fmt.Sprintf("i%de", pod.(int64))
	case uint64:
		return fmt.Sprintf("i%de", pod.(uint64))
	case string:
		s := pod.(string)
		return fmt.Sprintf("%d:%s", len(s), s)
	}
	return ""
}

func encodeRawMessage(m *RawMessage, w *bufio.Writer) {
	switch {
	case m.POD != nil:
		w.WriteString(encodePOD(m.POD))

	case len(m.L) > 0:
		w.WriteByte('l')
		for _, i := range m.L {
			encodeRawMessage(i, w)
		}
		w.WriteByte('e')

	case len(m.D) > 0:
		w.WriteByte('d')
		for _, kv := range m.D {
			w.WriteString(encodePOD(kv.K))
			encodeRawMessage(kv.V, w)
		}
		w.WriteByte('e')
	}
	return
}

type Encoder struct {
	w *bufio.Writer
}

func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{bufio.NewWriter(w)}
}

func (e *Encoder) Encode(m *RawMessage) {
	encodeRawMessage(m, e.w)
	e.w.Flush()
}

func Encode(m *RawMessage, w io.Writer) {
	e := NewEncoder(w)
	e.Encode(m)
}

func Marshal(val interface{}, w io.Writer) (err error) {
	var rm *RawMessage
	switch t := reflect.TypeOf(val); t.Kind() {
	case reflect.Ptr:
		kind := t.Elem().Kind()
		if kind == reflect.Struct {
			rm, err = marshalDict(reflect.ValueOf(val))
			if err != nil {
				return err
			}
		} else {
			rm = marshalPOD(reflect.ValueOf(val))
		}
	case reflect.Slice:
		rm, err = marshalList(reflect.ValueOf(val))
		if err != nil {
			return err
		}
	}

	if rm != nil {
		rm.Marshal(w)
	}
	return nil
}
