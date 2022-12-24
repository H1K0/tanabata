#include <malloc.h>
#include <string.h>
#include <sys/stat.h>

#include "../../include/tanabata.h"

int tanabata_init(Tanabata *tanabata) {
    if (sasahyou_init(&tanabata->sasahyou) != 0) {
        return 1;
    }
    if (sappyou_init(&tanabata->sappyou) != 0) {
        return 1;
    }
    if (shoppyou_init(&tanabata->shoppyou) != 0) {
        return 1;
    }
    return 0;
}

int tanabata_free(Tanabata *tanabata) {
    if (sasahyou_free(&tanabata->sasahyou) != 0) {
        return 1;
    }
    if (sappyou_free(&tanabata->sappyou) != 0) {
        return 1;
    }
    if (shoppyou_free(&tanabata->shoppyou) != 0) {
        return 1;
    }
    return 0;
}

int tanabata_weed(Tanabata *tanabata) {
    int status = 0;
    status |= sasahyou_weed(&tanabata->sasahyou);
    status |= sappyou_weed(&tanabata->sappyou);
    status |= shoppyou_weed(&tanabata->shoppyou);

    return status;
}

int tanabata_load(Tanabata *tanabata) {
    if (sasahyou_load(&tanabata->sasahyou) != 0) {
        return 1;
    }
    if (sappyou_load(&tanabata->sappyou) != 0) {
        return 1;
    }
    if (shoppyou_load(&tanabata->shoppyou) != 0) {
        return 1;
    }
    return 0;
}

int tanabata_save(Tanabata *tanabata) {
    if (sasahyou_save(&tanabata->sasahyou) != 0) {
        return 1;
    }
    if (sappyou_save(&tanabata->sappyou) != 0) {
        return 1;
    }
    if (shoppyou_save(&tanabata->shoppyou) != 0) {
        return 1;
    }
    return 0;
}

int tanabata_open(Tanabata *tanabata, const char *path) {
    struct stat st;
    if (stat(path, &st) != 0 || !S_ISDIR(st.st_mode)) {
        return 1;
    }
    char *file_path = malloc(strlen(path) + 10);
    strcpy(file_path, path);
    if (sasahyou_open(&tanabata->sasahyou, strcat(file_path, "/sasahyou")) != 0) {
        return 1;
    }
    strcpy(file_path, path);
    if (sappyou_open(&tanabata->sappyou, strcat(file_path, "/sappyou")) != 0) {
        return 1;
    }
    strcpy(file_path, path);
    if (shoppyou_open(&tanabata->shoppyou, strcat(file_path, "/shoppyou")) != 0) {
        return 1;
    }
    free(file_path);
    return 0;
}

int tanabata_dump(Tanabata *tanabata, const char *path) {
    struct stat st;
    if (stat(path, &st) != 0 || !S_ISDIR(st.st_mode)) {
        return 1;
    }
    char *file_path = malloc(strlen(path) + 10);
    strcpy(file_path, path);
    if (sasahyou_dump(&tanabata->sasahyou, strcat(file_path, "/sasahyou")) != 0) {
        return 1;
    }
    strcpy(file_path, path);
    if (sappyou_dump(&tanabata->sappyou, strcat(file_path, "/sappyou")) != 0) {
        return 1;
    }
    strcpy(file_path, path);
    if (shoppyou_dump(&tanabata->shoppyou, strcat(file_path, "/shoppyou")) != 0) {
        return 1;
    }
    free(file_path);
    return 0;
}
