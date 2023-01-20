#include <stdlib.h>
#include <string.h>
#include <stdio.h>
#include <unistd.h>
#include <sys/socket.h>
#include <sys/un.h>

#include "../../include/tdbms-client.h"

int tdbms_connect(const char *domain, const char *addr) {
    int socket_fd;
    struct sockaddr_un sockaddr;
    int domain_code;
    if (strcmp(domain, "UNIX") == 0) {
        domain_code = AF_UNIX;
    } else {
        fprintf(stderr, "ERROR: unexpected socket domain '%s'\n", domain);
        return -1;
    }
    if (strlen(addr) > sizeof(sockaddr.sun_path) - 1) {
        fprintf(stderr, "ERROR: too long socket address\n");
        return -1;
    }
    socket_fd = socket(domain_code, SOCK_STREAM, 0);
    if (socket_fd < 0) {
        fprintf(stderr, "ERROR: failed to initialize socket\n");
        return -1;
    }
    bzero(&sockaddr, sizeof(sockaddr));
    sockaddr.sun_family = domain_code;
    strcpy(sockaddr.sun_path, addr);
    if (connect(socket_fd, (const struct sockaddr *) &sockaddr, sizeof(sockaddr)) < 0) {
        fprintf(stderr, "ERROR: failed to connect the socket\n");
        return -1;
    }
    return socket_fd;
}

int tdbms_close(int socket_fd) {
    return close(socket_fd);
}

int tdb_query(int socket_fd, const char *db_name, char request_code, const char *request_body, char **response) {
    if (socket_fd < 0 || db_name == NULL || request_body == NULL) {
        return 1;
    }
    size_t req_size = 1 + strlen(db_name) + 1 + strlen(request_body) + 1, resp_size;
    ssize_t nread, nwrite;
    char *request = malloc(req_size);
    char *buffer = request;
    *buffer = request_code;
    buffer++;
    strcpy(buffer, db_name);
    buffer += strlen(db_name) + 1;
    strcpy(buffer, request_body);
    for (buffer = request; (nwrite = write(socket_fd, buffer, req_size)) > 0;) {
        buffer += nwrite;
        req_size -= nwrite;
        if (req_size == 0) {
            break;
        }
    }
    free(request);
    if (nwrite <= 0) {
        fprintf(stderr, "ERROR: failed to send request to server\n");
        return -1;
    }
    *response = malloc(BUFSIZ);
    resp_size = BUFSIZ;
    buffer = malloc(BUFSIZ);
    for (off_t offset = 0; (nread = read(socket_fd, buffer, BUFSIZ)) > 0;) {
        if (offset + nread > resp_size) {
            resp_size += BUFSIZ;
            *response = realloc(response, resp_size);
        }
        memcpy(*response + offset, buffer, nread);
        offset += nread;
        if ((*response)[offset - 1] == 0) {
            break;
        }
    }
    free(buffer);
    if (nread < 0) {
        fprintf(stderr, "ERROR: failed to get server response\n");
        return -1;
    }
    return 0;
}
