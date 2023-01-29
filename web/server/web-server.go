package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"time"
)

type JSON struct {
	Status bool                     `json:"status"`
	Token  string                   `json:"token,omitempty"`
	TRC    uint8                    `json:"trc,omitempty"`
	TRDB   string                   `json:"trdb,omitempty"`
	TRB    string                   `json:"trb,omitempty"`
	Data   []map[string]interface{} `json:"data,omitempty"`
}

var tdbms TDBMSConnection

const TOKEN_VALIDTIME = 604800

var SID int64 = 0
var TOKEN = ""

func TokenGenerate(seed []byte) {
	SID = time.Now().Unix()
	value := SID
	for i, char := range seed {
		value += int64(char) << (i * 8)
	}
	TOKEN = fmt.Sprintf("%x", sha256.Sum256([]byte(strconv.FormatInt(value, 16))))
}

func Auth(handler http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authorized := false
		defer func() {
			if authorized {
				handler.ServeHTTP(w, r)
			} else {
				http.Redirect(w, r, "/auth", http.StatusSeeOther)
			}
		}()
		token, err := r.Cookie("token")
		if err == nil && time.Now().Unix()-SID < TOKEN_VALIDTIME && token.Value == TOKEN {
			authorized = true
			return
		}
	})
}

func HandlerAuth(w http.ResponseWriter, r *http.Request) {
	var buffer = make([]byte, sha256.Size)
	var response = JSON{Status: false}
	var passhash = make([]byte, sha256.Size)
	var hash [sha256.Size]byte
	var passlen = sha256.Size
	var err error
	passfile, err := os.Open("/etc/tanabata/.htpasswd")
	if err != nil {
		log.Fatalf("Failed to open password file: %s\n", err)
	}
	read, err := passfile.Read(passhash)
	if err != nil {
		log.Fatalf("Failed to read password file: %s\n", err)
	}
	if read != sha256.Size {
		log.Fatalln("Invalid password file")
	}
	err = passfile.Close()
	if err != nil {
		log.Fatalf("Failed to close password file: %s\n", err)
	}
	_, err = r.Body.Read(buffer)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	for i := 0; i < sha256.Size; i++ {
		if buffer[i] == 0 {
			passlen = i
			break
		}
	}
	hash = sha256.Sum256(buffer[:passlen])
	if bytes.Equal(hash[:], passhash) {
		TokenGenerate(buffer)
		response.Status = true
		response.Token = TOKEN
	}
	http.SetCookie(w, &http.Cookie{
		Name:    "token",
		Value:   TOKEN,
		Expires: time.Now().Add(TOKEN_VALIDTIME * time.Second),
	})
	w.Header().Set("Content-Type", "application/json")
	jsonData, err := json.Marshal(response)
	if err != nil {
		log.Fatalln(err)
	}
	_, err = w.Write(jsonData)
	if err != nil {
		log.Fatalln(err)
	}
}

func HandlerTDBMS(w http.ResponseWriter, r *http.Request) {
	var request JSON
	var response []byte
	var err error
	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	json_decoder := json.NewDecoder(r.Body)
	json_decoder.DisallowUnknownFields()
	err = json_decoder.Decode(&request)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	response = tdbms.Query(request.TRDB, request.TRC, request.TRB)
	if response == nil {
		http.Error(w, "Failed to execute request", http.StatusInternalServerError)
		return
	}
	if request.TRDB == "$TFM" && (request.TRC == 0b10000 || request.TRC == 0b101000) {
		var json_response JSON
		err = json.Unmarshal(response, &json_response)
		if err != nil {
			http.Error(w, "TDBMS error", http.StatusInternalServerError)
			return
		}
		for index, element := range json_response.Data {
			path := strings.ReplaceAll(element["path"].(string), "/srv/data/tfm/", "")
			splitpath := strings.Split(path, "/")
			for pindex, pelement := range splitpath {
				splitpath[pindex] = url.PathEscape(pelement)
			}
			path = strings.Join(splitpath, "/")
			json_response.Data[index]["path"] = path
		}
		response, err = json.Marshal(json_response)
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, err = w.Write(response)
	if err != nil {
		log.Println(err)
	}
}

func main() {
	var err error
	log.Println("Connecting to TDBMS server...")
	err = tdbms.Connect("unix", "/tmp/tdbms.sock")
	if err != nil {
		log.Fatalln("Failed to connect to TDBMS server")
	}
	defer func(tdbms TDBMSConnection) {
		err := tdbms.Close()
		if err != nil {
			log.Fatalln(err)
		}
	}(tdbms)
	log.Println("Initializing...")
	server := &http.Server{
		Addr: ":42776",
	}
	public_fs := http.FileServer(http.Dir("/srv/www/tanabata"))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/tfm" {
			r.URL.Path += "/"
		}
		if r.URL.Path[len(r.URL.Path)-1] != '/' && path.Ext(r.URL.Path) == "" {
			r.URL.Path += ".html"
		}
		public_fs.ServeHTTP(w, r)
	})
	http.HandleFunc("/AUTH", HandlerAuth)
	http.HandleFunc("/TDBMS", Auth(HandlerTDBMS))
	tfm_fs := http.StripPrefix("/files", http.FileServer(http.Dir("/srv/data/tfm")))
	http.Handle("/files/", Auth(func(w http.ResponseWriter, r *http.Request) {
		tfm_fs.ServeHTTP(w, r)
	}))
	log.Println("Running...")
	err = server.ListenAndServeTLS("/etc/ssl/certs/web-global.crt", "/etc/ssl/private/web-global.key")
	if errors.Is(err, http.ErrServerClosed) {
		log.Fatalln("Server closed")
	} else if err != nil {
		log.Fatalf("Error starting server: %s\n", err)
	}
}
