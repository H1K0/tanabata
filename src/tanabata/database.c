#include <malloc.h>
#include <string.h>
#include <sys/stat.h>

#include "../../include/tanabata.h"

int tanabata_init(Tanabata *tanabata) {
    int status = 0;
    status |= sasahyou_init(&tanabata->sasahyou);
    status |= sappyou_init(&tanabata->sappyou);
    status |= shoppyou_init(&tanabata->shoppyou);

    return status;
}

int tanabata_free(Tanabata *tanabata) {
    int status = 0;
    status |= sasahyou_free(&tanabata->sasahyou);
    status |= sappyou_free(&tanabata->sappyou);
    status |= shoppyou_free(&tanabata->shoppyou);

    return status;
}

int tanabata_weed(Tanabata *tanabata) {
    int status = 0;
    status |= sasahyou_weed(&tanabata->sasahyou);
    status |= sappyou_weed(&tanabata->sappyou);
    status |= shoppyou_weed(&tanabata->shoppyou);

    return status;
}

int tanabata_load(Tanabata *tanabata) {
    int status = 0;
    status |= sasahyou_load(&tanabata->sasahyou);
    status |= sappyou_load(&tanabata->sappyou);
    status |= shoppyou_load(&tanabata->shoppyou);

    return status;
}

int tanabata_save(Tanabata *tanabata) {
    int status = 0;
    status |= sasahyou_save(&tanabata->sasahyou);
    status |= sappyou_save(&tanabata->sappyou);
    status |= shoppyou_save(&tanabata->shoppyou);

    return status;
}

int tanabata_open(Tanabata *tanabata, const char *path) {
    struct stat st;
    if (stat(path, &st) != 0 || !S_ISDIR(st.st_mode)) {
        return 1;
    }
    int status = 0;
    char *file_path = malloc(strlen(path) + 10);
    strcpy(file_path, path);
    status |= sasahyou_open(&tanabata->sasahyou, strcat(file_path, "/sasahyou"));
    strcpy(file_path, path);
    status |= sappyou_open(&tanabata->sappyou, strcat(file_path, "/sappyou"));
    strcpy(file_path, path);
    status |= shoppyou_open(&tanabata->shoppyou, strcat(file_path, "/shoppyou"));
    free(file_path);

    return status;
}

int tanabata_dump(Tanabata *tanabata, const char *path) {
    struct stat st;
    if (stat(path, &st) != 0 || !S_ISDIR(st.st_mode)) {
        return 1;
    }
    int status = 0;
    char *file_path = malloc(strlen(path) + 10);
    strcpy(file_path, path);
    status |= sasahyou_dump(&tanabata->sasahyou, strcat(file_path, "/sasahyou"));
    strcpy(file_path, path);
    status |= sappyou_dump(&tanabata->sappyou, strcat(file_path, "/sappyou"));
    strcpy(file_path, path);
    status |= shoppyou_dump(&tanabata->shoppyou, strcat(file_path, "/shoppyou"));
    free(file_path);

    return status;
}
