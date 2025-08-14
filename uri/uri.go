package uri

import (
	"strings"

	"github.com/ipfs/go-cid"
)

type List []URI

// implement the flag.Value interface for List
func (l *List) Set(value string) error {
	if value == "" {
		return nil
	}
	*l = append(*l, New(value))
	return nil
}

// String() returns the URIs as a comma-separated string.
func (l List) String() string {
	if len(l) == 0 {
		return ""
	}
	result := make([]string, len(l))
	for i, uri := range l {
		result[i] = uri.String()
	}
	return strings.Join(result, ",")
}

func New(uri string) URI {
	return URI(uri)
}

type URI string

// String() returns the URI as a string.
func (u URI) String() string {
	return string(u)
}

// IsZero returns true if the URI is empty.
func (u URI) IsZero() bool {
	return u == ""
}

// IsValid returns true if the URI is not empty and is a valid URI.
func (u URI) IsValid() bool {
	if u.IsZero() {
		return false
	}
	return u.IsFile() || u.IsWeb() || u.IsCID() || u.IsIPFS() || u.IsFilecoin()
}

// IsFile returns true if the URI is a local file or directory.
func (u URI) IsFile() bool {
	return (len(u) > 7 && u[:7] == "file://") || (len(u) > 1 && u[0] == '/')
}

// IsWeb returns true if the URI is a remote web URI (HTTP or HTTPS).
func (u URI) IsWeb() bool {
	// http:// or https://
	return len(u) > 7 && u[:7] == "http://" || len(u) > 8 && u[:8] == "https://"
}

// IsCID returns true if the URI is a CID.
func (u URI) IsCID() bool {
	if u.IsZero() {
		return false
	}
	parsed, err := cid.Parse(string(u))
	return err == nil && parsed.Defined()
}

// IsIPFS returns true if the URI is an IPFS URI.
func (u URI) IsIPFS() bool {
	return len(u) > 6 && u[:6] == "ipfs://"
}

// IsFilecoin returns true if the URI is a Filecoin URI.
func (u URI) IsFilecoin() bool {
	return len(u) > 10 && u[:10] == "filecoin://"
}
