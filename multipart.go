package messages

import (
	"io"
	"mime/multipart"
	"net/textproto"
)

// MultipartReader is an iterator over parts in a MIME multipart body.
type MultipartReader interface {
	// NextPart returns the next part in the multipart or an error. When there are
	// no more parts, the error io.EOF is returned.
	NextPart() (*Entity, error)
}

type multipartReader struct {
	r *multipart.Reader
}

// NextPart implements MultipartReader.
func (r *multipartReader) NextPart() (*Entity, error) {
	p, err := r.r.NextPart()
	if err != nil {
		return nil, err
	}
	return NewEntity(p.Header, p), nil
}

type multipartBody struct {
	header textproto.MIMEHeader
	parts  []*Entity

	r *io.PipeReader
	w *Writer

	i int
}

// Read implements io.Reader.
func (m *multipartBody) Read(p []byte) (n int, err error) {
	if m.r == nil {
		r, w := io.Pipe()
		m.r = r
		_, m.w = newWriter(w, m.header)

		// Prevent calls to NextPart to succeed
		m.i = len(m.parts)

		go func() {
			if err := m.writeTo(m.w); err != nil {
				w.CloseWithError(err)
				return
			}

			if err := m.w.Close(); err != nil {
				w.CloseWithError(err)
				return
			}

			w.Close()
		}()
	}

	return m.r.Read(p)
}

// Close implements io.Closer.
func (m *multipartBody) Close() error {
	if m.r != nil {
		m.r.Close()
	}
	return nil
}

// NextPart implements MultipartReader.
func (m *multipartBody) NextPart() (*Entity, error) {
	if m.i > len(m.parts) {
		return nil, io.EOF
	}

	part := m.parts[m.i]
	m.i++
	return part, nil
}

func (m *multipartBody) writeTo(w *Writer) error {
	for _, p := range m.parts {
		pw, err := m.w.CreatePart(p.Header)
		if err != nil {
			return err
		}

		if err := p.WriteTo(pw); err != nil {
			return err
		}
		if err := pw.Close(); err != nil {
			return err
		}
	}
	return nil
}
