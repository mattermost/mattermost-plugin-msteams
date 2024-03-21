package loadtest

import (
	"bytes"
	"io"
	"strings"
)

type lenReadSeeker interface {
	io.ReadSeeker
	Len() int
}

type dummyReadCloser struct {
	orig any           // string or []byte
	body lenReadSeeker // instanciated on demand from orig
}

// setup ensures d.body is correctly initialized.
func (d *dummyReadCloser) setup() {
	if d.body == nil {
		switch body := d.orig.(type) {
		case string:
			d.body = strings.NewReader(body)
		case []byte:
			d.body = bytes.NewReader(body)
		case io.ReadCloser:
			var buf bytes.Buffer
			io.Copy(&buf, body) //nolint: errcheck
			body.Close()
			d.body = bytes.NewReader(buf.Bytes())
		}
	}
}

func (d *dummyReadCloser) Read(p []byte) (n int, err error) {
	d.setup()
	return d.body.Read(p)
}

func (d *dummyReadCloser) Close() error {
	d.setup()
	d.body.Seek(0, io.SeekEnd) // nolint: errcheck
	return nil
}

func (d *dummyReadCloser) Len() int {
	d.setup()
	return d.body.Len()
}
