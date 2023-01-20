// Tanabata DBMS client lib
// By Masahiko AMANO aka H1K0

#pragma once
#ifndef TANABATA_DBMS_CLIENT_H
#define TANABATA_DBMS_CLIENT_H

#ifdef __cplusplus
extern "C" {
#endif

#include "tdbms.h"

// Connect to TDBMS server
int tdbms_connect(const char *domain, const char *addr);

// Close connection to TDBMS server
int tdbms_close(int socket_fd);

// Execute a TDB request
int tdb_query(int socket_fd, const char *db_name, char request_code, const char *request_body, char **response);

#ifdef __cplusplus
}
#endif

#endif //TANABATA_DBMS_CLIENT_H
