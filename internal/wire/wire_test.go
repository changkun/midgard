// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package wire

import (
	"bytes"
	"strings"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	png := []byte("\x89PNG\r\n\x1a\n\x00\nbinary, with newlines\n")
	f := Frame{
		Envelope: Envelope{
			Type: Event, Seq: 41, Kind: KindCopy, Time: 1700000000000, Origin: "laptop",
			Formats: []Format{{"text", 5}, {"image/png", len(png)}},
		},
		Payload: append([]byte("hello"), png...),
	}
	b, err := f.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	// the envelope is one line of JSON, and the bytes follow as they are
	head, rest, _ := bytes.Cut(b, []byte{'\n'})
	if !strings.HasPrefix(string(head), `{"type":"event"`) || !bytes.Equal(rest, f.Payload) {
		t.Fatalf("frame %q", b)
	}

	g, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}
	if g.Seq != 41 || g.Origin != "laptop" || !bytes.Equal(g.Payload, f.Payload) {
		t.Fatalf("decoded %+v", g)
	}
	parts := g.Parts()
	if len(parts) != 2 || string(parts[0]) != "hello" || !bytes.Equal(parts[1], png) {
		t.Fatalf("parts %q", parts)
	}
	if p, ok := g.Part("image/png"); !ok || !bytes.Equal(p, png) {
		t.Fatalf("Part(image/png) = %q, %v", p, ok)
	}
	if _, ok := g.Part("text/html"); ok {
		t.Fatal("found a format the copy does not have")
	}
}

func TestUnmarshalRefuses(t *testing.T) {
	for name, b := range map[string]string{
		"no envelope":   `{"type":"ack"}`,
		"no type":       "{}\n",
		"not json":      "hello\n",
		"short payload": `{"type":"copy","formats":[{"mime":"text","size":10}]}` + "\nhello",
		"long payload":  `{"type":"copy","formats":[{"mime":"text","size":1}]}` + "\nhello",
		"negative size": `{"type":"copy","formats":[{"mime":"text","size":-5}]}` + "\n",
	} {
		if _, err := Unmarshal([]byte(b)); err == nil {
			t.Errorf("%s: decoded %q", name, b)
		}
	}
	// a bare answer names formats whose bytes it leaves out, but for a preview
	if f, err := Unmarshal([]byte(`{"type":"have","bare":true,"formats":[{"mime":"text","size":10}]}` + "\n")); err != nil || f.Size() != 10 {
		t.Errorf("a bare have: %+v, %v", f, err)
	}
	f := Frame{Envelope: Envelope{Type: Have, Bare: true, Formats: []Format{{"text", 100}}}, Payload: []byte("the start")}
	b, err := f.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	if g, err := Unmarshal(b); err != nil || string(g.Payload) != "the start" || g.Size() != 100 {
		t.Errorf("a preview: %+v, %v", g, err)
	}
}

func TestMarshalChecksTheFormats(t *testing.T) {
	f := NewCopy("text", []byte("hello"))
	f.Payload = []byte("hi")
	if _, err := f.Marshal(); err == nil {
		t.Fatal("marshaled a payload the formats do not describe")
	}
}
