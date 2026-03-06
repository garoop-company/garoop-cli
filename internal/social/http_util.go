package social

import "net/textproto"

func textProtoHeader(values map[string]string) textproto.MIMEHeader {
	h := make(textproto.MIMEHeader)
	for k, v := range values {
		h.Set(k, v)
	}
	return h
}
