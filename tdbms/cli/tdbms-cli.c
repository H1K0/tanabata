#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#include "../../include/tdbms-client.h"

int main(int argc, char **argv) {
    if (argc == 1 || strcmp(argv[1], "-h") == 0) {
        printf("Tanabata Database Management client\n\n"
               "Usage\n"
               "  tdb [DB_NAME [REQUEST_CODE [REQUEST_BODY]]]\n\n"
               "Request codes:\n"
               "  0\tDB       stats\n"
               "  3\tDB       init\n"
               "  2\tDB       load\n"
               "  4\tDB       save\n"
               "  6\tDB       edit\n"
               "  1\tDB       remove soft\n"
               "  5\tDB       remove hard\n"
               "  7\tDB       weed\n"
               " 16\tSasa     get\n"
               " 40\tSasa     get by tanzaku\n"
               " 18\tSasa     add\n"
               " 20\tSasa     update\n"
               " 17\tSasa     remove\n"
               " 32\tTanzaku  get\n"
               " 24\tTanzaku  get by sasa\n"
               " 34\tTanzaku  add\n"
               " 36\tTanzaku  update\n"
               " 33\tTanzaku  remove\n"
               "  8\tKazari   get\n"
               " 10\tKazari   add\n"
               " 26\tKazari   add single sasa to multiple tanzaku\n"
               " 42\tKazari   add single tanzaku to multiple sasa\n"
               "  9\tKazari   remove\n"
               " 25\tKazari   remove single sasa to multiple tanzaku\n"
               " 41\tKazari   remove single tanzaku to multiple sasa\n");
        return 0;
    }
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
