// Package http adalah delivery HTTP: router, handler, middleware, DTO.
package http

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/mikbil/mikbil/internal/usecase"
)

// envelopeOK adalah amplop sukses {ok:true, data}.
type envelopeOK struct {
	OK   bool `json:"ok"`
	Data any  `json:"data"`
}

// envelopeErr adalah amplop gagal {ok:false, error, code?}.
type envelopeErr struct {
	OK    bool   `json:"ok"`
	Error string `json:"error"`
	Code  string `json:"code,omitempty"`
}

// WriteOK menulis respons sukses berenvelope.
func WriteOK(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(envelopeOK{OK: true, Data: data})
}

// WriteErr menulis respons gagal berenvelope.
func WriteErr(w http.ResponseWriter, status int, msg string) {
	WriteErrCode(w, status, msg, "")
}

// WriteErrCode menulis respons gagal berenvelope dengan kode mesin.
func WriteErrCode(w http.ResponseWriter, status int, msg, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(envelopeErr{OK: false, Error: msg, Code: code})
}

// maxBodyBytes membatasi ukuran body request (1 MB).
const maxBodyBytes = 1 << 20

// decodeJSON membaca body JSON dengan batas ukuran.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(dst); err != nil {
		WriteErr(w, http.StatusBadRequest, "isi request tidak valid")
		return false
	}
	return true
}

// writeUsecaseErr memetakan error usecase ke status HTTP berenvelope.
func writeUsecaseErr(w http.ResponseWriter, err error) {
	var input usecase.InputError
	var conflict usecase.ConflictError
	var unauth usecase.UnauthorizedError
	var notimpl usecase.NotImplementedError
	switch {
	case errors.As(err, &input):
		WriteErr(w, http.StatusBadRequest, input.Msg)
	case errors.As(err, &conflict):
		WriteErr(w, http.StatusConflict, conflict.Msg)
	case errors.As(err, &unauth):
		WriteErr(w, http.StatusUnauthorized, unauth.Msg)
	case errors.As(err, &notimpl):
		WriteErrCode(w, http.StatusNotImplemented, notimpl.Msg, "not_implemented")
	default:
		WriteErr(w, http.StatusInternalServerError, "kesalahan server")
	}
}
