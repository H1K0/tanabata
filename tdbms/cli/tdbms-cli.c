#include <stdio.h>
#include <stdlib.h>

#include "../../include/tdbms-client.h"

int main(int argc, char **argv) {
    char *db_name, request_code, *request_body;
    if (argc < 4) {
        request_body = "";
    } else {
        request_body = argv[3];
    }
    if (argc < 3) {
        request_code = 0;
    } else {
        char *endptr;
        request_code = (char) strtol(argv[2], &endptr, 0);
        if (*endptr != 0) {
            fprintf(stderr, "FATAL: invalid request code '%s'\n", argv[2]);
            return 1;
        }
    }
    if (argc < 2) {
        db_name = "";
    } else {
        db_name = argv[1];
    }
    int socket_fd = tdbms_connect("UNIX", "/tmp/tdbms.sock");
    if (socket_fd < 0) {
        fprintf(stderr, "FATAL: failed to connect to TDBMS server\n");
        return 1;
    }
    char *response = tdb_query(socket_fd, db_name, request_code, request_body);
    if (response == NULL) {
        fprintf(stderr, "FATAL: failed to execute request\n");
        return 1;
    }
    printf("%s\n", response);
    tdbms_close(socket_fd);
    return 0;
}
