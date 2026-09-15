package router

import (
	"mime"
	"mime/multipart"
	"net/http"
	"strings"
)

// MaxMultipartMemory is the number of bytes that ParseMultipartForm keeps in
// memory. A part that is larger goes to a temporary file, which net/http
// removes when the request ends.
//
// The value is the default of net/http. It bounds the memory of one request
// and not the size of an upload: a rule of the field bounds that, and a
// middleware bounds the whole body. See MaxBytes.
const MaxMultipartMemory = 32 << 20

// HasMultipartBody reports a request that carries a multipart form. The
// generated Bind reads it before it parses the form, because
// ParseMultipartForm refuses a body of another media type.
func HasMultipartBody(r *http.Request) bool {
	if r.Body == nil {
		return false
	}
	kind, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	return err == nil && kind == "multipart/form-data"
}

// FileValue returns the first file of one member of a multipart form, and
// reports whether the request carries it.
//
// The generated Bind calls it for a field of type *multipart.FileHeader with a
// form tag. The header carries the name, the size and the declared media type.
// Open reads the content.
func FileValue(r *http.Request, name string) (*multipart.FileHeader, bool) {
	files, ok := FileValues(r, name)
	if !ok {
		return nil, false
	}
	return files[0], true
}

// FileValues returns every file of one member of a multipart form.
func FileValues(r *http.Request, name string) ([]*multipart.FileHeader, bool) {
	if r.MultipartForm == nil || r.MultipartForm.File == nil {
		return nil, false
	}
	files, ok := r.MultipartForm.File[name]
	if !ok || len(files) == 0 {
		return nil, false
	}
	return files, true
}

// FileType returns the media type that the client declared for one file,
// without its parameters. It returns the empty string when the part carries no
// type.
//
// The value is the statement of the client, and a client can state a type that
// the content does not hold. A handler that must be sure reads the first bytes
// and calls http.DetectContentType. FileAccepted therefore states what a form
// refuses and not what a file is.
func FileType(file *multipart.FileHeader) string {
	if file == nil {
		return ""
	}
	kind, _, err := mime.ParseMediaType(file.Header.Get("Content-Type"))
	if err != nil {
		return ""
	}
	return strings.ToLower(kind)
}

// FileAccepted reports a file whose declared media type stands in the list. A
// member of the list that ends with /* accepts every subtype, such as image/*.
//
// The generated Validate calls it for the rule accept.
func FileAccepted(file *multipart.FileHeader, types ...string) bool {
	kind := FileType(file)
	if kind == "" {
		return false
	}
	for _, want := range types {
		want = strings.ToLower(strings.TrimSpace(want))
		if want == kind || want == "*/*" {
			return true
		}
		if prefix, found := strings.CutSuffix(want, "/*"); found &&
			strings.HasPrefix(kind, prefix+"/") {
			return true
		}
	}
	return false
}

// MaxBytes returns a middleware that refuses a request body that is larger
// than n bytes. It bounds the whole body, so an upload cannot fill the disk of
// the server before a rule of a field reads its size.
//
//	r.Use(router.MaxBytes(10 << 20))
//
// The reader answers 413 through the bind fault of the handler that reads it.
func MaxBytes(n int64) Middleware {
	return Middleware{Name: "max-bytes", Wrap: func(next Handler) Handler {
		return func(c *Ctx) (Response, error) {
			r := c.Request()
			r.Body = http.MaxBytesReader(c.Writer(), r.Body, n)
			c.setRequest(r)
			return next(c)
		}
	}}
}
