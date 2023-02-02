package main

import (
	"bytes"
	"net"
)

// TDBMS connection struct
type TDBMSConnection struct {
	domain string
	addr   string
	conn   net.Conn
}

// Connect to TDBMS server
func (tdbms *TDBMSConnection) Connect(domain, addr string) error {
	c, err := net.Dial(domain, addr)
	if err == nil {
		tdbms.domain = domain
		tdbms.addr = addr
		tdbms.conn = c
	}
	return err
}

// Reconnect to TDBMS server
func (tdbms *TDBMSConnection) Reconnect() error {
	tdbms.Close()
	c, err := net.Dial(tdbms.domain, tdbms.addr)
	if err == nil {
		tdbms.conn = c
	}
	return err
}

// Close connection to TDBMS server
func (tdbms *TDBMSConnection) Close() error {
	return tdbms.conn.Close()
}

// Execute a TDB request
func (tdbms *TDBMSConnection) Query(db_name string, trc uint8, request_body string) []byte {
	var err error
	buffer := []byte{trc}
	buffer = append(buffer, db_name...)
	buffer = append(buffer, 0)
	buffer = append(buffer, request_body...)
	buffer = append(buffer, 0, 4)
	_, err = tdbms.conn.Write(buffer)
	if err != nil {
		return nil
	}
	var response []byte
	var resp_size int = -1
	buffer = make([]byte, 8192)
	for resp_size <= 0 {
		_, err = tdbms.conn.Read(buffer)
		if err != nil {
			return nil
		}
		response = append(response, buffer...)
		resp_size = bytes.IndexByte(response, 4)
	}
	return response[:resp_size-1]
}
