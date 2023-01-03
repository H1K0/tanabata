#include <fcgi_stdio.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>
#include <openssl/sha.h>
#include <pthread.h>

#define TOKEN_RENTALTIME 604800
#define TOKEN_SIZE 64

static time_t SID;
static char TOKEN[TOKEN_SIZE];
static int socket_auth;
static int socket_cgi;

int validate(FCGX_Request *request) {
    if (time(NULL) - SID > TOKEN_RENTALTIME) {
        return 1;
    }
    char token[TOKEN_SIZE];
    FCGX_GetStr(token, TOKEN_SIZE, request->in);
    if (memcmp(token, TOKEN, TOKEN_SIZE) == 0) {
        return 0;
    }
    return 1;
}

static void *auth() {
    FCGX_Request request;
    if (FCGX_InitRequest(&request, socket_auth, FCGI_FAIL_ACCEPT_ON_INTR) != 0) {
        exit(1);
    }
    unsigned char password[SHA256_DIGEST_LENGTH];
    FILE *passfile = fopen("/etc/tfm/password", "rb");
    if (passfile == NULL ||
        fread(password, 1, SHA256_DIGEST_LENGTH, passfile) < SHA256_DIGEST_LENGTH) {
        exit(1);
    }
    fclose(passfile);
    int rc;
    char buffer[33];
    for (;;) {
        static pthread_mutex_t accept_mutex = PTHREAD_MUTEX_INITIALIZER;
        pthread_mutex_lock(&accept_mutex);
        rc = FCGX_Accept_r(&request);
        pthread_mutex_unlock(&accept_mutex);
        if (rc < 0) {
            break;
        }
        memset(buffer, 0, 33);
        FCGX_GetStr(buffer, 32, request.in);
        unsigned char hash[SHA256_DIGEST_LENGTH];
        SHA256((const unsigned char *) buffer, strlen(buffer), hash);
        if (memcmp(hash, password, SHA256_DIGEST_LENGTH) == 0) {
            time(&SID);
            uint64_t subtoken = SID;
            SHA256((const unsigned char *) &subtoken, 8, hash);
            for (int i = 0; i < SHA256_DIGEST_LENGTH; i++) {
                sprintf(TOKEN + (i * 2), "%02x", hash[i]);
            }
            FCGX_PutS("Content-type: application/json\r\n\r\n"
                      "{\"status\":true,\"token\":\"", request.out);
            FCGX_PutS(TOKEN, request.out);
            FCGX_PutS("\"}\n", request.out);
            FCGX_Finish_r(&request);
            continue;
        }
        FCGX_PutS("Content-type: application/json\r\n\r\n"
                  "{\"status\":false}\n", request.out);
        FCGX_Finish_r(&request);
    }
    return NULL;
}

static void *tfmcgi() {
    FCGX_Request request;
    if (FCGX_InitRequest(&request, socket_cgi, FCGI_FAIL_ACCEPT_ON_INTR) != 0) {
        exit(1);
    }
    int rc;
    for (;;) {
        static pthread_mutex_t accept_mutex = PTHREAD_MUTEX_INITIALIZER;
        pthread_mutex_lock(&accept_mutex);
        rc = FCGX_Accept_r(&request);
        pthread_mutex_unlock(&accept_mutex);
        if (rc < 0) {
            break;
        }
        if (validate(&request) != 0) {
            FCGX_PutS("Content-type: application/json\r\n\r\n"
                      "{\"status\":false}\n", request.out);
            FCGX_Finish_r(&request);
            continue;
        }
        FCGX_PutS("Content-type: application/json\r\n\r\n"
                  "{\"status\":true}\n", request.out);
        FCGX_Finish_r(&request);
    }
    return NULL;
}

int main() {
    pthread_t thread_auth, thread_cgi;
    FCGX_Init();
    if ((socket_auth = FCGX_OpenSocket("/tmp/tfm-auth.sock", 0)) == -1 ||
        (socket_cgi = FCGX_OpenSocket("/tmp/tfm-cgi.sock", 0)) == -1) {
        return 1;
    }
    pthread_create(&thread_auth, NULL, auth, NULL);
    pthread_create(&thread_cgi, NULL, tfmcgi, NULL);
    pthread_join(thread_auth, NULL);
    pthread_join(thread_cgi, NULL);
    return 0;
}
