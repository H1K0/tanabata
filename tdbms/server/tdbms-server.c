#include <stdlib.h>
#include <stdint.h>
#include <stdio.h>
#include <unistd.h>
#include <sys/socket.h>
#include <sys/un.h>
#include <pthread.h>
#include <stdarg.h>
#include <sys/time.h>

#include "../../include/tdbms.h"
#include "../../include/tanabata.h"

// TDBMS configuration file
#define TDBMS_CONFIG_FILE "/etc/tanabata/tdbms.conf"

// TDBMS variables directory
#define TDBMS_VAR_DIR "/var/lib/tanabata/tdbms"

// TDBMS default socket domain
#define DEFAULT_SOCK_DOMAIN "UNIX"

// TDBMS degault socket address
#define DEFAULT_SOCK_ADDR "/tmp/tdbms.sock"

// Maximum number of queued clients
#define DEFAULT_CLIENTS_MAX 0x100

// Maximum count of critical errors in row
#define DEFAULT_CRITICAL_MAX 0b10000

// Default log file path
#define DEFAULT_LOG_FILE "/var/log/tanabata/tdbms.log"

// Log levels
#define LOG_INFO 2
#define LOG_WARNING 3
#define LOG_ERROR 4
#define LOG_CRITICAL 5
#define LOG_FATAL 6

typedef struct tdb {
    char *name;          // TDB name
    char *path;          // TDB location path
    Tanabata *database;  // TDB data
} TDB;

char *sock_domain = NULL;
char *sock_addr = NULL;
uint16_t max_clients = DEFAULT_CLIENTS_MAX;
uint16_t max_criticals = DEFAULT_CRITICAL_MAX;
int server_fd;
TDB *db_list = NULL;
uint16_t db_count = 0;
FILE *logfile = NULL;

pthread_mutex_t mutex_log = PTHREAD_MUTEX_INITIALIZER;
pthread_mutex_t mutex_connection = PTHREAD_MUTEX_INITIALIZER;
pthread_mutex_t mutex_request = PTHREAD_MUTEX_INITIALIZER;
pthread_t *threads = NULL;

// Log formatted message
void logtf(int level, const char *fmt, ...) {
    va_list args;
    va_start(args, fmt);
    struct timeval now;
    gettimeofday(&now, NULL);
    char *slevel;
    switch (level) {
        case LOG_INFO:
            slevel = "INFO";
            break;
        case LOG_WARNING:
            slevel = "WARNING";
            break;
        case LOG_ERROR:
            slevel = "ERROR";
            break;
        case LOG_CRITICAL:
            slevel = "CRITICAL";
            break;
        case LOG_FATAL:
            slevel = "FATAL";
            break;
        default:
            logtf(LOG_ERROR, "Invalid log level %i", level);
            return;
    }
    size_t level_len = strlen(slevel);
    char *fmt_log = malloc(33 + level_len + strlen(fmt));
    strftime(fmt_log, 20, "%FT%T", localtime(&now.tv_sec));
    sprintf(fmt_log, "%s.%06li | %s | %s\n", fmt_log, now.tv_usec, slevel, fmt);
    pthread_mutex_lock(&mutex_log);
    if (logfile != NULL) {
        vfprintf(logfile, fmt_log, args);
    } else {
        vfprintf(stdout, fmt_log, args);
    }
    fflush(logfile);
    pthread_mutex_unlock(&mutex_log);
    free(fmt_log);
}

// Load configuration
void config_load() {
    FILE *config = fopen(TDBMS_CONFIG_FILE, "r");
    if (config == NULL) {
        config = fopen(TDBMS_CONFIG_FILE, "w");
        if (config == NULL) {
            logtf(LOG_FATAL, "failed to create config file");
            exit(1);
        }
        fprintf(config, "sockdomn=%s\nsockaddr=%s\nmax_clients=0x%x\nmax_criticals=0x%x\nlogfile=%s\n",
                DEFAULT_SOCK_DOMAIN, DEFAULT_SOCK_ADDR, DEFAULT_CLIENTS_MAX,
                DEFAULT_CRITICAL_MAX, DEFAULT_LOG_FILE);
        fclose(config);
        return;
    }
    char buffer[BUFSIZ];
    char *value;
    while (fgets(buffer, BUFSIZ, config) != NULL) {
        value = buffer;
        value[strlen(value) - 1] = 0;
        if (strncmp(buffer, "sockdomn", 8) == 0) {
            for (value += 8; *value == ' ' || *value == '='; value++);
            sock_domain = realloc(sock_domain, strlen(value) + 1);
            strcpy(sock_domain, value);
        } else if (strncmp(buffer, "sockaddr", 8) == 0) {
            for (value += 8; *value == ' ' || *value == '='; value++);
            sock_addr = realloc(sock_addr, strlen(value) + 1);
            strcpy(sock_addr, value);
        } else if (strncmp(buffer, "max_clients", 11) == 0) {
            for (value += 11; *value == ' ' || *value == '='; value++);
            char *endptr;
            max_clients = strtoul(value, &endptr, 0);
            if (*endptr != 0) {
                logtf(LOG_FATAL, "invalid config file");
                exit(1);
            }
        } else if (strncmp(buffer, "max_criticals", 13) == 0) {
            for (value += 13; *value == ' ' || *value == '='; value++);
            char *endptr;
            max_criticals = strtoul(value, &endptr, 0);
            if (*endptr != 0) {
                logtf(LOG_FATAL, "invalid config file");
                exit(1);
            }
        } else if (strncmp(buffer, "logfile", 7) == 0) {
            for (value += 7; *value == ' ' || *value == '='; value++);
            if ((logfile = fopen(value, "a")) == NULL) {
                logtf(LOG_ERROR, "failed to load custom log file, use default");
            }
        } else if (*buffer != 0) {
            logtf(LOG_WARNING, "unexpected parameter '%s'", value);
        }
    }
    fclose(config);
}

// Load database list
void dblist_load() {
    FILE *fdblist = fopen(TDBMS_VAR_DIR"/.dblist", "rb");
    if (fdblist == NULL) {
        fdblist = fopen(TDBMS_VAR_DIR"/.dblist", "wb");
        if (fdblist == NULL) {
            logtf(LOG_FATAL, "failed to create database list file");
            exit(1);
        }
        fclose(fdblist);
        return;
    }
    TDB newbie;
    char *name = NULL, *path = NULL;
    size_t MAX_PATH_LEN = 0;
    for (db_count = 0; db_count < UINT16_MAX;) {
        if (getdelim(&name, &MAX_PATH_LEN, ' ', fdblist) == -1 ||
            getdelim(&path, &MAX_PATH_LEN, '\n', fdblist) == -1) {
            break;
        }
        newbie.name = malloc(strlen(name));
        newbie.path = malloc(strlen(path));
        newbie.database = NULL;
        name[strlen(name) - 1] = 0;
        path[strlen(path) - 1] = 0;
        strcpy(newbie.name, name);
        strcpy(newbie.path, path);
        db_count++;
        db_list = realloc(db_list, db_count * sizeof(TDB));
        db_list[db_count - 1] = newbie;
    }
    fclose(fdblist);
}

// Save database list
int dblist_save() {
    FILE *fdblist = fopen(TDBMS_VAR_DIR"/.dblist", "wb");
    if (fdblist == NULL) {
        logtf(LOG_CRITICAL, "failed to save database list file");
        return 1;
    }
    TDB *tdb = db_list;
    for (uint16_t i = 0; i < db_count; i++) {
        fprintf(fdblist, "%s %s\n", tdb->name, tdb->path);
        tdb++;
    }
    fclose(fdblist);
    return 0;
}

// Open TDBMS server socket
int socket_open() {
    int socket_fd;
    struct sockaddr_un sockaddr;
    int domain;
    if (strcmp(sock_domain, "UNIX") == 0) {
        domain = AF_UNIX;
    } else {
        logtf(LOG_FATAL, "unexpected socket domain '%s'", sock_domain);
        exit(1);
    }
    if (strlen(sock_addr) + 1 > sizeof(sockaddr.sun_path)) {
        logtf(LOG_FATAL, "too long socket address");
        exit(1);
    }
    socket_fd = socket(domain, SOCK_STREAM, 0);
    if (socket_fd < 0) {
        logtf(LOG_FATAL, "failed to initialize socket");
        exit(-1);
    }
    bzero(&sockaddr, sizeof(sockaddr));
    sockaddr.sun_family = domain;
    strcpy(sockaddr.sun_path, sock_addr);
    unlink(sock_addr);
    if (bind(socket_fd, (const struct sockaddr *) &sockaddr, sizeof(sockaddr)) < 0) {
        logtf(LOG_FATAL, "failed to bind socket");
        exit(-1);
    }
    if (listen(socket_fd, DEFAULT_CLIENTS_MAX) < 0) {
        logtf(LOG_FATAL, "failed to listen to socket");
        exit(-1);
    }
    return socket_fd;
}

// Execute request
int execute(char *request, char **response) {
    char request_code = *request;
    char *request_db_name = request + 1;
    char *request_body = request_db_name + strlen(request_db_name) + 1;
    *response = malloc(BUFSIZ);
    **response = 0;
    TDB *tdb;
    Tanabata *tanabata;
    for (tdb = db_list + db_count - 1; tdb >= db_list; tdb--) {
        if (strcmp(tdb->name, request_db_name) == 0) {
            if (tdb->database == NULL) {
                tdb->database = malloc(sizeof(Tanabata));
                tanabata_init(tdb->database);
                if (request_code != trc_db_edit && tanabata_open(tdb->database, tdb->path) != 0) {
                    return 1;
                }
            }
            tanabata = tdb->database;
            break;
        }
    }
    if (tdb < db_list) {
        tdb = NULL;
        tanabata = NULL;
    }
    char *buffer;
    if (request_code == trc_db_stats) {
        if (*request_db_name != 0) {
            if (tanabata == NULL) {
                sprintf(*response, "{\"status\":true,\"loaded\":false}");
                return 0;
            }
            sprintf(*response, "{"
                               "\"status\":true,\"loaded\":true,\"unsaved\":%s,"
                               "\"sasahyou_cts\":0x%lx,\"sasahyou_mts\":0x%lx,\"sasahyou_size\":0x%lx,\"sasahyou_holes\":0x%lx,"
                               "\"sappyou_cts\":0x%lx,\"sappyou_mts\":0x%lx,\"sappyou_size\":0x%lx,\"sappyou_holes\":0x%lx,"
                               "\"shoppyou_cts\":0x%lx,\"shoppyou_mts\":0x%lx,\"shoppyou_size\":0x%lx,\"shoppyou_holes\":0x%lx"
                               "}",
                    (tanabata->sasahyou_mod != tanabata->sasahyou.modified_ts ||
                     tanabata->sappyou_mod != tanabata->sappyou.modified_ts ||
                     tanabata->shoppyou_mod != tanabata->shoppyou.modified_ts) ? "true" : "false",
                    tanabata->sasahyou.created_ts, tanabata->sasahyou.modified_ts, tanabata->sasahyou.size,
                    tanabata->sasahyou.hole_cnt,
                    tanabata->sappyou.created_ts, tanabata->sappyou.modified_ts, tanabata->sappyou.size,
                    tanabata->sappyou.hole_cnt,
                    tanabata->shoppyou.created_ts, tanabata->shoppyou.modified_ts, tanabata->shoppyou.size,
                    tanabata->shoppyou.hole_cnt);
            return 0;
        }
        sprintf(*response, "{\"status\":true,\"tdb_list\":[");
        size_t resp_size = BUFSIZ;
        buffer = malloc(BUFSIZ);
        TDB *temp = db_list;
        for (uint16_t i = 0; i < db_count; i++, temp++) {
            if (temp->database == NULL) {
                sprintf(buffer, "{\"tdb_name\":\"%s\",\"loaded\":false},", temp->name);
            } else {
                tanabata = temp->database;
                sprintf(buffer, "{"
                                "\"loaded\":true,\"unsaved\":%s,"
                                "\"sasahyou_cts\":0x%lx,\"sasahyou_mts\":0x%lx,\"sasahyou_size\":0x%lx,\"sasahyou_holes\":0x%lx,"
                                "\"sappyou_cts\":0x%lx,\"sappyou_mts\":0x%lx,\"sappyou_size\":0x%lx,\"sappyou_holes\":0x%lx,"
                                "\"shoppyou_cts\":0x%lx,\"shoppyou_mts\":0x%lx,\"shoppyou_size\":0x%lx,\"shoppyou_holes\":0x%lx"
                                "},",
                        (tanabata->sasahyou_mod != tanabata->sasahyou.modified_ts ||
                         tanabata->sappyou_mod != tanabata->sappyou.modified_ts ||
                         tanabata->shoppyou_mod != tanabata->shoppyou.modified_ts) ? "true" : "false",
                        tanabata->sasahyou.created_ts, tanabata->sasahyou.modified_ts, tanabata->sasahyou.size,
                        tanabata->sasahyou.hole_cnt,
                        tanabata->sappyou.created_ts, tanabata->sappyou.modified_ts, tanabata->sappyou.size,
                        tanabata->sappyou.hole_cnt,
                        tanabata->shoppyou.created_ts, tanabata->shoppyou.modified_ts, tanabata->shoppyou.size,
                        tanabata->shoppyou.hole_cnt);
            }
            if (strlen(*response) + strlen(buffer) >= resp_size) {
                resp_size += BUFSIZ;
                *response = realloc(*response, resp_size);
            }
            strcat(*response, buffer);
        }
        sprintf(buffer, "]}");
        if (strlen(*response) + 3 >= resp_size) {
            *response = realloc(*response, resp_size + 3);
        }
        strcat(*response, buffer);
        free(buffer);
        return 0;
    }
    if (request_code == trc_db_init) {
        if (db_count == UINT16_MAX || tdb != NULL) {
            return 1;
        }
        db_count++;
        db_list = reallocarray(db_list, db_count, sizeof(TDB));
        tdb = db_list + db_count - 1;
        tdb->path = malloc(strlen(request_body) + 1);
        tdb->name = malloc(strlen(request_db_name) + 1);
        tdb->database = malloc(sizeof(Tanabata));
        tanabata_init(tdb->database);
        strcpy(tdb->name, request_db_name);
        strcpy(tdb->path, request_body);
        return 0;
    }
    if (request_code == trc_db_load) {
        if (tdb == NULL) {
            return 1;
        }
        if (tanabata_load(tanabata) == 0) {
            return 0;
        }
        if (tanabata != NULL) {
            return tanabata_open(tanabata, tdb->path);
        }
        Tanabata temp;
        if (tanabata_open(&temp, tdb->path) != 0) {
            return 1;
        }
        tdb->database = malloc(sizeof(Tanabata));
        *tdb->database = temp;
        return 0;
    }
    if (request_code == trc_db_save) {
        if (*request_db_name == 0) {
            return dblist_save();
        }
        if (tanabata == NULL) {
            return 1;
        }
        if (strlen(request_body) > 0) {
            return tanabata_dump(tanabata, request_body);
        }
        return tanabata_dump(tanabata, tdb->path);
    }
    if (request_code == trc_db_edit) {
        if (tdb == NULL) {
            return 1;
        }
        buffer = request_body;
        off_t offset;
        while (*buffer != 0) {
            if (strncmp(buffer, "name", 4) == 0) {
                for (buffer += 4; *buffer == ' ' || *buffer == '='; buffer++);
                for (offset = 0; buffer[offset] != '\n' && buffer[offset] != 0; offset++);
                tdb->name = realloc(tdb->name, offset);
                strncpy(tdb->name, buffer, offset);
                tdb->name[offset] = 0;
                buffer += offset;
            }
            if (strncmp(buffer, "path", 4) == 0) {
                for (buffer += 4; *buffer == ' ' || *buffer == '='; buffer++);
                for (offset = 0; buffer[offset] != '\n' && buffer[offset] != 0; offset++);
                tdb->path = realloc(tdb->path, offset);
                strncpy(tdb->path, buffer, offset);
                tdb->path[offset] = 0;
                buffer += offset;
            } else {
                return 1;
            }
        }
        return 0;
    }
    if (request_code == trc_db_remove_soft) {
        if (tdb == NULL) {
            return 1;
        }
        free(tdb->name);
        free(tdb->path);
        if (tdb->database != NULL) {
            tanabata_free(tdb->database);
            free(tdb->database);
        }
        db_count--;
        for (uint16_t i = tdb - db_list; i < db_count; i++) {
            db_list[i] = db_list[i + 1];
        }
        db_list = reallocarray(db_list, db_count, sizeof(TDB));
        return 0;
    }
    if (request_code == trc_db_remove_hard) {
        if (tdb == NULL) {
            return 1;
        }
        size_t pathlen = strlen(tdb->path);
        char *remove_path = malloc(pathlen + 10);
        strcpy(remove_path, tdb->path);
        free(tdb->name);
        free(tdb->path);
        if (tdb->database != NULL) {
            tanabata_free(tdb->database);
            free(tdb->database);
        }
        db_count--;
        for (uint16_t i = tdb - db_list; i < db_count; i++) {
            db_list[i] = db_list[i + 1];
        }
        db_list = reallocarray(db_list, db_count, sizeof(TDB));
        strcpy(remove_path + pathlen, "/sasahyou");
        unlink(remove_path);
        strcpy(remove_path + pathlen, "/sappyou");
        unlink(remove_path);
        strcpy(remove_path + pathlen, "/shoppyou");
        unlink(remove_path);
        free(remove_path);
        return 0;
    }
    if (request_code == trc_db_weed) {
        if (tanabata == NULL) {
            return 1;
        }
        return tanabata_weed(tanabata);
    }
    if (request_code == trc_sasa_get) {
        if (tanabata == NULL) {
            return 1;
        }
        if (*request_body != 0) {
            char *endptr;
            uint64_t sasa_id = strtoull(request_body, &endptr, 0);
            if (*endptr != 0) {
                return 1;
            }
            Sasa temp = tanabata_sasa_get(tanabata, sasa_id);
            if (temp.id == HOLE_ID) {
                return 1;
            }
            sprintf(*response, "{\"status\":true,\"sasa_id\":0x%lx,\"sasa_cts\":0x%lx,\"sasa_path\":\"%s\"}",
                    temp.id, temp.created_ts, temp.path);
            return 0;
        }
        size_t resp_size = BUFSIZ;
        buffer = malloc(BUFSIZ);
        sprintf(*response, "{\"status\":true,\"sasa_list\":[");
        Sasa *temp = tanabata->sasahyou.database;
        for (uint64_t i = 0; i < tanabata->sasahyou.size; i++, temp++) {
            if (temp->id == HOLE_ID) {
                continue;
            }
            sprintf(buffer, "{\"sasa_id\":0x%lx,\"sasa_cts\":0x%lx,\"sasa_path\":\"%s\"},",
                    temp->id, temp->created_ts, temp->path);
            if (strlen(*response) + strlen(buffer) >= resp_size) {
                resp_size += BUFSIZ;
                *response = realloc(*response, resp_size);
            }
            strcat(*response, buffer);
        }
        sprintf(buffer, "]}");
        if (strlen(*response) + 3 >= resp_size) {
            *response = realloc(*response, resp_size + 3);
        }
        strcat(*response, buffer);
        free(buffer);
        return 0;
    }
    if (request_code == trc_sasa_get_by_tanzaku) {
        if (tanabata == NULL || *request_body == 0) {
            return 1;
        }
        char *endptr;
        uint64_t tanzaku_id = strtoull(request_body, &endptr, 0);
        if (*endptr != 0) {
            return 1;
        }
        Sasa *list = tanabata_sasa_get_by_tanzaku(tanabata, tanzaku_id);
        if (list == NULL) {
            return 1;
        }
        size_t resp_size = BUFSIZ;
        buffer = malloc(BUFSIZ);
        sprintf(*response, "{\"status\":true,\"sasa_list\":[");
        for (Sasa *temp = list; temp->id != HOLE_ID; temp++) {
            sprintf(buffer, "{\"sasa_id\":0x%lx,\"sasa_cts\":0x%lx,\"sasa_path\":\"%s\"},",
                    temp->id, temp->created_ts, temp->path);
            if (strlen(*response) + strlen(buffer) >= resp_size) {
                resp_size += BUFSIZ;
                *response = realloc(*response, resp_size);
            }
            strcat(*response, buffer);
        }
        sprintf(buffer, "]}");
        if (strlen(*response) + 3 >= resp_size) {
            *response = realloc(*response, resp_size + 3);
        }
        strcat(*response, buffer);
        free(buffer);
        return 0;
    }
    if (request_code == trc_sasa_add) {
        if (tanabata == NULL) {
            return 1;
        }
        return tanabata_sasa_add(tanabata, request_body);
    }
    if (request_code == trc_sasa_update) {
        if (tanabata == NULL) {
            return 1;
        }
        char *endptr;
        uint64_t sasa_id = strtoull(request_body, &endptr, 0);
        if (*endptr != ' ') {
            return 1;
        }
        return tanabata_sasa_upd(tanabata, sasa_id, endptr + 1);
    }
    if (request_code == trc_sasa_remove) {
        if (tanabata == NULL) {
            return 1;
        }
        char *endptr;
        uint64_t sasa_id = strtoull(request_body, &endptr, 0);
        if (*endptr != 0) {
            return 1;
        }
        return tanabata_sasa_rem(tanabata, sasa_id);
    }
    return 1;
}

// Client thread
void *client_thread(void *arg) {
    uint16_t thread_id = 1 + (pthread_t *) arg - threads;
    int client_fd;
    unsigned int critical_count = 0;
    char buffer[BUFSIZ];
    ssize_t nread, nwrite;
    size_t req_size, resp_size;
    char *request = NULL, *response = NULL;
    for (; critical_count < max_criticals;) {
        pthread_mutex_lock(&mutex_connection);
        if ((client_fd = accept(server_fd, NULL, NULL)) < 0) {
            logtf(LOG_CRITICAL, "thread %i: failed to accept connection", thread_id);
            critical_count++;
            continue;
        }
        logtf(LOG_INFO, "thread %i: client connected", thread_id);
        pthread_mutex_unlock(&mutex_connection);
        critical_count = 0;
        for (;;) {
            req_size = BUFSIZ;
            request = malloc(req_size);
            for (off_t offset = 0; (nread = read(client_fd, buffer, BUFSIZ)) > 0;) {
                if (offset + nread > req_size) {
                    req_size += BUFSIZ;
                    request = realloc(request, req_size);
                }
                memcpy(request + offset, buffer, nread);
                offset += nread;
                if (request[offset - 1] == 0) {
                    break;
                }
            }
            if (nread < 0) {
                logtf(LOG_ERROR, "thread %i: failed to read request", thread_id);
            } else if (nread > 0) {
                logtf(LOG_INFO, "thread %i: request got", thread_id);
                pthread_mutex_lock(&mutex_request);
                if (execute(request, &response) != 0) {
                    logtf(LOG_INFO, "thread %i: request executed, status 'false'", thread_id);
                    strcpy(response, "{\"status\":false}");
                } else {
                    logtf(LOG_INFO, "thread %i: request executed, status 'true'", thread_id);
                    if (*response == 0) {
                        strcpy(response, "{\"status\":true}");
                    }
                }
                pthread_mutex_unlock(&mutex_request);
                resp_size = strlen(response) + 1;
                for (char *buf = response; (nwrite = write(client_fd, buf, resp_size)) > 0;) {
                    buf += nwrite;
                    resp_size -= nwrite;
                    if (resp_size == 0) {
                        break;
                    }
                }
                if (nwrite <= 0) {
                    logtf(LOG_ERROR, "thread %i: failed to send response", thread_id);
                }
                logtf(LOG_INFO, "thread %i: response sent", thread_id);
            }
            free(request);
            free(response);
            request = NULL;
            response = NULL;
            if (nread == 0) {
                break;
            }
        }
        close(client_fd);
        logtf(LOG_INFO, "thread %i: connection closed", thread_id);
    }
    logtf(LOG_INFO, "thread %i: thread finished", thread_id);
    return NULL;
}

int main() {
    logtf(LOG_INFO, "TDBMS server started");
    sock_domain = malloc(strlen(DEFAULT_SOCK_DOMAIN) + 1);
    strcpy(sock_domain, DEFAULT_SOCK_DOMAIN);
    sock_addr = malloc(strlen(DEFAULT_SOCK_ADDR) + 1);
    strcpy(sock_addr, DEFAULT_SOCK_ADDR);
    config_load();
    if (logfile == NULL &&
        (logfile = fopen(DEFAULT_LOG_FILE, "a")) == NULL) {
        logtf(LOG_FATAL, "failed to load default log file");
        return 1;
    }
    logtf(LOG_INFO, "configuration loaded");
    dblist_load();
    server_fd = socket_open();
    threads = malloc(max_clients * sizeof(pthread_t));
    for (uint16_t i = 0; i < max_clients; i++) {
        if (pthread_create(threads + i, NULL, client_thread, threads + i) != 0) {
            logtf(LOG_CRITICAL, "failed to create a thread");
        }
    }
    for (unsigned int i = 0; i < max_clients; i++) {
        if (pthread_join(threads[i], NULL) != 0) {
            logtf(LOG_CRITICAL, "failed to join a thread");
        }
    }
    close(server_fd);
    return 0;
}
